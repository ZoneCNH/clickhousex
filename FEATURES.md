# clickhousex v1.0.10 功能清单

本文档记录当前仓库已对齐到 `v1.0.10` 的公开功能面，作为 `README.md` 与发布证据之间的可追溯投影。

## 定位

`clickhousex` 是 ClickHouse 客户端 SDK，面向 OLAP 查询、批量写入、连接管理、健康检查、指标与 tracing hooks。

当前发布姿态保持在 `L2-T3`，`factory_grade=false`。

## 核心能力

- `Exec` / `Query`：执行 SQL，并在扫描前校验目标数量、`nil` 指针、nullable 列和 `decimal.Decimal` 兼容性。
- `InsertBatch`：使用 ClickHouse native batch 协议写入，校验表名、列名、空列和行宽。
- `Health` / `HealthCheck` / `Ping`：输出 ready/live 状态，并上报健康指标。
- `Close` / `CloseContext`：幂等关闭底层连接。
- 配置能力：支持 DSN、脱敏输出、连接池参数、重试策略、指标、日志与 tracing hooks。

## 验证面

- `make release-check` 是当前可本地闭环的生产发布门禁。
- `make factory-check` 是更严格的 `L2-T4` Factory Grade 门禁。
- 真实 ClickHouse 集成测试默认不启用，需要显式设置 `CLICKHOUSEX_RUN_INTEGRATION=1`。
- 短周期 soak 复验需要额外设置 `CLICKHOUSEX_RUN_SOAK=1`。

## 发布边界

当前仓库允许发布到 `L2-T3`，但还没有进入 Factory Grade。

Factory Grade 仍然受以下证据缺口约束：

- 多小时真实 ClickHouse soak 证据尚未归档。
- 外部 consumer rollout 证据尚未归档。
- 来自通过的 factory workflow 的 factory release archive 尚未产出。

