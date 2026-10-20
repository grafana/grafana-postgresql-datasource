package main

import (
	"os"
	"os/user"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-postgresql-datasource/pkg/postgresql"
)

func main() {
	restoreHomeDir()
	logger := backend.NewLoggerWith("logger", "tsdb.postgres")
	if err := datasource.Manage("grafana-postgresql-datasource", postgresql.NewInstanceSettings(logger), datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}

// restoreHomeDir sets $HOME from the OS account when Grafana starts the plugin without host env vars,
// because pgconn finds ~/.pgpass, ~/.pg_service.conf and ~/.postgresql certificates through $HOME.
func restoreHomeDir() {
	if os.Getenv("HOME") != "" {
		return
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		_ = os.Setenv("HOME", u.HomeDir)
	}
}
