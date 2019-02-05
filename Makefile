GITIN_VERSION=$(shell git describe --long --tags --dirty --always --match=v*.*.* 2>/dev/null || echo 'Unknown')
GITIN_BUILD_DATETIME=$(shell date '+%Y-%m-%d %H:%M:%S %Z')

GOCMD=go

BINARY?=gitin
GITIN_SOURCE_DIR=.
GITIN_LDFLAGS=-X 'main.version=$(GITIN_VERSION)' -X 'main.buildDateTime=$(GITIN_BUILD_DATETIME)'
GITIN_STATIC_LDFLAGS=-extldflags '-lncurses -ltinfo -lgpm -static'
GITIN_BUILD_FLAGS=--tags static -ldflags "$(GITIN_LDFLAGS)"
GITIN_STATIC_BUILD_FLAGS=--tags static -ldflags "$(GITIN_LDFLAGS) $(GITIN_STATIC_LDFLAGS)"

GITIN_DIR:=$(dir $(realpath $(lastword $(MAKEFILE_LIST))))
GOPATH_DIR:=$(shell go env GOPATH)
GOBIN_DIR:=$(GOPATH_DIR)/bin

GIT2GO_VERSION=27
GIT2GO_DIR:=$(GOPATH_DIR)/src/gopkg.in/libgit2/git2go.v$(GIT2GO_VERSION)
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
update:
	git submodule -q foreach --recursive git reset -q --hard
	git submodule update --init --recursive

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
