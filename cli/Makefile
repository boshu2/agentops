.PHONY: build test lint clean install sync-hooks

# Build variables
BINARY_NAME=ao
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
IDENTIFIER=com.12factoragentops.ao
GOLANGCI_LINT ?= ../scripts/golangci-lint-v2.sh

# Default target
all: build

# Sync hooks, scripts, and skills into cli/embedded/ for Go embed
sync-hooks:
	@mkdir -p embedded/hooks embedded/lib embedded/skills/standards/references embedded/skills/using-agentops embedded/skills/compile/scripts
	@cp ../hooks/*.sh ../hooks/hooks.json embedded/hooks/
	@cp ../lib/hook-helpers.sh embedded/lib/
	@cp ../skills/standards/references/* embedded/skills/standards/references/
	@cp ../skills/using-agentops/SKILL.md embedded/skills/using-agentops/SKILL.md
	@cp ../skills/compile/scripts/compile.sh embedded/skills/compile/scripts/compile.sh

# Build the binary
build: sync-hooks
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ao
ifeq ($(shell uname -s),Darwin)
	codesign -f -s - -i $(IDENTIFIER) $(BUILD_DIR)/$(BINARY_NAME)
endif

# Run tests
test: build
	go test -v -shuffle=on ./...

# Run tests with coverage
test-coverage: build
	go test -v -shuffle=on -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	$(GOLANGCI_LINT) run ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install to GOPATH/bin (builds first to ensure embedded files are synced)
install: build
	@GOBIN=$$(go env GOBIN); [ -z "$$GOBIN" ] && GOBIN=$$(go env GOPATH)/bin; \
	/bin/cp $(BUILD_DIR)/$(BINARY_NAME) "$$GOBIN/$(BINARY_NAME)"

# Run the binary
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Tidy go modules
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Generate (for future code generation)
generate:
	go generate ./...
