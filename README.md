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
make build   # 编译
make test    # 运行测试
make integration-test # 运行显式启用的真实 ClickHouse 集成测试
make lint    # 代码检查
make vet     # 静态分析
make fmt     # 格式化
```

## Live 集成测试

默认测试不会连接外部服务。需要复验真实 ClickHouse 时，先在环境中提供 `CLICKHOUSEX_TEST_*` 或 `FOUNDATIONX_CLICKHOUSEX_*` 配置（`HOST`、`PORT`、`DATABASE`、`USERNAME`、`PASSWORD` 或 `DSN`），再运行：

```bash
CLICKHOUSEX_RUN_INTEGRATION=1 go test -count=1 -run TestClickHouseLiveIntegration -v ./pkg/clickhousex
```
