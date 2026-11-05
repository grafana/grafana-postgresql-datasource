# Grafana PostgreSQL Data Source - Native Plugin

Grafana ships with a built-in PostgreSQL data source plugin that allows you to query and visualize data from a PostgreSQL compatible database.

## Adding the data source

1. Open the side menu by clicking the Grafana icon in the top header.
2. In the side menu under the Dashboards link you should find a link named Data Sources.
3. Click the + Add data source button in the top header.
4. Select PostgreSQL from the Type dropdown.

[http://docs.grafana.org/features/datasources/postgres/](http://docs.grafana.org/features/datasources/postgres/)

## Limiting query result size

A broad `SELECT` can return more data than the Grafana backend can hold in memory. Two limits guard against this, and both are enforced as the result streams in, so the backend never buffers more than the configured amount:

- **Row limit** — the server-wide `[dataproxy] row_limit` setting. Applies to every SQL data source. A result with more rows than this is truncated and a warning is attached to the panel.
- **Response size limit** — an optional per–data-source cap on the total size, in bytes, of a single query's result set, set with the **Response size limit** field under _Connection limits_. Leave it empty or set it to `0` to disable. When the limit is reached the query stops reading and the partial result is returned with a warning.
