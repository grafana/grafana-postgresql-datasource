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

	// Regression test for https://github.com/grafana/grafana/issues/131104:
	// max lifetime = 0 ("no limit") must not set pgxpool.MaxConnLifetime to 0,
	// since pgxpool v5.10.0 treats a zero lifetime as immediate expiry and every
	// Acquire then fails with "too many failed attempts acquiring connection".
	t.Run("ConnMaxLifetime=0 leaves MaxConnLifetime unset (no limit)", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: 0})
		require.Equal(t, time.Duration(0), cfg.MaxConnLifetime)
	})

	t.Run("ConnMaxLifetime>0 sets MaxConnLifetime", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: 30})
		require.Equal(t, 30*time.Second, cfg.MaxConnLifetime)
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
}
