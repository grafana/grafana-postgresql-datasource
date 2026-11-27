package sqleng

import (
	"hash/maphash"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	pluginType             = "grafana-postgresql-datasource"
	envLongToWideCellLimit = "GF_PLUGIN_LONG_TO_WIDE_CELL_LIMIT"
	largeLongToWideCells   = int64(10_000_000)
)

// Name, buckets and label match the sqlds histogram, so one query covers every SQL data source.
var responseCellsMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace:                       "plugins",
	Name:                            "sql_response_cells",
	Help:                            "Number of cells (rows × fields) returned by a SQL datasource query",
	Buckets:                         []float64{1, 100, 10_000, 1_000_000, 10_000_000, 100_000_000, 1_000_000_000, 10_000_000_000},
	NativeHistogramBucketFactor:     1.1,
	NativeHistogramMaxBucketNumber:  100,
	NativeHistogramMinResetDuration: time.Hour,
}, []string{"datasource_type"})

var longToWideRejections = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "plugins",
	Name:      "sql_long_to_wide_rejections_total",
	Help:      "Number of time series queries rejected because the long-to-wide conversion passed the cell limit",
}, []string{"datasource_type"})

var (
	warnInvalidLongToWideCellLimit sync.Once
	wideShapeSeed                  = maphash.MakeSeed()
)

func observeResponseCells(cells int64) {
	responseCellsMetric.WithLabelValues(pluginType).Observe(float64(cells))
}

// longToWideCellLimit returns the cell limit from GF_PLUGIN_LONG_TO_WIDE_CELL_LIMIT, where 0 means no limit.
func longToWideCellLimit() int64 {
	s := os.Getenv(envLongToWideCellLimit)
	if s == "" {
		return 0
	}
	limit, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		warnInvalidLongToWideCellLimit.Do(func() {
			backend.Logger.Warn("ignoring invalid long-to-wide cell limit", "env", envLongToWideCellLimit, "value", s, "error", err)
		})
		return 0
	}
	return max(limit, 0)
}

// wideShape returns the rows and fields that data.LongToWide builds from frame, without building it.
// It returns ok=false for input that LongToWide rejects: null or unsorted times, or a factor that is not a string or bool.
func wideShape(frame *data.Frame, schema data.TimeSeriesSchema) (rows, fields int64, ok bool) {
	n := frame.Rows()
	if n == 0 {
		return 0, 0, false
	}
	series := make(map[uint64]struct{})
	var h maphash.Hash
	h.SetSeed(wideShapeSeed)
	var last time.Time
	for i := range n {
		t, ok := timeValue(frame.Fields[schema.TimeIndex].At(i))
		if !ok || t.Before(last) {
			return 0, 0, false
		}
		if i == 0 || t.After(last) {
			rows++
			last = t
		}
		h.Reset()
		for _, f := range schema.FactorIndices {
			if !writeFactor(&h, frame.Fields[f].At(i)) {
				return 0, 0, false
			}
			_ = h.WriteByte(0)
		}
		series[h.Sum64()] = struct{}{}
	}
	return rows, 1 + int64(len(schema.ValueIndices))*int64(len(series)), true
}

func timeValue(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	}
	return time.Time{}, false
}

func writeFactor(h *maphash.Hash, v any) bool {
	switch f := v.(type) {
	case string:
		_, _ = h.WriteString(f)
	case *string:
		if f != nil {
			_, _ = h.WriteString(*f)
		}
	case bool:
		_, _ = h.WriteString(strconv.FormatBool(f))
	case *bool:
		if f != nil {
			_, _ = h.WriteString(strconv.FormatBool(*f))
		}
	default:
		return false
	}
	return true
}
