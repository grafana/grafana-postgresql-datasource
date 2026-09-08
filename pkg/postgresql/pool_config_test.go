package postgresql

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana-postgresql-datasource/pkg/postgresql/sqleng"
)

func TestApplyPoolConfig(t *testing.T) {
	// Regression test for https://github.com/grafana/grafana/issues/119810:
	// maxOpenConns=0 must not produce "MaxSize must be >= 1" from pgxpool.
	t.Run("MaxOpenConns=0 leaves MaxConns unset so pgxpool uses its default", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{MaxOpenConns: 0})
		// MaxConns=0 tells pgxpool to apply its own default (max(4, NumCPU)).
		// Passing 0 directly to pgxpool.NewWithConfig would propagate to puddle
		// as MaxSize=0 and fail with "MaxSize must be >= 1".
		require.Equal(t, int32(0), cfg.MaxConns)
	})

	t.Run("MaxOpenConns>0 sets MaxConns", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{MaxOpenConns: 10})
		require.Equal(t, int32(10), cfg.MaxConns)
	})

	t.Run("negative MaxOpenConns leaves MaxConns unset", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{MaxOpenConns: -1})
		require.Equal(t, int32(0), cfg.MaxConns)
	})

	// Regression test for https://github.com/grafana/grafana/issues/131104:
	// connMaxLifetime=0 must not reach pgxpool as a zero lifetime. pgxpool expires
	// a connection at creation time + MaxConnLifetime, and since pgx v5.10.0 it
	// checks that when a connection is acquired, so a zero lifetime makes every
	// Acquire destroy its connection and retry until it fails with
	// "too many failed attempts acquiring connection".
	t.Run("ConnMaxLifetime=0 keeps the lifetime pgxpool defaults to", func(t *testing.T) {
		cfg := parseTestConfig(t)
		require.Positive(t, cfg.MaxConnLifetime)

		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: 0})
		require.Positive(t, cfg.MaxConnLifetime)
	})

	t.Run("negative ConnMaxLifetime keeps the lifetime pgxpool defaults to", func(t *testing.T) {
		cfg := parseTestConfig(t)
		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: -1})
		require.Positive(t, cfg.MaxConnLifetime)
	})

	t.Run("ConnMaxLifetime>0 sets MaxConnLifetime", func(t *testing.T) {
		cfg := parseTestConfig(t)
		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: 14400})
		require.Equal(t, 4*time.Hour, cfg.MaxConnLifetime)
	})
}

func parseTestConfig(t *testing.T) *pgxpool.Config {
	t.Helper()

	// ParseConfig, not a bare Config, so that the pool defaults the settings under
	// test are expected to fall back to are in place.
	cfg, err := pgxpool.ParseConfig("postgres://user:pwd@localhost:5432/db")
	require.NoError(t, err)
	return cfg
}
