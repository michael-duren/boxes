BINARY := demo
PKG := ./cmd/demo
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

.PHONY: run, build, setup-node

run:
	go run $(PKG) $(ARGS)

build:
	@go build -o $(BINARY) $(PKG)

setup-node:
	@bash scripts/setup.sh
