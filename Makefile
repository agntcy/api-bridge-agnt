# Copyright AGNTCY Contributors (https://github.com/agntcy)
# SPDX-License-Identifier: Apache-2.0

# Build settings
TYK_VERSION ?= v5.8.0
# Options: linux, darwin
TARGET_OS   ?= darwin
# Options: amd64, arm64
TARGET_ARCH ?= arm64

# Version for https://github.com/kelindar/search.git
SEARCH_VERSION := v0.4.0
SEARCH_LIB     ?= libllama_go.dylib

# Plugin configuration
PLUGIN_NAME        := agent-bridge-plugin
FULL_PLUGIN_NAME   := $(PLUGIN_NAME)_$(TYK_VERSION)_$(TARGET_OS)_$(TARGET_ARCH)
TYK_COMPILER_IMAGE_PLATFORM := linux/amd64
TYK_COMPILER_IMAGE := tykio/tyk-plugin-compiler:v$(TYK_VERSION)

PROJECT_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
REQUIRED_BINS := docker go git curl jq cmake
$(foreach bin,$(REQUIRED_BINS),\
    $(if $(shell command -v $(bin) 2> /dev/null),,$(error Please install `$(bin)`)))

ifeq ($(TARGET_OS), linux)
    LIB_ENV_VAR = LD_LIBRARY_PATH
else
    LIB_ENV_VAR = DYLD_LIBRARY_PATH
endif

.PHONY: default all build_release build setup clean setup \
  build_plugin check_plugin install_plugin load_plugin \
  test_plugin_select build_search_lib

default: install_plugin
all: build_release

# Builds a release version of the plugin using the official Tyk plugin compiler
build_release:
	docker run --rm \
	  --platform=$(TYK_COMPILER_IMAGE_PLATFORM) \
	  --mount type=bind,src=./plugins,dst=/plugin-source \
	  $(TYK_COMPILER_IMAGE) $(PLUGIN_NAME) _$$(date +%s) $(TARGET_OS) $(TARGET_ARCH)

test: search-release-$(SEARCH_VERSION)/build/lib/$(SEARCH_LIB)
	$(LIB_ENV_VAR)=$(PROJECT_ROOT)/search-release-$(SEARCH_VERSION)/build/lib go test -v ./plugins

clean:
	rm -f go/src/$(PLUGIN_NAME).so
	rm -f go/src/$(PLUGIN_NAME)_*.so

setup tyk-release-$(TYK_VERSION)/go.mod:
	git clone --branch $(TYK_VERSION) --depth 1 https://github.com/TykTechnologies/tyk.git tyk-release-$(TYK_VERSION) || true
	cd tyk-release-$(TYK_VERSION) && sed -i.bak 's/.*Version = ".*"/       Version = "$(TYK_VERSION)-dev"/g' internal/build/version.go

	# set the go version of the plugin to the one used by tyk
	/bin/sh -c 'tyk_go_version=`go mod edit -json tyk-release-$(TYK_VERSION)/go.mod | jq -r .Go` && cd plugins && go mod edit -go $${tyk_go_version}'
	rm go.work
	go work init ./tyk-release-$(TYK_VERSION)
	go work use ./plugins
	/bin/sh -c 'commit_hash=`cd tyk-release-$(TYK_VERSION) && git rev-parse HEAD` && cd plugins && go get github.com/TykTechnologies/tyk@$${commit_hash}'
	cd plugins && go mod tidy -go=$$(go mod edit -json ../tyk-release-$(TYK_VERSION)/go.mod | jq -r .Go)

search-release-$(SEARCH_VERSION)/README.md: download_models_for_semrouter
	@git lfs status || { echo "Error: you must install git-lfs from https://git-lfs.com/ and then enable it: git lfs install [--local]" ; false ; }
	git clone --branch $(SEARCH_VERSION) --depth 1 https://github.com/kelindar/search.git search-release-$(SEARCH_VERSION) || true
	cd "search-release-$(SEARCH_VERSION)" && git lfs install --local
	cd "search-release-$(SEARCH_VERSION)" && git submodule update --init --recursive
	cd "search-release-$(SEARCH_VERSION)" && git lfs pull

build_search_lib search-release-$(SEARCH_VERSION)/build/lib/$(SEARCH_LIB): search-release-$(SEARCH_VERSION)/README.md
	-mkdir -p "search-release-$(SEARCH_VERSION)/build"
	cd "search-release-$(SEARCH_VERSION)/build" && cmake -DCMAKE_BUILD_TYPE=Release ..
	cd "search-release-$(SEARCH_VERSION)/build" && cmake --build . --config Release

