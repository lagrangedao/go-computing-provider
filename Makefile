SHELL=/usr/bin/env bash

project_name=computing-provider

cpRepo=$(shell env | grep CP_PATH | awk -F= '{print $$2}')
ifeq ($(cpRepo),)
$(error CP_PATH is not set. Please set it using: export CP_PATH=xxx)
else
$(info CP_PATH is set to $(cpRepo))
endif

unexport GOFLAGS

GOCC?=go

ldflags=-X=main.CurrentCommit=+git.$(subst -,.,$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null))
GOFLAGS+=-ldflags="$(ldflags)"

build: build/.get-model
	rm -rf $(project_name)
	$(GOCC) build $(GOFLAGS) -o $(project_name) main.go
.PHONY: build

build/.get-model:
	if [ ! -d $(cpRepo)/inference-model ]; then \
    	mkdir -p $(cpRepo)/inference-model; \
    fi
	git clone https://github.com/sonic-chain/api-inference-community.git $(cpRepo)/inference-model
	cd $(cpRepo)/inference-model && git checkout fea-lag-code

install:
	install -C $(project_name) /usr/local/bin/$(project_name)

clean:
	rm -rf $(cpRepo)/inference-model
	rm -rf /usr/local/bin/$(project_name)
.PHONY: clean