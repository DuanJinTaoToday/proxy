REPO_ROOT := $(abspath ../..)
include ../../build/tools.mk

API_DIR := ../../api
GOOGLEAPIS_DIR := ../../third_party/googleapis
CONFIG_DIR := internal/conf
API_PROTOS := $(wildcard $(API_DIR)/proxycontrol/*/*.proto)
CONFIG_PROTOS := $(wildcard $(CONFIG_DIR)/protos/*.proto)

.PHONY: build pb api config dao lint wire
build: | bin
	go build -o bin/proxy-control$(EXE) ./cmd

bin:
	$(MKDIR) bin

pb: api config

api:
	$(PROTOC) -I "$(API_DIR)" -I "$(GOOGLEAPIS_DIR)" -I "$(PROTOC_INCLUDE)" --go_out="$(API_DIR)" --go_opt=paths=source_relative --go-http_out="$(API_DIR)" --go-http_opt=paths=source_relative $(API_PROTOS)

config:
	$(PROTOC) -I "$(CONFIG_DIR)/protos" -I "$(PROTOC_INCLUDE)" --go_out="$(CONFIG_DIR)" --go_opt=paths=source_relative $(CONFIG_PROTOS)

dao:
	go run ./tools/dao

lint:
	$(GOLANGCI_LINT) run ./... ../../api/proxycontrol/...

wire:
	$(WIRE) ./cmd
