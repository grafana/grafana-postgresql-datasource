package postgresql

import (
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-postgresql-datasource/pkg/postgresql/sqleng"
)

// TestFrameBuilder in sqleng already covers appendRow's byte-limit accounting
// against mocked rows. This exercises the same limit over a real connection -
// queryToDataFrame's mrr.NextResult()/rr.NextRow() loop against pgconn - to
// guard against the early-stop-without-draining behavior ever regressing to
// full buffering, or a protocol desync, at the wire level.
func TestIntegrationResponseLimitBytes(t *testing.T) {
	skipIntegrationTestInShortMode(t)

	origInterpolate := sqleng.Interpolate
	t.Cleanup(func() { sqleng.Interpolate = origInterpolate })
	sqleng.Interpolate = func(query backend.DataQuery, timeRange backend.TimeRange, timeInterval string, sql string) string {
		return sql
	}

	jsonData := sqleng.JsonData{
		MaxOpenConns:        10,
		MaxIdleConns:        2,
		ConnMaxLifetime:     14400,
		Mode:                "disable",
		ConfigurationMethod: "file-path",
	}
	dsInfo := sqleng.DataSourceInfo{JsonData: jsonData, DecryptedSecureJSONData: map[string]string{}}
	logger := backend.NewLoggerWith("logger", "response-limit-bytes.test")
	cnnstr := postgresTestDBConnString()

	const (
		rowWidthBytes      = 1_000
		totalRows          = 1_000
		responseLimitBytes = int64(5_000)
	)
	// Each row's single text value is exactly rowWidthBytes raw bytes, so the
	// limit is reached, deterministically, after this many rows.
	expectedRows := int(responseLimitBytes) / rowWidthBytes

	p, exe, err := newPostgres(t.Context(), "error", 1_000_000, responseLimitBytes, dsInfo, cnnstr, logger, backend.DataSourceInstanceSettings{})
	require.NoError(t, err)

	_, err = p.Exec(t.Context(), fmt.Sprintf(`
		DROP TABLE IF EXISTS response_limit_bytes_test;
		CREATE TABLE response_limit_bytes_test (id bigserial PRIMARY KEY, data text);
		INSERT INTO response_limit_bytes_test (data)
		SELECT repeat('x', %d) FROM generate_series(1, %d);
	`, rowWidthBytes, totalRows))
	require.NoError(t, err)

	query := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				JSON:  []byte(`{"rawSql": "SELECT * FROM response_limit_bytes_test", "format": "table"}`),
				RefID: "A",
			},
		},
	}

	resp, err := exe.QueryData(t.Context(), query)
	require.NoError(t, err)

	queryResult := resp.Responses["A"]
	require.NoError(t, queryResult.Error)
	require.Len(t, queryResult.Frames, 1)

	frame := queryResult.Frames[0]
	require.Equal(t, expectedRows, frame.Rows(), "expected the %d-byte limit to truncate the %d-row result to %d rows", responseLimitBytes, totalRows, expectedRows)
	require.NotNil(t, frame.Meta)
	require.Len(t, frame.Meta.Notices, 1)
	require.Contains(t, frame.Meta.Notices[0].Text, fmt.Sprintf("response size limit of %d bytes", responseLimitBytes))
}
