BINARY := box
PKG := ./cmd/cli
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: all build run test vet fmt lint tidy clean install uninstall

all: build

ctr: 
	@./scripts/run-in-ctr.sh

alpine:
	@./scripts/create-alpine.sh

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(PKG)

run:
	go run $(PKG) $(ARGS)

test:
	go test ./...

runtime-test:
	@./scripts/oci-validation.sh

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

console:
	tail -f "$$XDG_STATE_HOME/boxes/logs/boxes.log"

# CONTAINER SPECIFIC SUB CMDS W EX ARGS
CONTAINER := mycontainer
IMG := alpinefs

create:
	go run $(PKG) --debug create --bundle $(IMG) $(CONTAINER) 

start:
	go run $(PKG) --debug start $(CONTAINER)

state:
	go run $(PKG) --debug state $(CONTAINER) | jq

state-broken:
	go run $(PKG) --debug state 26ede6c5-5103-405b-ac03-348ffc42e35d 

kill:
	go run $(PKG) --debug kill $(CONTAINER) 9

delete:
	go run $(PKG) --debug delete $(CONTAINER)

force-delete:
	go run $(PKG) --debug delete --force $(CONTAINER)
