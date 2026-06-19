# clickhousex

ClickHouse 客户端 SDK — OLAP 查询、批量写入、连接管理、指标和健康检查。

## 模块信息

- 模块路径: `github.com/ZoneCNH/clickhousex`
- Go 版本: 1.23

## 安装

```bash
go get github.com/ZoneCNH/clickhousex
```

## 功能

- `Exec` / `Query`: 执行 SQL、返回受控 `Rows` 包装，并在扫描前校验目标数量、nil 指针、nullable 列和 `decimal.Decimal` 兼容性。
- `InsertBatch`: 使用 ClickHouse native batch 协议写入，校验表名、列名、空列和行宽。
- `Health` / `HealthCheck` / `Ping`: 输出 ready/live 状态，并上报健康指标。
- `Close` / `CloseContext`: 幂等关闭底层连接。
- 配置支持 DSN、脱敏输出、连接池参数、重试策略、指标、日志和 tracing hooks。

## 基本用法

```go
package main

import (
	"context"
	"log"

	"github.com/ZoneCNH/clickhousex/pkg/clickhousex"
)

func main() {
	ctx := context.Background()

	cfg := clickhousex.Config{
		Name:     "my-clickhouse",
		Host:     "localhost",
		Port:     clickhousex.DefaultPort,
		Database: "default",
		Username: "default",
		Timeout:  clickhousex.DefaultTimeout,
	}

	client, err := clickhousex.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close clickhouse: %v", err)
		}
	}()

	// 健康检查
	status := client.HealthCheck(ctx)
	log.Printf("health: %s", status.Status)

	if err := client.Exec(ctx, "CREATE TABLE IF NOT EXISTS demo (id UInt64) ENGINE = Memory"); err != nil {
		log.Fatal(err)
	}

	if err := client.InsertBatch(ctx, "demo", []string{"id"}, [][]any{{uint64(1)}}); err != nil {
		log.Fatal(err)
	}
}
```

## 构建与测试

```bash
make build         # 编译
make test-unit     # 单元测试
make test-race     # race 测试
make test-coverage # 100.0% 覆盖率门禁
make test-contract # L2 契约测试切片
make test-chaos    # 重试、错误映射、故障分支切片
make test-adoption # 典型消费方 API 使用切片
make test-arch     # L2 依赖边界检查
make test-security # secret 扫描
make integration-test # 显式启用的真实 ClickHouse 集成测试
make soak-test        # 显式启用的真实 ClickHouse soak 测试
make benchmark     # 本地 fake-driver 基准
make profile       # 生成本地 CPU / memory profile
make release-check # L2-T3 生产发布门禁
make factory-check # L2-T4 Factory Grade 门禁
make lint          # 代码检查
make vet           # 静态分析
make fmt           # 格式化
```

## 生产级门禁

`make release-check` 是当前可本地闭环的生产发布门禁，覆盖 build、unit、race、100.0% coverage、vet、contract、chaos、benchmark、adoption、架构边界、安全扫描和 `.agent/evidence` 证据完整性。通过该门禁表示模块达到 L2-T3 release-ready。GitHub Actions CI 已同步执行常规质量门禁、release metadata consistency、secret scan 和真实 ClickHouse integration job。

`make factory-check` 是 L2-T4 Factory Grade 硬门禁。当前基线刻意保持失败，直到归档多小时真实 ClickHouse soak、外部 consumer rollout 和 factory release archive 证据后，才能把 `.agent/evidence/decision/release-readiness.json` 提升为 `factory_grade=true`。`.github/workflows/factory-grade.yml` 提供手动/定时 factory evidence 采集入口。

## Live 集成测试

默认测试不会连接外部服务。需要复验真实 ClickHouse 时，先在环境中提供 `CLICKHOUSEX_TEST_*` 或 `FOUNDATIONX_CLICKHOUSEX_*` 配置（`HOST`、`PORT`、`DATABASE`、`USERNAME`、`PASSWORD` 或 `DSN`），再运行：

```bash
CLICKHOUSEX_RUN_INTEGRATION=1 go test -count=1 -run TestClickHouseLiveIntegration -v ./pkg/clickhousex
```

需要运行短周期 soak 复验时，额外设置 `CLICKHOUSEX_RUN_SOAK=1`。可用 `CLICKHOUSEX_SOAK_DURATION` 和 `CLICKHOUSEX_SOAK_INTERVAL` 控制时长与间隔：

```bash
CLICKHOUSEX_RUN_INTEGRATION=1 CLICKHOUSEX_RUN_SOAK=1 CLICKHOUSEX_SOAK_DURATION=60s CLICKHOUSEX_SOAK_INTERVAL=100ms go test -count=1 -run TestClickHouseLiveSoak -v ./pkg/clickhousex
```

## Benchmark 与 Profile

本地 benchmark 使用 fake driver，不连接外部 ClickHouse：

```bash
make benchmark
PROFILE_DIR=/tmp/clickhousex-profile make profile
```
