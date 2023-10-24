SHELL=/usr/bin/env bash

cpRepo := $(shell echo $$CP_PATH)

project_name=computing-provider

unexport GOFLAGS

GOCC?=go

ldflags=-X=main.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
GOFLAGS+=-ldflags="$(ldflags)"

build: get-model
	rm -rf $(project_name)
	$(GOCC) build $(GOFLAGS) -o $(project_name) main.go
.PHONY: build

check_path:
ifeq ($(cpRepo),)
$(error CP_PATH is not set. Please set it using: export CP_PATH=xxx)
else
$(info CP_PATH is set to $(cpRepo))
endif
.PHONY: check_path

get-model: check_path
ifeq (,$(wildcard $(cpRepo)/inference-model))
	mkdir -p $(cpRepo)/inference-model
endif
	git clone https://github.com/sonic-chain/api-inference-community.git $(cpRepo)/inference-model
	cd $(cpRepo)/inference-model && git checkout fea-lag-transformer
	pip install -r requirements.txt
.PHONY: get-model

install:
	sudo install -C $(project_name) /usr/local/bin/$(project_name)

clean:
	rm -rf $(cpRepo)/inference-model
	sudo rm -rf /usr/local/bin/$(project_name)
.PHONY: clean