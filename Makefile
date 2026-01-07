# Makefile for dorikin
# Kubernetes Configuration Drift Detector

# ══════════════════════════════════════════════════════════════════════════════
# Variables
# ══════════════════════════════════════════════════════════════════════════════

BINARY_NAME := dorikin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS := -s -w \
	-X github.com/indrasvat/dorikin/internal/cli.Version=$(VERSION) \
	-X github.com/indrasvat/dorikin/internal/cli.Commit=$(COMMIT) \
	-X github.com/indrasvat/dorikin/internal/cli.BuildDate=$(BUILD_DATE) \
	-X github.com/indrasvat/dorikin/internal/cli.GoVersion=$(GO_VERSION)

# Directories
BIN_DIR := bin
DIST_DIR := dist
COVERAGE_DIR := coverage

# Tools
GOLANGCI_LINT := golangci-lint
GORELEASER := goreleaser

# Test track script
TEST_TRACK_SCRIPT := ./scripts/test-track.sh

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_MAGENTA := \033[35m

# ══════════════════════════════════════════════════════════════════════════════
# Default target
# ══════════════════════════════════════════════════════════════════════════════

.DEFAULT_GOAL := help

# ══════════════════════════════════════════════════════════════════════════════
# Help
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: help
help: ## Show this help message
	@echo ""
	@echo "$(COLOR_BOLD)🏎️  dorikin - Kubernetes Configuration Drift Detector$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Usage:$(COLOR_RESET)"
	@echo "  make $(COLOR_GREEN)<target>$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Development:$(COLOR_RESET)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ && !/^track-/ { printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(COLOR_BOLD)Test Track:$(COLOR_RESET)"
	@awk 'BEGIN {FS = ":.*##"} /^track-[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_YELLOW)%-15s$(COLOR_RESET) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Development
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: build
build: ## Build the binary
	@echo "$(COLOR_BLUE)▶ Building $(BINARY_NAME)...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/dorikin
	@echo "$(COLOR_GREEN)✓ Built $(BIN_DIR)/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: build-all
build-all: ## Build for all platforms
	@echo "$(COLOR_BLUE)▶ Building for all platforms...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/dorikin
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/dorikin
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/dorikin
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/dorikin
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/dorikin
	@echo "$(COLOR_GREEN)✓ Built all platforms in $(DIST_DIR)/$(COLOR_RESET)"

.PHONY: install
install: build ## Install to GOPATH/bin
	@echo "$(COLOR_BLUE)▶ Installing $(BINARY_NAME)...$(COLOR_RESET)"
	go install -ldflags "$(LDFLAGS)" ./cmd/dorikin
	@echo "$(COLOR_GREEN)✓ Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: run
run: build ## Run the TUI (development)
	@$(BIN_DIR)/$(BINARY_NAME) ui

.PHONY: run-scan
run-scan: build ## Run a scan (development)
	@$(BIN_DIR)/$(BINARY_NAME) scan -f testdata/manifests/

# ══════════════════════════════════════════════════════════════════════════════
# Testing
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: test
test: ## Run tests
	@echo "$(COLOR_BLUE)▶ Running tests...$(COLOR_RESET)"
	go test -race -shuffle=on ./...
	@echo "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

.PHONY: test-v
test-v: ## Run tests with verbose output
	@echo "$(COLOR_BLUE)▶ Running tests (verbose)...$(COLOR_RESET)"
	go test -race -shuffle=on -v ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage
	@echo "$(COLOR_BLUE)▶ Running tests with coverage...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	go test -race -shuffle=on -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out | tail -1

.PHONY: test-short
test-short: ## Run short tests only
	@echo "$(COLOR_BLUE)▶ Running short tests...$(COLOR_RESET)"
	go test -race -shuffle=on -short ./...

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(COLOR_BLUE)▶ Running benchmarks...$(COLOR_RESET)"
	go test -bench=. -benchmem ./...

# ══════════════════════════════════════════════════════════════════════════════
# Code Quality
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: lint
lint: ## Run linter
	@echo "$(COLOR_BLUE)▶ Running linter...$(COLOR_RESET)"
	$(GOLANGCI_LINT) run ./...
	@echo "$(COLOR_GREEN)✓ Linting passed$(COLOR_RESET)"

.PHONY: lint-fix
lint-fix: ## Run linter with auto-fix
	@echo "$(COLOR_BLUE)▶ Running linter with auto-fix...$(COLOR_RESET)"
	$(GOLANGCI_LINT) run --fix ./...
	@echo "$(COLOR_GREEN)✓ Linting complete$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format code
	@echo "$(COLOR_BLUE)▶ Formatting code...$(COLOR_RESET)"
	go fmt ./...
	$(GOLANGCI_LINT) fmt ./...
	@echo "$(COLOR_GREEN)✓ Formatting complete$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(COLOR_BLUE)▶ Running go vet...$(COLOR_RESET)"
	go vet ./...
	@echo "$(COLOR_GREEN)✓ Vet passed$(COLOR_RESET)"

.PHONY: tidy
tidy: ## Tidy go.mod
	@echo "$(COLOR_BLUE)▶ Tidying go.mod...$(COLOR_RESET)"
	go mod tidy
	@echo "$(COLOR_GREEN)✓ go.mod tidied$(COLOR_RESET)"

.PHONY: verify
verify: ## Verify dependencies
	@echo "$(COLOR_BLUE)▶ Verifying dependencies...$(COLOR_RESET)"
	go mod verify
	@echo "$(COLOR_GREEN)✓ Dependencies verified$(COLOR_RESET)"

# ══════════════════════════════════════════════════════════════════════════════
# CI Pipeline
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: ci
ci: tidy verify vet lint test build ## Run full CI pipeline
	@echo ""
	@echo "$(COLOR_GREEN)$(COLOR_BOLD)✓ CI pipeline passed!$(COLOR_RESET)"
	@echo ""

.PHONY: ci-fast
ci-fast: vet lint-fix test-short build ## Run fast CI (for local development)
	@echo "$(COLOR_GREEN)✓ Fast CI passed$(COLOR_RESET)"

# ══════════════════════════════════════════════════════════════════════════════
# Release
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: release-dry
release-dry: ## Dry run release
	@echo "$(COLOR_BLUE)▶ Running release dry-run...$(COLOR_RESET)"
	$(GORELEASER) release --snapshot --clean

.PHONY: release
release: ## Create release (requires GITHUB_TOKEN)
	@echo "$(COLOR_BLUE)▶ Creating release...$(COLOR_RESET)"
	$(GORELEASER) release --clean

# ══════════════════════════════════════════════════════════════════════════════
# Utilities
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(COLOR_BLUE)▶ Cleaning...$(COLOR_RESET)"
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	go clean -cache -testcache
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"

.PHONY: deps
deps: ## Download dependencies
	@echo "$(COLOR_BLUE)▶ Downloading dependencies...$(COLOR_RESET)"
	go mod download
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"

.PHONY: tools
tools: ## Install development tools
	@echo "$(COLOR_BLUE)▶ Installing tools...$(COLOR_RESET)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/goreleaser/goreleaser@latest
	@echo "$(COLOR_GREEN)✓ Tools installed$(COLOR_RESET)"

.PHONY: demo
demo: build ## Generate demo GIF (requires vhs, ffmpeg)
	@echo "$(COLOR_BLUE)▶ Generating demo GIF...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)  Resetting test track to baseline...$(COLOR_RESET)"
	@$(TEST_TRACK_SCRIPT) reset >/dev/null 2>&1
	@echo "$(COLOR_YELLOW)  Recording with VHS (this takes ~45s)...$(COLOR_RESET)"
	vhs assets/demo.tape
	@echo "$(COLOR_GREEN)✓ Demo saved to assets/demo.gif$(COLOR_RESET)"

.PHONY: version
version: ## Show version info
	@echo "$(COLOR_MAGENTA)Version:    $(VERSION)$(COLOR_RESET)"
	@echo "$(COLOR_MAGENTA)Commit:     $(COMMIT)$(COLOR_RESET)"
	@echo "$(COLOR_MAGENTA)Build Date: $(BUILD_DATE)$(COLOR_RESET)"
	@echo "$(COLOR_MAGENTA)Go Version: $(GO_VERSION)$(COLOR_RESET)"

.PHONY: info
info: ## Show project info
	@echo ""
	@echo "$(COLOR_BOLD)🏎️  dorikin$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Catch your Kubernetes configs drifting before they$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Tokyo Drift into production chaos.$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)Repository:$(COLOR_RESET) https://github.com/indrasvat/dorikin"
	@echo "$(COLOR_BLUE)Go Version:$(COLOR_RESET) $(GO_VERSION)"
	@echo "$(COLOR_BLUE)Build:$(COLOR_RESET)      $(VERSION) ($(COMMIT))"
	@echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Git Hooks
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: hooks
hooks: ## Install git hooks
	@echo "$(COLOR_BLUE)▶ Installing git hooks...$(COLOR_RESET)"
	@echo '#!/bin/sh\nmake ci-fast' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "$(COLOR_GREEN)✓ Git hooks installed$(COLOR_RESET)"

# ══════════════════════════════════════════════════════════════════════════════
# Test Track (Local K8s Testing)
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: track-setup
track-setup: build ## Setup test track (Colima k3s + resources)
	@$(TEST_TRACK_SCRIPT) setup

.PHONY: track-drift
track-drift: ## Apply drift scenarios (interactive)
	@$(TEST_TRACK_SCRIPT) drift

.PHONY: track-reset
track-reset: ## Reset test track (remove all drift)
	@$(TEST_TRACK_SCRIPT) reset

.PHONY: track-status
track-status: ## Show test track status
	@$(TEST_TRACK_SCRIPT) status

.PHONY: track-scan
track-scan: build ## Run dorikin scan on test track
	@$(TEST_TRACK_SCRIPT) scan

.PHONY: track-tui
track-tui: build ## Launch dorikin TUI on test track
	@$(TEST_TRACK_SCRIPT) tui

.PHONY: track-cleanup
track-cleanup: ## Destroy test track completely
	@$(TEST_TRACK_SCRIPT) cleanup

.PHONY: track-demo
track-demo: build ## Full demo: setup → baseline → drift → detect → TUI
	@echo ""
	@echo "$(COLOR_BOLD)🏎️ DORIKIN TEST TRACK DEMO$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)▶ Phase 1: Setup$(COLOR_RESET)"
	@$(TEST_TRACK_SCRIPT) setup
	@echo ""
	@echo "$(COLOR_BLUE)▶ Phase 2: Baseline Scan$(COLOR_RESET)"
	@sleep 2
	@$(TEST_TRACK_SCRIPT) scan || true
	@echo ""
	@echo "$(COLOR_BLUE)▶ Phase 3: Applying ALL Drift Scenarios$(COLOR_RESET)"
	@sleep 2
	@echo "A" | $(TEST_TRACK_SCRIPT) drift
	@echo ""
	@echo "$(COLOR_BLUE)▶ Phase 4: Post-Drift Scan$(COLOR_RESET)"
	@sleep 2
	@$(TEST_TRACK_SCRIPT) scan || true
	@echo ""
	@echo "$(COLOR_BLUE)▶ Phase 5: Launching TUI$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)Press 'q' to quit the TUI.$(COLOR_RESET)"
	@sleep 3
	@$(TEST_TRACK_SCRIPT) tui
	@echo ""
	@echo "$(COLOR_GREEN)$(COLOR_BOLD)✓ Demo complete!$(COLOR_RESET)"
	@echo ""

.PHONY: track-quick
track-quick: build ## Quick test: setup → drift-all → scan
	@$(TEST_TRACK_SCRIPT) setup
	@echo "A" | $(TEST_TRACK_SCRIPT) drift
	@$(TEST_TRACK_SCRIPT) scan || true
