# Changelog

## v1.0.10 - 2026-06-21

- Integrate factory release validation into main and align release metadata to v1.0.10.
- Keep the release at L2-T3; factory-grade evidence remains blocked until multi-hour soak, external rollout, and archive evidence exist.

## v1.0.9 - 2026-06-19

- Add repository CI and a manual/scheduled factory evidence workflow, with pinned actions and a release metadata consistency job.
- Promote `make test-coverage` into the release gate and enforce exact 100.0% total coverage without leaving coverage artifacts in the worktree.
- Add machine-readable L2 evidence, `make release-check`, and `make factory-check`; keep the release at L2-T3 until multi-hour soak, external rollout, and factory artifact evidence are archived.
- Re-run the local dev ClickHouse integration and 60s soak gates from environment-supplied configuration without committing credential values.

## v1.0.8 - 2026-06-19

- Re-verify the complete client release gate against the local dev ClickHouse instance: unit, race, vet, build, lint, 100.0% statement coverage, live integration, and a 60s live soak (366 iterations).
- Refresh release evidence and re-align all version metadata (`VERSION`, `.repo-contract.yaml`, `pkg/clickhousex/version.go`, `release/manifest/latest.json`) to v1.0.8.
- Merge the clickhousex integration branch into `main` so the v1.0.3–v1.0.8 release line is reachable from the default branch.

## v1.0.7 - 2026-06-19

- Add a default-skipped live ClickHouse soak gate that repeatedly writes, counts, and reads back rows using environment-supplied dev credentials.
- Add reproducible fake-driver benchmarks for `Exec`, query/scan, batch insert, and health checks.
- Add `make soak-test`, `make benchmark`, and `make profile` so local release evidence can include soak plus CPU/memory profile artifacts without committing generated profiles.

## v1.0.6 - 2026-06-19

- Add default-skipped live ClickHouse integration coverage for `New`, `Ping`, `HealthCheck`, `Exec`, `InsertBatch`, `Query`, `Rows` metadata, scan, and cleanup against a real development instance.
- Add `make integration-test` plus README environment wiring for secret-free local live verification.
- Record release evidence for the local dev integration gate while keeping credential values outside committed artifacts.

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
