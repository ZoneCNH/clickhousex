# clickhousex

ClickHouse 客户端 SDK — OLAP 查询、批量写入、连接管理和健康检查。

## 模块信息

- 模块路径: `github.com/ZoneCNH/clickhousex`
- Go 版本: 1.23

## 安装

```bash
go get github.com/ZoneCNH/clickhousex
```

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
	defer client.Close(ctx)

	// 健康检查
	status := client.HealthCheck(ctx)
	log.Printf("health: %s", status.Status)
}
```

## 构建与测试

```bash
make build   # 编译
make test    # 运行测试
make lint    # 代码检查
make vet     # 静态分析
make fmt     # 格式化
```
