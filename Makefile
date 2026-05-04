.PHONY: build test vet lint fmt vuln tidy vendor clean install help

MODULE  := github.com/stuffbucket/ovh-cli
BIN     := ovh
PKG     := ./cmd/$(BIN)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/internal/cli/version.version=$(VERSION) \
	-X $(MODULE)/internal/cli/version.commit=$(COMMIT) \
	-X $(MODULE)/internal/cli/version.date=$(DATE)

build:   ## Build the ovh binary
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:    ## Run tests with race detector
	go test -race -coverprofile=coverage.out ./...

vet:     ## go vet
	go vet ./...

lint:    ## golangci-lint
	golangci-lint run

fmt:     ## gofmt -l -w
	gofmt -l -w .

vuln:    ## govulncheck
	govulncheck ./...

tidy:    ## go mod tidy
	go mod tidy

vendor:  ## go mod vendor
	go mod vendor

clean:   ## remove build artifacts
	rm -f $(BIN) coverage.out coverage.html

install: build  ## install to GOBIN
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

help:    ## show this help
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
