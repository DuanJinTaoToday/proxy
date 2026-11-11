REPO_ROOT := $(abspath ../..)
include ../../build/tools.mk

.PHONY: build lint
build: | bin
	go build -o bin/proxy-gateway-collector$(EXE) ./collector

bin:
	$(MKDIR) bin

lint:
	$(GOLANGCI_LINT) run ./...
