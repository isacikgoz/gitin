GOCMD=go

BINARY?=gitin
GITIN_SOURCE_DIR=./cmd/gitin

GOPATH_DIR?=$(shell go env GOPATH | cut -d: -f1)
GOBIN_DIR:=$(GOPATH_DIR)/bin

all: $(BINARY)

.PHONY: $(BINARY)
$(BINARY):
	CGO_ENABLED=0 $(GOCMD) build -o $(BINARY) $(GITIN_SOURCE_DIR)

# kept for existing build scripts, the binary is always statically linked
.PHONY: static
static: $(BINARY)

.PHONY: install
install: $(BINARY)
	install -m755 -d $(GOBIN_DIR)
	install -m755 $(BINARY) $(GOBIN_DIR)

.PHONY: test
test:
	$(GOCMD) test -race ./...

# coverage of the unit tests and of the gitin processes the e2e tests run
.PHONY: coverage
coverage:
	rm -rf coverage && mkdir -p coverage/unit coverage/e2e
	$(GOCMD) test -race -cover -coverpkg=./... $$($(GOCMD) list ./... | grep -v /e2e$$) -args -test.gocoverdir=$(CURDIR)/coverage/unit
	GITIN_E2E_COVERDIR=$(CURDIR)/coverage/e2e $(GOCMD) test -race ./e2e/
	$(GOCMD) tool covdata textfmt -i=coverage/unit,coverage/e2e -o coverage/coverage.txt
	$(GOCMD) tool covdata percent -i=coverage/unit,coverage/e2e
	$(GOCMD) tool cover -func=coverage/coverage.txt | tail -n 1

# release archives and checksums for every platform, VERSION is the git tag
VERSION?=dev
DIST_TARGETS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: dist
dist:
	rm -rf dist build && mkdir -p dist
	@for target in $(DIST_TARGETS); do \
		os=$${target%/*}; arch=$${target#*/}; dir=build/$${os}_$${arch}; \
		echo "building $$target"; \
		mkdir -p $$dir && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GOCMD) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $$dir/gitin $(GITIN_SOURCE_DIR) && \
		tar -czf dist/gitin_$(VERSION)_$${os}_$${arch}.tar.gz -C $$dir gitin -C $(CURDIR) README.md LICENSE || exit 1; \
	done
	cd dist && (sha256sum *.tar.gz 2>/dev/null || shasum -a 256 *.tar.gz) > checksums.txt

.PHONY: clean
clean:
	rm -rf $(BINARY) coverage dist build
