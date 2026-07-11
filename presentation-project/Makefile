BINARY := demo
PKG := ./cmd/demo
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: run, build, setup-node

run:
	@go run $(PKG) run "/bin/bash" "node"

reexec:
	@echo "warning: this is for testing"
	@echo "this should only be called internally"
	@go run $(PKG) reexec "/bin/bash" "node"

build:
	@go build -o $(BINARY) $(PKG)

setup-node:
	@bash scripts/setup.sh
