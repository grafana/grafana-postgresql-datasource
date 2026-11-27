package postgresql

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
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

	t.Run("ConnMaxLifetime>0 sets MaxConnLifetime", func(t *testing.T) {
		cfg := &pgxpool.Config{}
		applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: 30})
		require.Equal(t, 30*time.Second, cfg.MaxConnLifetime)
	})

	// Regression test for https://github.com/grafana/grafana/issues/131104:
	// connMaxLifetime<=0 means no limit and must not expire connections on acquire.
	for _, lifetime := range []int{0, -1} {
		t.Run(fmt.Sprintf("ConnMaxLifetime=%d does not expire connections", lifetime), func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig("postgres://user@127.0.0.1/db?sslmode=disable")
			require.NoError(t, err)
			cfg.ConnConfig.DialFunc = dialFakePostgres
			applyPoolConfig(cfg, sqleng.JsonData{ConnMaxLifetime: lifetime})

			pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
			require.NoError(t, err)
			defer pool.Close()

			conn, err := pool.Acquire(t.Context())
			require.NoError(t, err)
			conn.Release()
		})
	}
}

func dialFakePostgres(context.Context, string, string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer func() { _ = server.Close() }()
		backend := pgproto3.NewBackend(server, server)
		if _, err := backend.ReceiveStartupMessage(); err != nil {
			return
		}
		backend.Send(&pgproto3.AuthenticationOk{})
		backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		if err := backend.Flush(); err != nil {
			return
		}
		for {
			if _, err := backend.Receive(); err != nil {
				return
			}
		}
	}()
	return client, nil
}
