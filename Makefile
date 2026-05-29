BINARY_NAME := bot
BUILD_DIR := bin
GO := go
GOFLAGS := -v
MAIN_PATH := ./cmd/bot

.DEFAULT_GOAL := build

.PHONY: run
run:
	@echo "Running $(BINARY_NAME)..."
	$(GO) run $(MAIN_PATH)

.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf data/

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: lint
lint: vet fmt tidy

.PHONY: sqlc
sqlc:
	sqlc generate
