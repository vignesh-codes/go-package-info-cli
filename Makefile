# Project variables
BINARY_NAME := package_statistics
PKG := ./...
BUILD_DIR := build
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
ARCH ?= -help

GOVULNCHECK := $(shell which govulncheck || echo $$HOME/go/bin/govulncheck)

# Go related variables
GO := go

.PHONY: all build clean test lint fmt vet run deps help scan ci docker-test docker-build docker-clean

all: build

install:
	@echo "==> Installing required dependencies..."
	$(GO) mod tidy
	$(GO) mod download
	$(GO) install -v ./cmd/package_statistics

## Build binary
build:
	@echo "==> Building..."
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/package_statistics

## Run the application
run: build
	@echo "==> Running..."
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARCH)

## Run tests
test:
	@echo "==> Running tests..."
	$(GO) test $(PKG) -v -cover


## Format code
fmt:
	@echo "==> Formatting code..."
	$(GO) fmt $(PKG)

## Vet code
vet:
	@echo "==> Vetting code..."
	$(GO) vet $(PKG)

## Lint code (requires golangci-lint)
lint:
	@echo "==> Linting..."
	golangci-lint run ./...

## Download dependencies
deps:
	@echo "==> Downloading dependencies..."
	$(GO) mod tidy
	$(GO) mod download

## Clean build artifacts
clean:
	@echo "==> Cleaning..."
	rm -rf $(BUILD_DIR)

## Scan for Go vulnerabilities
scan:
	@echo "==> Scanning Go dependencies for vulnerabilities..."
	$(GOVULNCHECK) ./...

## CI checks
ci: deps fmt vet lint test scan
	@echo "==> CI checks completed"

## Help
help:
	@echo "Common make targets:"
	@echo "  build         - Build the binary"
	@echo "  run           - Run the binary"
	@echo "  test          - Run tests"
	@echo "  fmt           - Format source code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run linters"
	@echo "  deps          - Download dependencies"
	@echo "  clean         - Remove build artifacts"
	@echo "  scan          - Scan dependencies for vulnerabilities"
	@echo "  ci            - Run all CI checks - runs tests, lint, vet, scan"

