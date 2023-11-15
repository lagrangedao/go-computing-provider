SHELL=/usr/bin/env bash

cpRepo := $(shell echo $$CP_PATH)

project_name=computing-provider

unexport GOFLAGS

GOCC?=go

ldflags=-X=github.com/lagrangedao/go-computing-provider/build.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
GOFLAGS+=-ldflags="$(ldflags)"

all: build
.PHONY: all

build: get-model computing-provider
.PHONY: build

get-model: check_path
ifeq (,$(wildcard $(cpRepo)/inference-model))
	mkdir -p $(cpRepo)/inference-model
endif
	git clone https://github.com/lagrangedao/api-inference-community.git $(cpRepo)/inference-model
	cd $(cpRepo)/inference-model && git checkout fea-lag-transformer && pip install -r requirements.txt
.PHONY: get-model

check_path:
ifeq ($(cpRepo),)
$(error CP_PATH is not set. Please set it using: export CP_PATH=xxx)
else
$(info CP_PATH is set to $(cpRepo))
endif
.PHONY: check_path

computing-provider:
	rm -rf computing-provider
	$(GOCC) build $(GOFLAGS) -o computing-provider ./cmd/computing-provider
.PHONY: computing-provider

install:
	sudo install -C computing-provider /usr/local/bin/computing-provider

clean:
	rm -rf $(cpRepo)/inference-model
	sudo rm -rf /usr/local/bin/computing-provider
.PHONY: clean