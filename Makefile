BINARY := box
PKG := ./cmd/cli
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: all build run test vet fmt lint tidy clean install uninstall

all: build

alpine:
	./scripts/create-alpine.sh

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
	golangci-lint fmt ./...

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

# CONTAINER SPECIFIC SUB CMDS W EX ARGS
CONTAINER := mycontainer
IMG := alpinefs

create:
	go run $(PKG) create --bundle $(IMG) $(CONTAINER) 

start:
	go run $(PKG) start $(CONTAINER)

state:
	go run $(PKG) state $(CONTAINER) | jq

state-broken:
	go run $(PKG) state 26ede6c5-5103-405b-ac03-348ffc42e35d 

kill:
	go run $(PKG) kill $(CONTAINER) 9

delete:
	go run $(PKG) delete $(CONTAINER)

force-delete:
	go run $(PKG) delete --force $(CONTAINER)
