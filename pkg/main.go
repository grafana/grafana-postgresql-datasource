package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-postgresql-datasource/pkg/postgresql"
)

func main() {
	logger := backend.NewLoggerWith("logger", "tsdb.postgres")
	if err := datasource.Manage("grafana-postgresql-datasource", postgresql.NewInstanceSettings(logger), datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error(err.Error())
		os.Exit(1)
	}
}
