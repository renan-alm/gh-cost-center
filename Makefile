BINARY_NAME := gh-cost-center
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/renan-alm/gh-cost-center/cmd.version=$(VERSION)"

.PHONY: build install test lint clean fmt vet tidy

## build: Compile the binary
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

## install: Install as a local gh extension
install: build
	gh extension install .

## test: Run all tests
test:
	go test ./... -v

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format all Go source files
fmt:
	gofmt -s -w .

## vet: Run go vet
vet:
	go vet ./...

## tidy: Tidy and verify module dependencies
tidy:
	go mod tidy
	go mod verify

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	go clean

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
