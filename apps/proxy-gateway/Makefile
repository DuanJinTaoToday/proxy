REPO_ROOT := $(abspath ../..)
include ../../build/tools.mk

.PHONY: build lint wire
build: | bin
	go build -o bin/proxy-gateway$(EXE) ./cmd

bin:
	$(MKDIR) bin

lint:
	$(GOLANGCI_LINT) run ./...

wire:
	$(WIRE) ./cmd
