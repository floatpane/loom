.PHONY: build test run clean lint fmt vet test-verbose test-coverage install build-full all

INSTALL_DIR ?= /usr/local/bin

BINARY_NAME=loom
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

build-full:
	@echo "Building with version information..."
	@VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	echo "Version: $$VERSION"; \
	echo "Commit: $$COMMIT"; \
	echo "Date: $$DATE"; \
	go build -trimpath -ldflags="-s -w -X 'main.version=$$VERSION' -X 'main.commit=$$COMMIT' -X 'main.date=$$DATE'" -o $(BUILD_DIR)/$(BINARY_NAME) .

install:
	@echo "Building and installing $(BINARY_NAME)..."
	@EXISTING=$$(which $(BINARY_NAME) 2>/dev/null); \
	DEST=$$([ -n "$$EXISTING" ] && dirname "$$EXISTING" || echo "$(INSTALL_DIR)"); \
	VERSION=$$([ -n "$(VERSION)" ] && echo "$(VERSION)" || git describe --tags --abbrev=0 2>/dev/null || echo "dev"); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	echo "Version: $$VERSION"; \
	echo "Commit: $$COMMIT"; \
	echo "Date: $$DATE"; \
	go build -trimpath -ldflags="-s -w -X 'main.version=$$VERSION' -X 'main.commit=$$COMMIT' -X 'main.date=$$DATE'" -o $(BUILD_DIR)/$(BINARY_NAME) .; \
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) "$$DEST/$(BINARY_NAME)"; \
	echo "Installed to $$DEST/$(BINARY_NAME)"

run:
	go run .

test:
	go test ./...

test-verbose:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

all: lint test build
