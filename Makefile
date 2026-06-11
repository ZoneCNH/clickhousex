.PHONY: build test lint vet fmt

build:
	go build ./...

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .
