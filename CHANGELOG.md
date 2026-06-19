# Changelog

## v1.0.5 - 2026-06-19

- Raise the local release evidence from broad fake-driver coverage to exact 100.0% statement coverage across the module.
- Add targeted branch coverage for default validation failures, canceled query setup, context-canceled close waits, timed retry waits, and retry-delay clamping.
- Keep the runtime client API unchanged while adding a private adapter hook so native `clickhouse-go/v2` option conversion remains covered.
- Remove the unreachable retry-loop return path so Staticcheck and the coverage gate agree on the executable surface.

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
