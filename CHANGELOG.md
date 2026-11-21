# Changelog

## Unreleased

- Stream query results row by row instead of buffering the whole result set, and add an optional per-data-source response size limit ([#140](https://github.com/grafana/grafana-postgresql-datasource/issues/140))

## [13.1.0](https://github.com/grafana/grafana-postgresql-datasource/compare/v13.0.7...v13.1.0) (2026-11-21)


### 🎉 Features

* stream query results and add a per-datasource response size limit ([#172](https://github.com/grafana/grafana-postgresql-datasource/issues/172)) ([6840391](https://github.com/grafana/grafana-postgresql-datasource/commit/6840391a01473d49bb882000362d098a28f4a6e6))


### 🐛 Bug Fixes

* **deps:** update backend dependencies ([#188](https://github.com/grafana/grafana-postgresql-datasource/issues/188)) ([a30b656](https://github.com/grafana/grafana-postgresql-datasource/commit/a30b656d484cbb957fe57a188e2e262bf4fb7cbb))
* **deps:** update frontend dependencies ([#198](https://github.com/grafana/grafana-postgresql-datasource/issues/198)) ([1966e22](https://github.com/grafana/grafana-postgresql-datasource/commit/1966e22798f684c74dbb66da90aed6220a450b06))
* **deps:** update frontend dependencies ([#209](https://github.com/grafana/grafana-postgresql-datasource/issues/209)) ([f2d827a](https://github.com/grafana/grafana-postgresql-datasource/commit/f2d827a099948cfadee58c70b73a24aceb7f39e8))
* **deps:** update module github.com/grafana/grafana-plugin-sdk-go to v0.299.0 ([#207](https://github.com/grafana/grafana-postgresql-datasource/issues/207)) ([27c93c2](https://github.com/grafana/grafana-postgresql-datasource/commit/27c93c28e2dc00953a7292e36099dae2eba61a6c))
* **deps:** update module github.com/grafana/grafana-plugin-sdk-go to v0.300.0 ([6a9becc](https://github.com/grafana/grafana-postgresql-datasource/commit/6a9beccff0443842b1de911019693c96d7ab14b7))
* **deps:** update module github.com/jackc/pgx/v5 to v5.13.0 ([#210](https://github.com/grafana/grafana-postgresql-datasource/issues/210)) ([a02d8f4](https://github.com/grafana/grafana-postgresql-datasource/commit/a02d8f4322f4cac21c078c669a6a1ab2d5322aa6))
* **security/medium/:** update dependency glob to v14 [security] ([#192](https://github.com/grafana/grafana-postgresql-datasource/issues/192)) ([4135565](https://github.com/grafana/grafana-postgresql-datasource/commit/4135565dd3f160286341a5287850e5dd16511371))


### 📝 Documentation

* **contributing:** correct the version bump of docs, test and ci commits ([#216](https://github.com/grafana/grafana-postgresql-datasource/issues/216)) ([0369c85](https://github.com/grafana/grafana-postgresql-datasource/commit/0369c853024ca68b58dba12bfec3e70794e4445d))


### ✅ Tests

* **e2e:** detect Grafana versions that serve the core PostgreSQL plugin ([#180](https://github.com/grafana/grafana-postgresql-datasource/issues/180)) ([c8de8e4](https://github.com/grafana/grafana-postgresql-datasource/commit/c8de8e4c3ad2584c87831e395b9b9e458a63fcfc))
* **e2e:** enable SQLite WAL and more transaction retries at startup ([#181](https://github.com/grafana/grafana-postgresql-datasource/issues/181)) ([d6bcff1](https://github.com/grafana/grafana-postgresql-datasource/commit/d6bcff156d102772fce9ca1f6dbaa8223eb90fdc))


### 🤖 Continuous Integration

* enrol in release-please and conventional commits ([#213](https://github.com/grafana/grafana-postgresql-datasource/issues/213)) ([9d081d8](https://github.com/grafana/grafana-postgresql-datasource/commit/9d081d83c6be477c4000574fc808e8876425b8ea))

## 13.0.7

- Update backend dependencies
- Fix a connection freeze between Grafana and the plugin under heavy query load by updating google.golang.org/grpc to v1.85.0 ([grpc/grpc-go#9330](https://github.com/grpc/grpc-go/issues/9330))

## 13.0.6

- Fix security vulnerabilities in frontend and Go dependencies ([#178](https://github.com/grafana/grafana-postgresql-datasource/pull/178))
- Update Go to 1.27.3 ([#183](https://github.com/grafana/grafana-postgresql-datasource/pull/183))
- Pin golangci-lint to 2.15.0 across CI and release workflows ([#184](https://github.com/grafana/grafana-postgresql-datasource/pull/184))

## 13.0.5

- Fix security vulnerabilities (CVE-2026-102276, CVE-2026-102278, CVE-2026-102990)

## 13.0.4

- Fix security vulnerabilities (CVE-2026-84445)

## 13.0.3

- Clear CVEs due in the current SLO window across frontend dependencies ([#169](https://github.com/grafana/grafana-postgresql-datasource/pull/169))
- Add secure socks proxy (PDC) end to end coverage ([#159](https://github.com/grafana/grafana-postgresql-datasource/pull/159))
- Fixes CVE-2026-19475 ([#168](https://github.com/grafana/grafana-postgresql-datasource/pull/168))

## 13.0.2

- Bump go v1.26.7 and grafana-plugin-sdk-go v0.296.4 ([#119](https://github.com/grafana/grafana-postgresql-datasource/pull/119))
- Bump frontend dependencies ([#110](https://github.com/grafana/grafana-postgresql-datasource/pull/110))
- Bump grafana/* dependencies ([#154](https://github.com/grafana/grafana-postgresql-datasource/pull/154))
- Preserve SQLCommenter tags in stripSQLComments ([#156](https://github.com/grafana/grafana-postgresql-datasource/pull/156))
- feat(config): add PostgreSQL versions 16, 17, and 18 to version dropdown ([#156](https://github.com/grafana/grafana-postgresql-datasource/pull/157))

## 13.0.1

- Bump and pin frontend dependencies
- Fix data source init failure when maxOpenConns=0 ([#103](https://github.com/grafana/grafana-postgresql-datasource/pull/103))

## 13.0.0

- Initial public release as external plugin.
- Allow sql_engine to return results for EXPLAIN queries ([#23](https://github.com/grafana/grafana-postgresql-datasource/pull/23))

## 1.0.0 (Unreleased)

- Initial release.
