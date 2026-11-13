REPO_ROOT := $(abspath ../..)
include ../../build/tools.mk

API_DIR := ../../api
API_PROTO_DIR := $(API_DIR)/protos
GOOGLEAPIS_DIR := ../../third_party/googleapis
CONFIG_DIR := internal/conf
API_PROTOS := $(wildcard $(API_PROTO_DIR)/proxycontrol/*/*.proto)
CONFIG_PROTOS := $(wildcard $(CONFIG_DIR)/protos/*.proto)

.PHONY: build pb api config dao lint wire
build: | bin
	go build -ldflags "$(GO_LDFLAGS)" -o bin/proxy-control$(EXE) ./cmd

bin:
	$(MKDIR) bin

pb: api config

api:
	$(PROTOC) -I "$(API_PROTO_DIR)" -I "$(GOOGLEAPIS_DIR)" -I "$(PROTOC_INCLUDE)" \
		--go_out=paths=source_relative:"$(API_DIR)" \
		--go-http_out=paths=source_relative:"$(API_DIR)" \
		--openapi_out=fq_schema_naming=true,default_response=false:"$(API_DIR)/proxycontrol" \
		$(API_PROTOS)

config:
	$(PROTOC) -I "$(CONFIG_DIR)/protos" -I "$(PROTOC_INCLUDE)" --go_out="$(CONFIG_DIR)" --go_opt=paths=source_relative $(CONFIG_PROTOS)

dao:
	go run ./internal/data/gen/gen.go

lint:
	$(GOLANGCI_LINT) run ./... ../../api/proxycontrol/...

wire:
	$(WIRE) ./cmd
