package sqleng

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longFrame builds the long frame frameBuilder emits for SELECT time, metric, value.
// With shared timestamps every series has a point at each time, otherwise every point has its own time.
func longFrame(series, points int, shared bool) *data.Frame {
	t0 := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	n := series * points
	times := make([]*time.Time, 0, n)
	metrics := make([]*string, 0, n)
	values := make([]*float64, 0, n)
	for p := range points {
		for s := range series {
			ts := t0.Add(time.Duration(p*series+s) * time.Millisecond)
			if shared {
				ts = t0.Add(time.Duration(p) * time.Second)
			}
			name := fmt.Sprintf("series_%d", s)
			v := float64(p*series + s)
			times = append(times, &ts)
			metrics = append(metrics, &name)
			values = append(values, &v)
		}
	}
	return data.NewFrame("", data.NewField("time", nil, times), data.NewField("metric", nil, metrics), data.NewField("value", nil, values))
}

func seriesQueryModel() *dataQueryModel {
	return &dataQueryModel{Format: dataQueryFormatSeries, columnNames: []string{"time", "metric", "value"}, timeIndex: 0, timeEndIndex: -1, metricIndex: 1}
}

func runProcessFrame(t *testing.T, frame *data.Frame, qm *dataQueryModel, logger log.Logger) backend.DataResponse {
	t.Helper()
	ch := make(chan DBDataResponse, 1)
	(&DataSourceHandler{rowLimit: 1_000_000}).processFrame(frame, qm, DBDataResponse{refID: "A"}, ch, logger)
	return (<-ch).dataResponse
}

func cellsHistogram(t *testing.T) *dto.Histogram {
	t.Helper()
	m, err := responseCellsMetric.GetMetricWithLabelValues(pluginType)
	require.NoError(t, err)
	out := &dto.Metric{}
	require.NoError(t, m.(prometheus.Histogram).Write(out))
	return out.Histogram
}

func rejectionCount(t *testing.T) float64 {
	t.Helper()
	out := &dto.Metric{}
	require.NoError(t, longToWideRejections.WithLabelValues(pluginType).Write(out))
	return out.GetCounter().GetValue()
}

func TestWideShapeMatchesLongToWide(t *testing.T) {
	twoValues := longFrame(3, 4, false)
	second := make([]*float64, twoValues.Rows())
	twoValues.Fields = append(twoValues.Fields, data.NewField("other", nil, second))

	tests := []struct {
		name  string
		frame *data.Frame
	}{
		{name: "dense", frame: longFrame(5, 4, true)},
		{name: "sparse", frame: longFrame(5, 4, false)},
		{name: "two value fields", frame: twoValues},
		{name: "one row", frame: longFrame(1, 1, false)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, fields, ok := wideShape(tc.frame, tc.frame.TimeSeriesSchema())
			require.True(t, ok)
			wide, err := data.LongToWide(tc.frame, nil)
			require.NoError(t, err)
			assert.Equal(t, int64(wide.Rows()), rows)
			assert.Equal(t, int64(len(wide.Fields)), fields)
		})
	}
}

func TestWideShapeRejectsInputLongToWideRejects(t *testing.T) {
	unsorted := longFrame(2, 2, false)
	first := unsorted.Fields[0].At(0).(*time.Time)
	unsorted.Fields[0].Set(0, new(first.Add(time.Hour)))

	nullTime := longFrame(2, 2, false)
	nullTime.Fields[0].Set(1, (*time.Time)(nil))

	for name, frame := range map[string]*data.Frame{"unsorted": unsorted, "null time": nullTime} {
		t.Run(name, func(t *testing.T) {
			_, _, ok := wideShape(frame, frame.TimeSeriesSchema())
			assert.False(t, ok)
		})
	}
}

func TestProcessFrameObservesPredictedCells(t *testing.T) {
	t.Setenv(envLongToWideCellLimit, "")
	before := cellsHistogram(t)

	res := runProcessFrame(t, longFrame(4, 3, false), seriesQueryModel(), backend.Logger)

	require.NoError(t, res.Error)
	after := cellsHistogram(t)
	assert.Equal(t, before.GetSampleCount()+1, after.GetSampleCount())
	assert.Equal(t, before.GetSampleSum()+12*5, after.GetSampleSum())
}

func TestProcessFrameObservesTableCells(t *testing.T) {
	before := cellsHistogram(t)
	qm := seriesQueryModel()
	qm.Format = dataQueryFormatTable

	res := runProcessFrame(t, longFrame(4, 3, false), qm, backend.Logger)

	require.NoError(t, res.Error)
	after := cellsHistogram(t)
	assert.Equal(t, before.GetSampleCount()+1, after.GetSampleCount())
	assert.Equal(t, before.GetSampleSum()+12*3, after.GetSampleSum())
}

func TestProcessFrameLongToWideCellLimit(t *testing.T) {
	t.Run("no limit by default", func(t *testing.T) {
		t.Setenv(envLongToWideCellLimit, "")
		res := runProcessFrame(t, longFrame(4, 3, false), seriesQueryModel(), backend.Logger)
		require.NoError(t, res.Error)
		assert.Equal(t, data.TimeSeriesTypeWide, res.Frames[0].TimeSeriesSchema().Type)
	})

	t.Run("the env limit rejects and counts after observing the shape", func(t *testing.T) {
		t.Setenv(envLongToWideCellLimit, "10")
		rejections := rejectionCount(t)
		before := cellsHistogram(t)

		res := runProcessFrame(t, longFrame(4, 3, false), seriesQueryModel(), backend.Logger)

		require.ErrorIs(t, res.Error, data.ErrorWideFrameTooLarge)
		assert.ErrorContains(t, res.Error, "exceed the limit of 10 cells")
		assert.ErrorContains(t, res.Error, envLongToWideCellLimit)
		assert.Equal(t, backend.ErrorSourceDownstream, res.ErrorSource)
		assert.Equal(t, rejections+1, rejectionCount(t))
		assert.Equal(t, before.GetSampleSum()+12*5, cellsHistogram(t).GetSampleSum())
	})
}

type warnRecorder struct {
	log.Logger
	warnings []string
	args     [][]any
}

func (w *warnRecorder) Warn(msg string, args ...any) {
	w.warnings = append(w.warnings, msg)
	w.args = append(w.args, args)
}

func (w *warnRecorder) FromContext(context.Context) log.Logger { return w }

func TestProcessFrameWarnsOnLargeConversion(t *testing.T) {
	t.Setenv(envLongToWideCellLimit, "100")
	logger := &warnRecorder{Logger: backend.Logger}

	res := runProcessFrame(t, longFrame(2000, 5, false), seriesQueryModel(), logger)

	require.ErrorIs(t, res.Error, data.ErrorWideFrameTooLarge)
	require.Equal(t, []string{"Large long-to-wide conversion"}, logger.warnings)
	assert.Equal(t, []any{"refId", "A", "rows", int64(10_000), "fields", int64(2001), "cells", int64(20_010_000)}, logger.args[0])
}
