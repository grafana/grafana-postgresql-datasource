package schema_test

import (
	_ "embed"
	"reflect"
	"testing"

	"github.com/grafana/dsconfig/schema"
	"github.com/grafana/grafana-plugin-sdk-go/experimental/pluginschema"
	"github.com/grafana/grafana-postgresql-datasource/pkg/postgresql/sqleng"
	"k8s.io/kube-openapi/pkg/spec3"
)

//go:embed dsconfig.json
var configSchemaJSON []byte

// The SQL model also contains settings used only for health diagnostics or by other drivers.
func jsonDataModel() any {
	type editorAndSDKSettings struct {
		PostgresVersion int `json:"postgresVersion"`
		Timeout         int `json:"timeout"`
		KeepAlive       int `json:"keepAlive"`
	}
	fields := reflect.VisibleFields(reflect.TypeFor[editorAndSDKSettings]())
	for _, field := range reflect.VisibleFields(reflect.TypeFor[sqleng.JsonData]()) {
		switch field.Tag.Get("json") {
		case "maxIdleConns", "connectionTimeout", "tlsSkipVerify", "timezone", "encrypt", "servername", "allowCleartextPasswords", "authenticationType":
			continue
		}
		fields = append(fields, field)
	}
	return reflect.New(reflect.StructOf(fields)).Interface()
}

//go:generate go test -run TestPlugin -generateArtifacts
func TestPlugin(t *testing.T) {
	schema.RunPluginTests(t, schema.PluginUnderTest{
		ID:                "grafana-postgresql-datasource",
		ConfigSchemaJSON:  configSchemaJSON,
		SettingsJSONModel: jsonDataModel(),
		SecureKeys:        []string{"password", "tlsCACert", "tlsClientCert", "tlsClientKey", "secureSocksProxyPassword"},
		SettingsExamples: &pluginschema.SettingsExamples{Examples: map[string]*spec3.Example{
			"":           example("PostgreSQL over TCP", "localhost:5432", "disable", nil, "password"),
			"unixSocket": example("PostgreSQL peer authentication over a Unix socket", "/var/run/postgresql", "disable", nil),
			"timescale": example("TimescaleDB over TLS", "timescale.example.com:5432", "require", map[string]any{
				"database": "metrics", "timescaledb": true, "postgresVersion": 1000,
			}, "password"),
			"privateCA": example("Verify the server using an inline CA certificate", "db.example.com:5432", "verify-ca", map[string]any{
				"tlsConfigurationMethod": "file-content",
			}, "tlsCACert", "password"),
			"clientCertificateFiles": example("Verify the server name and authenticate with certificate files", "db.example.com:5432", "verify-full", map[string]any{
				"tlsConfigurationMethod": "file-path",
				"sslRootCertFile":        "/etc/grafana/certs/root.crt",
				"sslCertFile":            "/etc/grafana/certs/client.crt",
				"sslKeyFile":             "/etc/grafana/certs/client.key",
			}),
			"directTLS": example("Direct TLS with client certificate files through an SNI proxy", "sni-proxy:5432", "verify-full", map[string]any{
				"sslNegotiation": "direct", "tlsConfigurationMethod": "file-path",
				"sslRootCertFile": "/etc/grafana/certs/root.crt",
				"sslCertFile":     "/etc/grafana/certs/client.crt",
				"sslKeyFile":      "/etc/grafana/certs/client.key",
			}),
			"privateNetwork": example("Private Data Source Connect with proxy credentials", "db.example.com:5432", "require", map[string]any{
				"enableSecureSocksProxy": true, "secureSocksProxyUsername": "<PROXY_USERNAME>",
			}, "password", "secureSocksProxyPassword"),
			"preferTLS": example("Try TLS first and allow a plaintext fallback", "db.example.com:5432", "prefer", nil, "password"),
			"allowTLS":  example("Try plaintext first and allow a TLS fallback", "db.example.com:5432", "allow", nil, "password"),
		}},
	})
}

func example(summary, host, sslMode string, settings map[string]any, secrets ...string) *spec3.Example {
	jsonData := map[string]any{"database": "grafana", "sslmode": sslMode}
	for key, value := range settings {
		jsonData[key] = value
	}
	value := map[string]any{
		"apiVersion": "grafana-postgresql-datasource.datasource.grafana.app/v0alpha1",
		"kind":       "DataSource",
		"metadata":   map[string]string{"name": "postgresql"},
		"spec":       map[string]any{"title": "PostgreSQL", "url": host, "user": "grafana", "jsonData": jsonData},
	}
	if len(secrets) > 0 {
		placeholders := map[string]string{
			"password": "<PASSWORD>", "tlsCACert": "<CA_CERTIFICATE_PEM>",
			"secureSocksProxyPassword": "<PROXY_PASSWORD>",
		}
		secure := map[string]map[string]string{}
		for _, key := range secrets {
			secure[key] = map[string]string{"create": placeholders[key]}
		}
		value["secure"] = secure
	}
	return &spec3.Example{ExampleProps: spec3.ExampleProps{Summary: summary, Value: value}}
}
