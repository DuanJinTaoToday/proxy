GO_VERSION := go1.27.4
TOOLS_BIN := $(REPO_ROOT)/.tools/bin
export GOTOOLCHAIN := $(GO_VERSION)

ifeq ($(OS),Windows_NT)
EXE := .exe
PATH_SEPARATOR := ;
MKDIR := cmd /c mkdir
else
PATH_SEPARATOR := :
MKDIR := mkdir -p
endif

export PATH := $(TOOLS_BIN)$(PATH_SEPARATOR)$(PATH)
PROTOC := "$(TOOLS_BIN)/protoc$(EXE)"
PROTOC_INCLUDE := $(REPO_ROOT)/.tools/include
GOLANGCI_LINT := "$(TOOLS_BIN)/golangci-lint$(EXE)"
WIRE := "$(TOOLS_BIN)/wire$(EXE)"

GO_LDFLAGS ?=
