.PHONY: build test integration-test lint vet fmt

build:
	go build ./...

test:
	go test -race -count=1 ./...

integration-test:
	CLICKHOUSEX_RUN_INTEGRATION=1 GOWORK=off go test -count=1 -run TestClickHouseLiveIntegration -v ./pkg/clickhousex

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .
