.PHONY: build build-flywheel test lint clean install sync-hooks inner membrane-parity verify-buildtags

# Build variables
BINARY_NAME=ao
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
IDENTIFIER=com.12factoragentops.ao
GOLANGCI_LINT ?= ../scripts/golangci-lint-v2.sh

# ADR-0012 archive build tags. The default build OMITS both the corpus/flywheel
# (`flywheel`) and RPI/factory (`legacy`) command sets — they are archived, not
# deleted, so the code stays buildable for the revival conditions. Restore them:
#   make build-flywheel        # both tags (flywheel + legacy)
#   AGENTOPS_LEGACY=1 make build  # legacy only
GO_TAGS ?=
ifdef AGENTOPS_LEGACY
GO_TAGS += legacy
endif
GO_TAGS_FLAG = $(if $(strip $(GO_TAGS)),-tags "$(strip $(GO_TAGS))",)

# Default target
all: build

# Sync lib helpers and skills into cli/embedded/ for Go embed
sync-hooks:
	@mkdir -p embedded/lib embedded/skills/standards/references embedded/skills/compile/scripts
	@cp ../lib/chain-parser.sh embedded/lib/
	@cp ../skills/standards/references/* embedded/skills/standards/references/
	@cp ../scripts/lib/flywheel-compile.sh embedded/skills/compile/scripts/compile.sh
	@mkdir -p embedded/pawl/scripts embedded/pawl/scripts/lib embedded/pawl/schemas embedded/pawl/hooks
	@cp ../scripts/pawl-review.sh ../scripts/pawl-verdict.sh ../scripts/pawl.sh embedded/pawl/scripts/
	@cp ../scripts/gen-membrane-receipts.sh ../scripts/check-membrane-receipts-freshness.sh embedded/pawl/scripts/
	@cp ../scripts/lib/codex-exec.sh ../scripts/lib/verify-config.sh ../scripts/lib/diff-identity.sh ../scripts/lib/trivial-waiver.sh ../scripts/lib/pawl-preflight.sh ../scripts/lib/pawl-amend-guard.sh ../scripts/lib/pawl-tip-coherence.sh ../scripts/lib/pawl-evidence-dir.sh embedded/pawl/scripts/lib/
	@cp ../schemas/pawl-verdict.v1.schema.json embedded/pawl/schemas/
	@cp ../scripts/hooks/pre-push-verify.template embedded/pawl/hooks/pre-push-verify.template

# Build the binary (default = spine; archived satellites omitted)
build: sync-hooks
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) $(GO_TAGS_FLAG) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ao
ifeq ($(shell uname -s),Darwin)
	codesign -f -s - -i $(IDENTIFIER) $(BUILD_DIR)/$(BINARY_NAME)
endif

# Build with the archived satellites restored (ADR-0012): corpus/flywheel
# (`flywheel`) + RPI/factory (`legacy`). Used to run or maintain the archived
# command set under the revival conditions.
build-flywheel: GO_TAGS += flywheel legacy
build-flywheel: build

# Verify the build-tag mechanism: default build omits the tagged set; the
# flywheel/legacy tags restore it. Compiles all three variants and checks the
# hidden `ao buildtags` introspection surface.
verify-buildtags:
	bash ../scripts/verify-buildtags.sh

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

# Membrane demo preflight (epic age-cwo, bead age-membrane-memory-arch-tz2s.2.5):
# fail if the demo-path `ao` lacks the current source's membrane surface, so the
# e2e demo can never run against a stale binary and emit a false proof.
# Override the binary under test with AO_BIN=/path/to/ao.
membrane-parity:
	bash ../scripts/check-membrane-parity.sh

# Run the binary
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Tidy go modules
tidy:
	go mod tidy

# Inner loop: <10s compile + vet (cached)
inner:
	go build ./...
	go vet ./...

# Format code
fmt:
	go fmt ./...

# Generate (for future code generation)
generate:
	go generate ./...
