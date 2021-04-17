GOCMD=go

BINARY?=gitin
GITIN_SOURCE_DIR=cmd/gitin/main.go
GITIN_BUILD_FLAGS=--tags static

GITIN_DIR:=$(dir $(realpath $(lastword $(MAKEFILE_LIST))))
GOPATH_DIR?=$(shell go env GOPATH | cut -d: -f1)
GOBIN_DIR:=$(GOPATH_DIR)/bin

GIT2GO_VERSION=30
PARENT_DIR=$(realpath $(GITIN_DIR)../)
GIT2GO_DIR:=$(PARENT_DIR)/git2go
LIBGIT2_DIR=$(GIT2GO_DIR)/vendor/libgit2
GIT2GO_PATCH=patch/git2go.v$(GIT2GO_VERSION).patch

all: $(BINARY)

$(BINARY): build-libgit2
	$(GOCMD) build $(GITIN_BUILD_FLAGS) -o $(BINARY) $(GITIN_SOURCE_DIR)

.PHONY: build-only
build-only:
	make -C $(GIT2GO_DIR) install-static
	$(GOCMD) build $(GITIN_BUILD_FLAGS) -o $(BINARY) $(GITIN_SOURCE_DIR)

.PHONY: build-libgit2
build-libgit2: apply-patches
	make -C $(GIT2GO_DIR) install-static

.PHONY: install
install: $(BINARY)
	install -m755 -d $(GOBIN_DIR)
	install -m755 $(BINARY) $(GOBIN_DIR)

.PHONY: update
update: check-git2go
	git clone https://github.com/libgit2/git2go.git $(GIT2GO_DIR)
	cd $(GIT2GO_DIR) && git checkout v30.0.9
	cd $(GIT2GO_DIR) && git submodule -q foreach --recursive git reset -q --hard
	cd $(GIT2GO_DIR) && git submodule update --init --recursive

check-git2go:
	@if [ "$(FORCE)" == "YES" ]; then \
		echo "removing by force"; \
		rm -rf $(GIT2GO_DIR); \
	elif [ -d "$(GIT2GO_DIR)" ]; then  \
		echo "$(GIT2GO_DIR) will be deleted, are you sure? [y/N] " && read ans && [ $${ans:-N} = y ]; \
		if [ $$ans = y ] || [ $$ans = Y ]  ; then \
			rm -rf $(GIT2GO_DIR); \
		fi; \
	fi; \

.PHONY: apply-patches
apply-patches: update
	if patch --dry-run -N -d $(GIT2GO_DIR) -p1 < $(GIT2GO_PATCH) >/dev/null; then \
		patch -d $(GIT2GO_DIR) -p1 < $(GIT2GO_PATCH); \
	fi

.PHONY: static
static: build-libgit2
	$(GOCMD) build $(GITIN_STATIC_BUILD_FLAGS) -o $(BINARY) $(GITIN_SOURCE_DIR)

.PHONY: clean
clean:
	rm -f $(BINARY)
