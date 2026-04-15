# PostgreSQL data source for Grafana

> **Note**: This core plugin was extracted from the [grafana/grafana](https://github.com/grafana/grafana) repository
> and is now bundled with Grafana.

## Overview

The PostgreSQL data source plugin allows you to query and visualize data from a PostgreSQL (and compatible) relational database directly in Grafana.

## Requirements

- Grafana 12.3.0 or later

## Getting started

This plugin is bundled with Grafana — no installation is required for standard Grafana deployments.

1. Navigate to **Connections > Data sources** in Grafana.
2. Click **Add data source** and search for "PostgreSQL".
3. Configure the connection settings and click **Save & test**.

For detailed setup instructions, see the
[PostgreSQL data source documentation](https://grafana.com/docs/grafana/latest/datasources/postgres/).

### Custom Grafana distributions

If you are building a custom Grafana binary or distribution that excludes bundled plugins,
you can install this plugin from the [Grafana plugin catalog](https://grafana.com/grafana/plugins/).

## Documentation

Full documentation is available at:

https://grafana.com/docs/grafana/latest/datasources/postgres/

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

This plugin is licensed under the [AGPL-3.0](LICENSE).
