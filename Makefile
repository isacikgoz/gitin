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

.PHONY: clean
clean:
	rm -f $(BINARY)
