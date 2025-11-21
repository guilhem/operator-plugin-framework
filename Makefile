.PHONY: help test test-e2e lint fmt vet build clean

help:
	@echo "Operator Plugin Framework - Make targets"
	@echo ""
	@echo "Usage:"
	@echo "  make test              Run unit tests with coverage"
	@echo "  make test-e2e          Run end-to-end tests"
	@echo "  make test-all          Run all tests"
	@echo "  make lint              Run golangci-lint"
	@echo "  make fmt               Run go fmt"
	@echo "  make vet               Run go vet"
	@echo "  make build             Build all packages"
	@echo "  make clean             Clean build artifacts"

# Test targets
test:
	go test -v -race -coverprofile=coverage.out ./server ./registry ./client
	@echo ""
	@echo "Coverage report generated: coverage.out"

test-e2e:
	go test -v -race -timeout 30s ./test/e2e

test-all: test test-e2e
	@echo ""
	@echo "All tests passed!"

# Lint and format targets
lint:
	which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

# Build targets
build:
	go build ./...

clean:
	go clean -testcache
	rm -f coverage.out
