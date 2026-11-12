REPO_ROOT := $(abspath ../..)
include ../../build/tools.mk

CONFIG_DIR := internal/conf
CONFIG_PROTOS := $(wildcard $(CONFIG_DIR)/protos/*.proto)

.PHONY: build config lint wire
build: | bin
	go build -ldflags "$(GO_LDFLAGS)" -o bin/proxy-gateway$(EXE) ./cmd

bin:
	$(MKDIR) bin

config:
	$(PROTOC) -I "$(CONFIG_DIR)/protos" -I "$(PROTOC_INCLUDE)" --go_out="$(CONFIG_DIR)" --go_opt=paths=source_relative $(CONFIG_PROTOS)

lint:
	$(GOLANGCI_LINT) run ./...

wire:
	$(WIRE) ./cmd
