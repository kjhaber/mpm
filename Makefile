# mpm Makefile — formatter, linter, tests, coverage, build

BUILD_DIR := build
GOLANGCI_LINT_VERSION := v2.11.3
GOLANGCI_LINT := $(BUILD_DIR)/golangci-lint

.PHONY: fmt lint test cover build completions clean all

# Default target runs all quality checks then builds
all: fmt lint cover build

# Check that all Go files are formatted (fails if any need formatting)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "These files need formatting (run: gofmt -w .):"; gofmt -l .; exit 1)

# Download golangci-lint binary if not present at the pinned version
$(GOLANGCI_LINT):
	@mkdir -p $(BUILD_DIR)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(BUILD_DIR) $(GOLANGCI_LINT_VERSION)

# Run golangci-lint
lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run

# Run unit tests (MPM_* vars cleared so user env doesn't leak in)
TEST_ENV := MPM_ROOT= MPM_FORMAT= MPM_REPO= MPM_PASSWORD=

test:
	@$(TEST_ENV) go test ./...

# Run tests with coverage and print report
cover:
	@mkdir -p $(BUILD_DIR)
	@$(TEST_ENV) go test -race ./... -coverprofile=$(BUILD_DIR)/coverage.out
	@go tool cover -func=$(BUILD_DIR)/coverage.out

# Build the binary (inject version from nearest git tag, fallback to "dev")
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
build:
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/mpm ./cmd/mpm

# Generate shell completion scripts (zsh, bash, fish) into build/completions/
completions: build
	@mkdir -p $(BUILD_DIR)/completions
	@$(BUILD_DIR)/mpm completion zsh > $(BUILD_DIR)/completions/_mpm
	@$(BUILD_DIR)/mpm completion bash > $(BUILD_DIR)/completions/mpm.bash
	@$(BUILD_DIR)/mpm completion fish > $(BUILD_DIR)/completions/mpm.fish

# Remove all build outputs 
clean:
	@rm -rf $(BUILD_DIR)
