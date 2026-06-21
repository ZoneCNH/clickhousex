# clickhousex v1.0.10 验收标准

本文档定义 `v1.0.10` 当前可接受的发布条件，以及仍然保留的 Factory Grade 门槛。

## 当前验收

以下条件满足时，可将当前发布视为 `L2-T3 release-ready`：

- `make release-check` 通过。
- `release_level_actual` 为 `L2-T3`。
- `release_allowed` 为 `true`。
- `factory_grade` 维持为 `false`，不冒充 `L2-T4`。
- 证据文件完整且可读：
  - `.agent/evidence/decision/release-readiness.json`
  - `.agent/evidence/manifest.json`
  - `.agent/evidence/trace/traceability-matrix.json`
- `README.md`、执行方案和 release evidence 在版本号与门禁描述上保持一致。

## 功能验收

- `Exec` / `Query` 的扫描校验行为与文档描述一致。
- `InsertBatch` 的参数校验行为与文档描述一致。
- `Health` / `HealthCheck` / `Ping` 能输出健康状态并暴露指标。
- `Close` / `CloseContext` 保持幂等。
- 默认测试不连接外部 ClickHouse，真实连接仅在显式启用时发生。

## 发布门禁

- `make release-check` 是当前生产发布门禁。
- `make factory-check` 只有在补齐以下证据后才应转为通过：
  - 多小时真实 ClickHouse soak 归档。
  - 外部 consumer rollout 归档。
  - factory-grade release archive。
  - `retrospective` 证据。

## 不可混淆项

- `L2-T3` 不等于 `L2-T4`。
- 现有发布并未达到 Factory Grade。
- 未补齐证据之前，不应把 `factory_grade` 直接改为 `true`。

