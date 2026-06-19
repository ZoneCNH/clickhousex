# Changelog

## v1.0.4 - 2026-06-19

- Add the complete ClickHouse client runtime surface: `Exec`, `Query`, `InsertBatch`, `Ping`, `Health`, `HealthCheck`, `Close`, and `CloseContext`.
- Wire the public client to `clickhouse-go/v2` with DSN support, sanitized config output, connection pool metrics, retry policy hooks, tracing hooks, and structured logging hooks.
- Add scan validation for nullable values and `decimal.Decimal`, table-not-found mapping, write/query metrics, and fake-driver unit coverage for client behavior.
- Keep the module Go contract at 1.23 by pinning `ch-go` to the compatible version declared by `clickhouse-go/v2 v2.39.0`.

## v1.0.3 - 2026-06-19

- Pin CI trust alignment to an xlibgate commit that includes the `trust` command group.
- Add release metadata required by offline release consistency checks.
- Keep v1.0.2 as published history; v1.0.3 is the CI alignment patch release.

## v1.0.2 - 2026-06-19

- Align team integration release gates with the foundation client boundary.
- Preserve the narrow typed client surface while recording remaining complete-client work.
