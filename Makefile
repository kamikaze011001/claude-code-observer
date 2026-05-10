.PHONY: build test vet lint clean run

BIN_DIR := bin
BIN := $(BIN_DIR)/claude-code-observer
PKG := ./...
LDFLAGS := -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
           -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/app

test:
	go test $(PKG)

test-cover:
	go test -cover $(PKG)

vet:
	go vet $(PKG)

lint:
	golangci-lint run

run: build
	$(BIN)

clean:
	rm -rf $(BIN_DIR)
