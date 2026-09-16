package sqleng

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MetaKeyExecutedQueryString is the key where the executed query should get stored
const MetaKeyExecutedQueryString = "executedQueryString"

// SQLMacroEngine interpolates macros into sql. It takes in the Query to have access to query context and
// timeRange to be able to generate queries that use from and to.
type SQLMacroEngine interface {
	Interpolate(query *backend.DataQuery, timeRange backend.TimeRange, sql string) (string, error)
}

// SqlQueryResultTransformer transforms a query result row to RowValues with proper types.
type SqlQueryResultTransformer interface {
	// TransformQueryError transforms a query error.
	TransformQueryError(logger log.Logger, err error) error
	GetConverterList() []sqlutil.StringConverter
}

type JsonData struct {
	MaxOpenConns            int    `json:"maxOpenConns"`
	MaxIdleConns            int    `json:"maxIdleConns"`
	ConnMaxLifetime         int    `json:"connMaxLifetime"`
	ConnectionTimeout       int    `json:"connectionTimeout"`
	Timescaledb             bool   `json:"timescaledb"`
	Mode                    string `json:"sslmode"`
	ConfigurationMethod     string `json:"tlsConfigurationMethod"`
	TlsSkipVerify           bool   `json:"tlsSkipVerify"`
	RootCertFile            string `json:"sslRootCertFile"`
	CertFile                string `json:"sslCertFile"`
	CertKeyFile             string `json:"sslKeyFile"`
	Timezone                string `json:"timezone"`
	Encrypt                 string `json:"encrypt"`
	Servername              string `json:"servername"`
	TimeInterval            string `json:"timeInterval"`
	Database                string `json:"database"`
	SecureDSProxy           bool   `json:"enableSecureSocksProxy"`
	SecureDSProxyUsername   string `json:"secureSocksProxyUsername"`
	AllowCleartextPasswords bool   `json:"allowCleartextPasswords"`
	AuthenticationType      string `json:"authenticationType"`
	ResponseLimitBytes      int64  `json:"responseLimitBytes"`
}

type DataSourceInfo struct {
	JsonData                JsonData
	URL                     string
	User                    string
	Database                string
	ID                      int64
	Updated                 time.Time
	UID                     string
	DecryptedSecureJSONData map[string]string
}

type DataPluginConfiguration struct {
	DSInfo             DataSourceInfo
	TimeColumnNames    []string
	MetricColumnTypes  []string
	RowLimit           int64
	ResponseLimitBytes int64
}

type DataSourceHandler struct {
	macroEngine            SQLMacroEngine
	queryResultTransformer SqlQueryResultTransformer
	timeColumnNames        []string
	metricColumnTypes      []string
	log                    log.Logger
	dsInfo                 DataSourceInfo
	rowLimit               int64
	responseLimitBytes     int64
	userError              string
	pool                   *pgxpool.Pool
}

type QueryJson struct {
	RawSql       string  `json:"rawSql"`
	Fill         bool    `json:"fill"`
	FillInterval float64 `json:"fillInterval"`
	FillMode     string  `json:"fillMode"`
	FillValue    float64 `json:"fillValue"`
	Format       string  `json:"format"`
}

func (e *DataSourceHandler) TransformQueryError(logger log.Logger, err error) error {
	// OpError is the error type usually returned by functions in the net
	// package. It describes the operation, network type, and address of
	// an error. We log this error rather than return it to the client
	// for security purposes.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Errorf("failed to connect to server - %s", e.userError)
	}

	return e.queryResultTransformer.TransformQueryError(logger, err)
}

func NewQueryDataHandler(userFacingDefaultError string, p *pgxpool.Pool, config DataPluginConfiguration, queryResultTransformer SqlQueryResultTransformer,
	macroEngine SQLMacroEngine, log log.Logger) (*DataSourceHandler, error) {
	queryDataHandler := DataSourceHandler{
		queryResultTransformer: queryResultTransformer,
		macroEngine:            macroEngine,
		timeColumnNames:        []string{"time"},
		log:                    log,
		dsInfo:                 config.DSInfo,
		rowLimit:               config.RowLimit,
		responseLimitBytes:     config.ResponseLimitBytes,
		userError:              userFacingDefaultError,
	}

	if len(config.TimeColumnNames) > 0 {
		queryDataHandler.timeColumnNames = config.TimeColumnNames
	}

	if len(config.MetricColumnTypes) > 0 {
		queryDataHandler.metricColumnTypes = config.MetricColumnTypes
	}

	queryDataHandler.pool = p
	return &queryDataHandler, nil
}

