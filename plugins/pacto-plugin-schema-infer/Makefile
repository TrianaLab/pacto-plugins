PLUGIN_NAME := pacto-plugin-schema-infer
BIN_DIR := ../../dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
GOBIN := $(shell go env GOBIN 2>/dev/null)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

.PHONY: build test e2e coverage lint ci install clean

build:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(PLUGIN_NAME) ./cmd/

test:
	go test ./... -v

e2e:
	go test -tags e2e ./tests/e2e/ -v -count=1 -timeout 120s

coverage:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)
	go vet ./...

ci: lint test e2e

install:
	go build $(LDFLAGS) -o "$(GOBIN)/$(PLUGIN_NAME)" ./cmd/

clean:
	rm -f $(BIN_DIR)/$(PLUGIN_NAME) coverage.out coverage.html
