BINARY := box
PKG := ./cmd/cli
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: all build run test vet fmt lint tidy clean install uninstall

all: build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(PKG)

run:
	go run $(PKG) $(ARGS)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

install: build
	install -Dm755 $(BIN_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)