type DBDataResponse struct {
	dataResponse backend.DataResponse
	refID        string
}

func (e *DataSourceHandler) Dispose() {
	e.log.Debug("Disposing DB...")

	if e.pool != nil {
		e.pool.Close()
	}

	e.log.Debug("DB disposed")
}

func (e *DataSourceHandler) Ping(ctx context.Context) error {
	return e.pool.Ping(ctx)
}

func (e *DataSourceHandler) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	result := backend.NewQueryDataResponse()
	ch := make(chan DBDataResponse, len(req.Queries))
	var wg sync.WaitGroup
	// Execute each query in a goroutine and wait for them to finish afterwards
	for _, query := range req.Queries {
		queryjson := QueryJson{
			Fill:   false,
			Format: "time_series",
		}
		err := json.Unmarshal(query.JSON, &queryjson)
		if err != nil {
			return nil, backend.DownstreamErrorf("error unmarshal query json: %s", err.Error())
		}

		// the fill-params are only stored inside this function, during query-interpolation. we do not support
		// sending them in "from the outside"
		if queryjson.Fill || queryjson.FillInterval != 0.0 || queryjson.FillMode != "" || queryjson.FillValue != 0.0 {
			return nil, backend.DownstreamErrorf("query fill-parameters not supported")
		}

		if queryjson.RawSql == "" {
			continue
		}

		wg.Add(1)
		go e.executeQuery(ctx, query, &wg, ch, queryjson)
	}

	wg.Wait()

	// Read results from channels
	close(ch)
	result.Responses = make(map[string]backend.DataResponse)
	for queryResult := range ch {
		result.Responses[queryResult.refID] = queryResult.dataResponse
	}

	return result, nil
}

// queryToDataFrame runs query and streams the result straight into a data.Frame,
// one row at a time, instead of buffering the whole result set in memory first.
// It stops reading as soon as the row-count or response-byte limit is hit,
// appending a warning notice to the frame. It also returns the field
// descriptions of the first row-returning result, which the caller needs to
// build the query model.
func (e *DataSourceHandler) queryToDataFrame(ctx context.Context, query string) (*data.Frame, []pgconn.FieldDescription, error) {
	c, err := e.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, backend.DownstreamErrorf("failed to acquire connection: %w", err)
	}
	// When we stop early (a limit was hit) the connection still has unread rows
	// queued. We deliberately do not drain them - mrr.Close()/rr.Close() would
	// read them all into memory, which is exactly what the limit exists to
	// prevent. Release() sees the busy connection and destroys it instead.
	defer c.Release()

	mrr := c.Conn().PgConn().Exec(ctx, query)

	fb := newFrameBuilder(e.rowLimit, e.responseLimitBytes)

	for mrr.NextResult() {
		rr := mrr.ResultReader()
		fds := rr.FieldDescriptions()

		if err := fb.startResult(fds); err != nil {
			return nil, nil, err
		}

		if len(fds) == 0 {
			// A statement that does not return rows (INSERT/UPDATE/DELETE/SET/...).
			if _, err := rr.Close(); err != nil {
				return nil, nil, err
			}
			continue
		}

		limited := false
		for rr.NextRow() {
			stop, err := fb.appendRow(fds, rr.Values())
			if err != nil {
				return nil, nil, err
			}
			if stop {
				limited = true
				break
			}
		}
		if limited {
			return fb.frame(), fb.firstFieldDescriptions, nil
		}

		if _, err := rr.Close(); err != nil {
			return nil, nil, err
		}
	}

	if err := mrr.Close(); err != nil {
		return nil, nil, err
	}

	return fb.frame(), fb.firstFieldDescriptions, nil
}

