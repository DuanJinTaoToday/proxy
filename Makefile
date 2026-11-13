REPO_ROOT := $(CURDIR)
include build/tools.mk

export GOBIN := $(TOOLS_BIN)

PROTOBUF_GO_VERSION := v1.36.12
KRATOS_HTTP_VERSION := v3.0.0-20260526000039-30da04b769dc
GNOSTIC_VERSION := v0.7.1
GOLANGCI_LINT_VERSION := v2.15.0
GOOSE_VERSION := v3.30.0
WIRE_VERSION := v0.7.0

.PHONY: init
init:
	go run ./build/install-protoc "$(REPO_ROOT)/.tools"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOBUF_GO_VERSION)
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@$(KRATOS_HTTP_VERSION)
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@$(GNOSTIC_VERSION)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)

TEST_COMPOSE := docker compose -f deploy/test/compose.yaml

.PHONY: test-env
test-env:
	$(TEST_COMPOSE) build --build-arg GOOSE_VERSION=$(GOOSE_VERSION)
	$(TEST_COMPOSE) up -d --wait --wait-timeout 60 db
	$(TEST_COMPOSE) run --rm migrate
	$(TEST_COMPOSE) up -d --remove-orphans --wait --wait-timeout 120 console gateway
	$(TEST_COMPOSE) ps