tyk-release-$(TYK_VERSION)/$(SEARCH_LIB): search-release-$(SEARCH_VERSION)/build/lib/$(SEARCH_LIB)
	cp -p "$<" "$@"
	-mkdir -p "tyk-release-$(TYK_VERSION)/models"
	cp -p search-release-$(SEARCH_VERSION)/dist/*.gguf "tyk-release-$(TYK_VERSION)/models"

build_tyk tyk-release-$(TYK_VERSION)/tyk: tyk-release-$(TYK_VERSION)/go.mod
	cd tyk-release-$(TYK_VERSION) && GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go build -tags=goplugin -trimpath .

start_tyk: tyk-release-$(TYK_VERSION)/tyk tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so tyk-release-$(TYK_VERSION)/$(SEARCH_LIB)
	if [ -f .env ]; then export $$(grep -v '^[:space:]*#' .env | xargs); fi ; \
	  cd tyk-release-$(TYK_VERSION) && TYK_LOGLEVEL=debug ./tyk start --conf=../configs/tyk.conf

start_redis: deploy/docker-compose.yaml
	docker compose -f deploy/docker-compose.yaml up --detach

build_plugin plugins/$(FULL_PLUGIN_NAME).so: plugins/*.go plugins/go.mod tyk-release-$(TYK_VERSION)/go.mod
	cd plugins && go mod tidy -go=$$(go mod edit -json ../tyk-release-$(TYK_VERSION)/go.mod | jq -r .Go)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go build -C plugins -trimpath -buildmode=plugin -o $(FULL_PLUGIN_NAME).so .

install_plugin tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so: plugins/$(FULL_PLUGIN_NAME).so
	cp plugins/$(FULL_PLUGIN_NAME).so ./tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so

check_plugin: plugins/$(FULL_PLUGIN_NAME).so
	if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi ; \
	  ./tyk-release-$(TYK_VERSION)/tyk plugin load -f plugins/$(FULL_PLUGIN_NAME).so -s RewriteQueryToOas && \
	  ./tyk-release-$(TYK_VERSION)/tyk plugin load -f plugins/$(FULL_PLUGIN_NAME).so -s RewriteResponseToNl

load_plugin: configs/httpbin.org.oas.json tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so
	curl http://localhost:8080/tyk/apis/oas --header "x-tyk-authorization: foo" --header 'Content-Type: text/plain' -d@configs/httpbin.org.oas.json && sleep 3
	curl http://localhost:8080/tyk/reload/group --header "x-tyk-authorization: foo"

bundle: plugins/$(FULL_PLUGIN_NAME).so
	sed 's/agent-bridge-plugin.so/$(FULL_PLUGIN_NAME).so/g' configs/manifest.json > configs/$(FULL_PLUGIN_NAME).manifest.json
	cd plugins && ../tyk-release-$(TYK_VERSION)/tyk bundle build --skip-signing --manifest "../configs/$(FULL_PLUGIN_NAME).manifest.json" --output "../$(FULL_PLUGIN_NAME)_bundle.zip"

test_plugin:
	curl -vv http://localhost:8080/httpbin_tyk/json -H "X-Nl-Query-Enabled: yes" -H "X-Nl-Response-Type: nl" -H "Content-Type: text/plain" -d "Hello"

load_plugin_select: configs/httpbin.org.api-selection.json tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so
	curl http://localhost:8080/tyk/apis/oas --header "x-tyk-authorization: foo" --header 'Content-Type: text/plain' -d@configs/httpbin.org.api-selection.json && sleep 3
	curl http://localhost:8080/tyk/reload/group --header "x-tyk-authorization: foo"

test_plugin_select: configs/httpbin.org.api-selection.json tyk-release-$(TYK_VERSION)/middleware/agent-bridge-plugin.so
	curl -vv 'http://localhost:8080/httpbin_select/?query=I%20would%20like%20an%20XML%20response.' --header 'Content-Type: text/plain'

download_models_for_semrouter:
ifeq (,$(wildcard ./tyk-release-$(TYK_VERSION)/models/jina-embeddings-v2-base-en-q5_k_m.gguf))
	mkdir -p "tyk-release-$(TYK_VERSION)/models"
	curl -L 'https://huggingface.co/djuna/jina-embeddings-v2-base-en-Q5_K_M-GGUF/resolve/main/jina-embeddings-v2-base-en-q5_k_m.gguf' -o "tyk-release-$(TYK_VERSION)/models/jina-embeddings-v2-base-en-q5_k_m.gguf"
endif


lint:
	golangci-lint run --timeout=10m plugins/