func (e *DataSourceHandler) executeQuery(queryContext context.Context, query backend.DataQuery, wg *sync.WaitGroup,
	ch chan DBDataResponse, queryJSON QueryJson) {
	defer wg.Done()
	queryResult := DBDataResponse{
		dataResponse: backend.DataResponse{},
		refID:        query.RefID,
	}

	logger := e.log.FromContext(queryContext)
	defer e.handlePanic(logger, &queryResult, ch)

	if queryJSON.RawSql == "" {
		panic("Query model property rawSql should not be empty at this point")
	}

	// A trailing SQLCommenter attribution tag must reach the database verbatim,
	// so split it off before interpolation and re-append it afterwards. This
	// keeps it out of comment stripping and macro substitution, and prevents a
	// macro from completing across the comment boundary in either direction.
	rawSQL, sqlCommenterTag := SplitTrailingSQLCommenter(queryJSON.RawSql, "--")

	// global substitutions
	interpolatedQuery := Interpolate(query, query.TimeRange, e.dsInfo.JsonData.TimeInterval, rawSQL)

	// data source specific substitutions
	interpolatedQuery, err := e.macroEngine.Interpolate(&query, query.TimeRange, interpolatedQuery)
	if err != nil {
		e.handleQueryError("interpolation failed", e.TransformQueryError(logger, err), interpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
		return
	}
	interpolatedQuery += sqlCommenterTag

	frame, fieldDescriptions, err := e.queryToDataFrame(queryContext, interpolatedQuery)
	if err != nil {
		e.handleQueryError("db query error", e.TransformQueryError(logger, err), interpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
		return
	}

	qm, err := e.newProcessCfg(queryContext, query, fieldDescriptions, interpolatedQuery)
	if err != nil {
		e.handleQueryError("failed to get configurations", err, interpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
		return
	}

	e.processFrame(frame, qm, queryResult, ch, logger)
}

func (e *DataSourceHandler) handleQueryError(frameErr string, err error, query string, source backend.ErrorSource, ch chan DBDataResponse, queryResult DBDataResponse) {
	var emptyFrame data.Frame
	emptyFrame.SetMeta(&data.FrameMeta{ExecutedQueryString: query})
	if isDownstreamError(err) {
		source = backend.ErrorSourceDownstream
	}
	queryResult.dataResponse.Error = fmt.Errorf("%s: %w", frameErr, err)
	queryResult.dataResponse.ErrorSource = source
	queryResult.dataResponse.Frames = data.Frames{&emptyFrame}
	ch <- queryResult
}

func (e *DataSourceHandler) handlePanic(logger log.Logger, queryResult *DBDataResponse, ch chan DBDataResponse) {
	if r := recover(); r != nil {
		logger.Error("ExecuteQuery panic", "error", r, "stack", string(debug.Stack()))
		if theErr, ok := r.(error); ok {
			queryResult.dataResponse.Error = theErr
			queryResult.dataResponse.ErrorSource = backend.ErrorSourcePlugin
		} else if theErrString, ok := r.(string); ok {
			queryResult.dataResponse.Error = errors.New(theErrString)
			queryResult.dataResponse.ErrorSource = backend.ErrorSourcePlugin
		} else {
			queryResult.dataResponse.Error = fmt.Errorf("unexpected error - %s", e.userError)
			queryResult.dataResponse.ErrorSource = backend.ErrorSourceDownstream
		}
		ch <- *queryResult
	}
}

func (e *DataSourceHandler) processFrame(frame *data.Frame, qm *dataQueryModel, queryResult DBDataResponse, ch chan DBDataResponse, logger log.Logger) {
	if frame.Meta == nil {
		frame.Meta = &data.FrameMeta{}
	}
	frame.Meta.ExecutedQueryString = qm.InterpolatedQuery

	// If no rows were returned, clear any previously set `Fields` with a single empty `data.Field` slice.
	// Then assign `queryResult.dataResponse.Frames` the current single frame with that single empty Field.
	// This assures 1) our visualization doesn't display unwanted empty fields, and also that 2)
	// additionally-needed frame data stays intact and is correctly passed to our visulization.
	if frame.Rows() == 0 {
		frame.Fields = []*data.Field{}
		queryResult.dataResponse.Frames = data.Frames{frame}
		ch <- queryResult
		return
	}

	if err := convertSQLTimeColumnsToEpochMS(frame, qm); err != nil {
		e.handleQueryError("converting time columns failed", err, qm.InterpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
		return
	}

	if qm.Format == dataQueryFormatSeries {
		// time series has to have time column
		if qm.timeIndex == -1 {
			e.handleQueryError("db has no time column", errors.New("time column is missing; make sure your data includes a time column for time series format or switch to a table format that doesn't require it"), qm.InterpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
			return
		}

		// Make sure to name the time field 'Time' to be backward compatible with Grafana pre-v8.
		frame.Fields[qm.timeIndex].Name = data.TimeSeriesTimeFieldName

		for i := range qm.columnNames {
			if i == qm.timeIndex || i == qm.metricIndex {
				continue
			}

			if t := frame.Fields[i].Type(); t == data.FieldTypeString || t == data.FieldTypeNullableString {
				continue
			}

			var err error
			if frame, err = convertSQLValueColumnToFloat(frame, i); err != nil {
				e.handleQueryError("convert value to float failed", err, qm.InterpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
				return
			}
		}

		tsSchema := frame.TimeSeriesSchema()
		if tsSchema.Type == data.TimeSeriesTypeLong {
			var err error
			originalData := frame
			frame, err = data.LongToWide(frame, qm.FillMissing)
			if err != nil {
				e.handleQueryError("failed to convert long to wide series when converting from dataframe", err, qm.InterpolatedQuery, backend.ErrorSourceDownstream, ch, queryResult)
				return
			}

			// Before 8x, a special metric column was used to name time series. The LongToWide transforms that into a metric label on the value field.
			// But that makes series name have both the value column name AND the metric name. So here we are removing the metric label here and moving it to the
			// field name to get the same naming for the series as pre v8
			if len(originalData.Fields) == 3 {
				for _, field := range frame.Fields {
					if len(field.Labels) == 1 { // 7x only supported one label
						name, ok := field.Labels["metric"]
						if ok {
							field.Name = name
							field.Labels = nil
						}
					}
				}
			}
		}
		if qm.FillMissing != nil {
			frame = e.applyFill(frame, qm)
		}
	}

	queryResult.dataResponse.Frames = data.Frames{frame}
	ch <- queryResult
}

// Interpolate provides global macros/substitutions for all sql datasources.
var Interpolate = func(query backend.DataQuery, timeRange backend.TimeRange, timeInterval string, sql string) string {
	interval := query.Interval

	sql = strings.ReplaceAll(sql, "$__interval_ms", strconv.FormatInt(interval.Milliseconds(), 10))
	sql = strings.ReplaceAll(sql, "$__interval", gtime.FormatInterval(interval))
	sql = strings.ReplaceAll(sql, "$__unixEpochFrom()", fmt.Sprintf("%d", timeRange.From.UTC().Unix()))
	sql = strings.ReplaceAll(sql, "$__unixEpochTo()", fmt.Sprintf("%d", timeRange.To.UTC().Unix()))

	return sql
}

func (e *DataSourceHandler) newProcessCfg(queryContext context.Context, query backend.DataQuery,
	fieldDescriptions []pgconn.FieldDescription, interpolatedQuery string) (*dataQueryModel, error) {
	columnNames := make([]string, 0, len(fieldDescriptions))
	columnTypes := make([]string, 0, len(fieldDescriptions))

	// The field descriptions of the first row-returning result carry the column
	// metadata (a multi-statement query's later results must have the same shape).
	for _, field := range fieldDescriptions {
		columnNames = append(columnNames, field.Name)
		pqtype, ok := pgtype.NewMap().TypeForOID(field.DataTypeOID)
		if !ok {
			// Handle special cases for field types
			switch field.DataTypeOID {
			case pgtype.TimetzOID:
				columnTypes = append(columnTypes, "timetz")
			// money type is 790
			case 790:
				columnTypes = append(columnTypes, "money")
			default:
				columnTypes = append(columnTypes, "unknown")
			}
		} else {
			columnTypes = append(columnTypes, pqtype.Name)
		}
	}

	qm := &dataQueryModel{
		columnTypes:  columnTypes,
		columnNames:  columnNames,
		timeIndex:    -1,
		timeEndIndex: -1,
		metricIndex:  -1,
		metricPrefix: false,
		queryContext: queryContext,
	}

	queryJSON := QueryJson{}
	err := json.Unmarshal(query.JSON, &queryJSON)
	if err != nil {
		return nil, err
	}

	if queryJSON.Fill {
		qm.FillMissing = &data.FillMissing{}
		qm.Interval = time.Duration(queryJSON.FillInterval * float64(time.Second))
		switch strings.ToLower(queryJSON.FillMode) {
		case "null":
			qm.FillMissing.Mode = data.FillModeNull
		case "previous":
			qm.FillMissing.Mode = data.FillModePrevious
		case "value":
			qm.FillMissing.Mode = data.FillModeValue
			qm.FillMissing.Value = queryJSON.FillValue
		default:
		}
	}

	qm.TimeRange.From = query.TimeRange.From.UTC()
	qm.TimeRange.To = query.TimeRange.To.UTC()

	// Default to time_series if no format is provided
	switch queryJSON.Format {
	case "table":
		qm.Format = dataQueryFormatTable
	case "time_series":
		fallthrough
	default:
		qm.Format = dataQueryFormatSeries
	}

	for i, col := range qm.columnNames {
		for _, tc := range e.timeColumnNames {
			if col == tc {
				qm.timeIndex = i
				break
			}
		}

		if qm.Format == dataQueryFormatTable && strings.EqualFold(col, "timeend") {
			qm.timeEndIndex = i
			continue
		}

		switch col {
		case "metric":
			qm.metricIndex = i
		default:
			if qm.metricIndex == -1 {
				columnType := qm.columnTypes[i]
				for _, mct := range e.metricColumnTypes {
					if columnType == mct {
						qm.metricIndex = i
						continue
					}
				}
			}
		}
	}
	qm.InterpolatedQuery = interpolatedQuery
	return qm, nil
}

// dataQueryFormat is the type of query.
type dataQueryFormat string

const (
	// dataQueryFormatTable identifies a table query (default).
	dataQueryFormatTable dataQueryFormat = "table"
	// dataQueryFormatSeries identifies a time series query.
	dataQueryFormatSeries dataQueryFormat = "time_series"
)

type dataQueryModel struct {
	InterpolatedQuery string // property not set until after Interpolate()
	Format            dataQueryFormat
	TimeRange         backend.TimeRange
	FillMissing       *data.FillMissing // property not set until after Interpolate()
	Interval          time.Duration
	columnNames       []string
	columnTypes       []string
	timeIndex         int
	timeEndIndex      int
	metricIndex       int
	metricPrefix      bool
	queryContext      context.Context
}

func convertSQLTimeColumnsToEpochMS(frame *data.Frame, qm *dataQueryModel) error {
	if qm.timeIndex != -1 {
		if err := convertSQLTimeColumnToEpochMS(frame, qm.timeIndex); err != nil {
			return fmt.Errorf("%v: %w", "failed to convert time column", err)
		}
	}

	if qm.timeEndIndex != -1 {
		if err := convertSQLTimeColumnToEpochMS(frame, qm.timeEndIndex); err != nil {
			return fmt.Errorf("%v: %w", "failed to convert timeend column", err)
		}
	}

	return nil
}

// frameBuilder incrementally assembles a data.Frame from query results as they
// are streamed off the wire, enforcing the row-count and response-byte limits
// row by row so a large result never has to be fully materialised in memory.
type frameBuilder struct {
	m         *pgtype.Map
	rowLimit  int64
	byteLimit int64

	dataFrame              *data.Frame
	firstFieldDescriptions []pgconn.FieldDescription
	rowCount               int64
	byteCount              int64
	limited                bool
}

func newFrameBuilder(rowLimit, byteLimit int64) *frameBuilder {
	return &frameBuilder{m: pgtype.NewMap(), rowLimit: rowLimit, byteLimit: byteLimit}
}

// startResult prepares the builder for the next result set. The first
// row-returning result establishes the frame's columns; later results in a
// multi-statement query must match that shape. Results that return no rows
// (INSERT/UPDATE/DELETE/SET/...) are ignored - callers pass an empty fds slice.
//
// We key on len(fds) rather than CommandTag.Select() because EXPLAIN and
// EXPLAIN ANALYZE return rows but carry a non-SELECT command tag.
func (b *frameBuilder) startResult(fds []pgconn.FieldDescription) error {
	if len(fds) == 0 {
		return nil
	}

	if b.dataFrame == nil {
		b.firstFieldDescriptions = append([]pgconn.FieldDescription(nil), fds...)

		fieldTypes, err := getFieldTypesFromDescriptions(fds, b.m)
		if err != nil {
			return err
		}
		fields := make(data.Fields, len(fds))
		for i, fd := range fds {
			fields[i] = data.NewFieldFromFieldType(fieldTypes[i], 0)
			fields[i].Name = fd.Name
		}
		b.dataFrame = data.NewFrame("", fields...)
		return nil
	}

	if len(fds) != len(b.dataFrame.Fields) {
		return fmt.Errorf("incompatible result structure: expected %d columns, got %d columns",
			len(b.dataFrame.Fields), len(fds))
	}
	for i, fd := range fds {
		if fd.Name != b.dataFrame.Fields[i].Name {
			return fmt.Errorf("column name mismatch at position %d: expected %q, got %q",
				i, b.dataFrame.Fields[i].Name, fd.Name)
		}
	}
	return nil
}

// appendRow converts and appends one row. It returns stop=true once a limit has
// been reached (after appending a warning notice to the frame), at which point
// the caller must stop reading and not call appendRow again.
func (b *frameBuilder) appendRow(fds []pgconn.FieldDescription, values [][]byte) (bool, error) {
	if b.limited {
		return true, nil
	}

	if b.rowLimit > 0 && b.rowCount >= b.rowLimit {
		b.dataFrame.AppendNotices(data.Notice{
			Severity: data.NoticeSeverityWarning,
			Text:     fmt.Sprintf("Results have been limited to %v because the SQL row limit was reached", b.rowLimit),
		})
		b.limited = true
		return true, nil
	}

	row := make([]any, len(fds))
	rowBytes := 0
	for i, fd := range fds {
		rawValue := values[i]
		if rawValue == nil {
			row[i] = nil
			continue
		}
		rowBytes += len(rawValue)

		convertedValue, err := convertPostgresValue(rawValue, fd, b.m)
		if err != nil {
			return false, err
		}
		row[i] = convertedValue
	}

	if len(row) != len(b.dataFrame.Fields) {
		return false, fmt.Errorf("row data length mismatch: expected %d values, got %d values",
			len(b.dataFrame.Fields), len(row))
	}

	b.dataFrame.AppendRow(row...)
	b.rowCount++
	b.byteCount += int64(rowBytes)

	if b.byteLimit > 0 && b.byteCount >= b.byteLimit {
		b.dataFrame.AppendNotices(data.Notice{
			Severity: data.NoticeSeverityWarning,
			Text:     fmt.Sprintf("Results have been limited because the response size limit of %d bytes was reached", b.byteLimit),
		})
		b.limited = true
		return true, nil
	}

	return false, nil
}

// frame returns the assembled frame, or an empty frame if no result returned rows.
func (b *frameBuilder) frame() *data.Frame {
	if b.dataFrame == nil {
		return data.NewFrame("")
	}
	return b.dataFrame
}

// convertPostgresValue converts a raw PostgreSQL value to the appropriate Go type
func convertPostgresValue(rawValue []byte, fd pgconn.FieldDescription, m *pgtype.Map) (interface{}, error) {
	dataTypeOID := fd.DataTypeOID
	format := fd.Format

	// Convert based on type
	switch fd.DataTypeOID {
	case pgtype.Int2OID:
		var d *int16
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.Int4OID:
		var d *int32
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.Int8OID:
		var d *int64
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.NumericOID, pgtype.Float8OID, pgtype.Float4OID:
		var d *float64
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.BoolOID:
		var d *bool
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.ByteaOID:
		d, err := pgtype.ByteaCodec.DecodeValue(pgtype.ByteaCodec{}, m, dataTypeOID, format, rawValue)
		if err != nil {
			return nil, err
		}
		str := string(d.([]byte))
		return &str, nil
	case pgtype.TimestampOID, pgtype.TimestamptzOID, pgtype.DateOID:
		var d *time.Time
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.TimeOID, pgtype.TimetzOID:
		var d *string
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	case pgtype.JSONOID, pgtype.JSONBOID:
		var d *string
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		// Handle null JSON values
		if d == nil {
			return nil, nil
		}
		j := json.RawMessage(*d)
		return &j, nil
	default:
		var d *string
		scanPlan := m.PlanScan(dataTypeOID, format, &d)
		err := scanPlan.Scan(rawValue, &d)
		if err != nil {
			return nil, err
		}
		return d, nil
	}
}

func getFieldTypesFromDescriptions(fieldDescriptions []pgconn.FieldDescription, m *pgtype.Map) ([]data.FieldType, error) {
	fieldTypes := make([]data.FieldType, len(fieldDescriptions))
	for i, v := range fieldDescriptions {
		typeName, ok := m.TypeForOID(v.DataTypeOID)
		if !ok {
			fieldTypes[i] = data.FieldTypeNullableString
		} else {
			switch typeName.Name {
			case "int2":
				fieldTypes[i] = data.FieldTypeNullableInt16
			case "int4":
				fieldTypes[i] = data.FieldTypeNullableInt32
			case "int8":
				fieldTypes[i] = data.FieldTypeNullableInt64
			case "float4", "float8", "numeric":
				fieldTypes[i] = data.FieldTypeNullableFloat64
			case "bool":
				fieldTypes[i] = data.FieldTypeNullableBool
			case "timestamptz", "timestamp", "date":
				fieldTypes[i] = data.FieldTypeNullableTime
			case "json", "jsonb":
				fieldTypes[i] = data.FieldTypeNullableJSON //nolint:staticcheck
			default:
				fieldTypes[i] = data.FieldTypeNullableString
			}
		}
	}
	return fieldTypes, nil
}

// convertSQLTimeColumnToEpochMS converts column named time to unix timestamp in milliseconds
// to make native datetime types and epoch dates work in annotation and table queries.
func convertSQLTimeColumnToEpochMS(frame *data.Frame, timeIndex int) error {
	if timeIndex < 0 || timeIndex >= len(frame.Fields) {
		return fmt.Errorf("timeIndex %d is out of range", timeIndex)
	}

	origin := frame.Fields[timeIndex]
	valueType := origin.Type()
	if valueType == data.FieldTypeTime || valueType == data.FieldTypeNullableTime {
		return nil
	}

	newField := data.NewFieldFromFieldType(data.FieldTypeNullableTime, 0)
	newField.Name = origin.Name
	newField.Labels = origin.Labels

	valueLength := origin.Len()
	for i := 0; i < valueLength; i++ {
		v, err := origin.NullableFloatAt(i)
		if err != nil {
			return fmt.Errorf("unable to convert data to a time field")
		}
		if v == nil {
			newField.Append(nil)
		} else {
			timestamp := time.Unix(0, int64(epochPrecisionToMS(*v))*int64(time.Millisecond))
			newField.Append(&timestamp)
		}
	}
	frame.Fields[timeIndex] = newField

	return nil
}

// convertSQLValueColumnToFloat converts timeseries value column to float.
func convertSQLValueColumnToFloat(frame *data.Frame, Index int) (*data.Frame, error) {
	if Index < 0 || Index >= len(frame.Fields) {
		return frame, fmt.Errorf("metricIndex %d is out of range", Index)
	}

	origin := frame.Fields[Index]
	valueType := origin.Type()
	if valueType == data.FieldTypeFloat64 || valueType == data.FieldTypeNullableFloat64 {
		return frame, nil
	}

	newField := data.NewFieldFromFieldType(data.FieldTypeNullableFloat64, origin.Len())
	newField.Name = origin.Name
	newField.Labels = origin.Labels

	for i := 0; i < origin.Len(); i++ {
		v, err := origin.NullableFloatAt(i)
		if err != nil {
			return frame, err
		}
		newField.Set(i, v)
	}

	frame.Fields[Index] = newField

	return frame, nil
}

// fillFreeFields is how wide a frame may be before the resample allowance starts
// shrinking. Frames at or below this width keep the historical allowance of
// rowLimit fill points, so ordinary panels are unaffected.
const fillFreeFields = 10

// maxFillPoints returns the largest number of fill points allowed for a frame
// numFields wide. Resampling allocates one cell per field per fill point, and
// nothing bounds the column count of a query, so past fillFreeFields the
// allowance falls off as 1/numFields. That holds the total number of cells the
// resample allocates at rowLimit*fillFreeFields however wide the frame gets,
// while leaving narrow frames on the historical allowance.
func maxFillPoints(rowLimit, numFields int64) int64 {
	if numFields <= fillFreeFields {
		return rowLimit
	}
	// Divide before multiplying so the product cannot overflow.
	return rowLimit / numFields * fillFreeFields
}

// applyFill resamples frame using the fill configuration in qm. If the fill
// would need more points than the frame's width allows the fill is skipped and a
// warning notice is appended to the frame instead, leaving the rows the query
// really returned untouched.
func (e *DataSourceHandler) applyFill(frame *data.Frame, qm *dataQueryModel) *data.Frame {
	startUnixTime := qm.TimeRange.From.Unix() / int64(qm.Interval.Seconds()) * int64(qm.Interval.Seconds())
	alignedTimeRange := backend.TimeRange{
		From: time.Unix(startUnixTime, 0),
		To:   qm.TimeRange.To,
	}

	// Guard against excessive memory allocation from fill operations that span a
	// very large time range relative to the fill interval, or that fan a modest
	// point count out across a very wide result set.
	numFillPoints := int64(alignedTimeRange.To.Sub(alignedTimeRange.From) / qm.Interval)
	numFields := int64(len(frame.Fields))
	maxPoints := maxFillPoints(e.rowLimit, numFields)
	if numFillPoints > maxPoints {
		e.log.Warn("Skipping fill: number of fill points exceeds the limit for this frame width",
			"numFillPoints", numFillPoints, "numFields", numFields, "maxFillPoints", maxPoints, "rowLimit", e.rowLimit)
		frame.AppendNotices(data.Notice{
			Text:     "Fill operation skipped: time range, interval and number of columns would require more points than the configured row limit allows",
			Severity: data.NoticeSeverityWarning,
		})
		return frame
	}

	var err error
	frame, err = sqlutil.ResampleWideFrame(frame, qm.FillMissing, alignedTimeRange, qm.Interval) //nolint:staticcheck
	if err != nil {
		e.log.Error("Failed to resample dataframe", "err", err)
		frame.AppendNotices(data.Notice{Text: "Failed to resample dataframe", Severity: data.NoticeSeverityWarning})
	}
	return frame
}

func SetupFillmode(query *backend.DataQuery, interval time.Duration, fillmode string) error {
	rawQueryProp := make(map[string]any)
	queryBytes, err := query.JSON.MarshalJSON()
	if err != nil {
		return err
	}
	err = json.Unmarshal(queryBytes, &rawQueryProp)
	if err != nil {
		return err
	}
	rawQueryProp["fill"] = true
	rawQueryProp["fillInterval"] = interval.Seconds()

	switch fillmode {
	case "NULL":
		rawQueryProp["fillMode"] = "null"
	case "previous":
		rawQueryProp["fillMode"] = "previous"
	default:
		rawQueryProp["fillMode"] = "value"
		floatVal, err := strconv.ParseFloat(fillmode, 64)
		if err != nil {
			return fmt.Errorf("error parsing fill value %v", fillmode)
		}
		rawQueryProp["fillValue"] = floatVal
	}
	query.JSON, err = json.Marshal(rawQueryProp)
	if err != nil {
		return err
	}
	return nil
}

type SQLMacroEngineBase struct{}

func NewSQLMacroEngineBase() *SQLMacroEngineBase {
	return &SQLMacroEngineBase{}
}

func (m *SQLMacroEngineBase) ReplaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func([]string) string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllStringSubmatchIndex(str, -1) {
		groups := []string{}
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}

// epochPrecisionToMS converts epoch precision to millisecond, if needed.
// Only seconds to milliseconds supported right now
func epochPrecisionToMS(value float64) float64 {
	s := strconv.FormatFloat(value, 'e', -1, 64)
	if strings.HasSuffix(s, "e+09") {
		return value * float64(1e3)
	}

	if strings.HasSuffix(s, "e+18") {
		return value / float64(time.Millisecond)
	}

	return value
}

func isDownstreamError(err error) bool {
	if backend.IsDownstreamError(err) {
		return true
	}
	resultProcessingDownstreamErrors := []error{
		data.ErrorInputFieldsWithoutRows,
		data.ErrorSeriesUnsorted,
		data.ErrorNullTimeValues,
	}
	for _, e := range resultProcessingDownstreamErrors {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
