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

.PHONY: clean
clean:
	rm -f $(BINARY)
