.PHONY: build test install clean

BINARY_NAME=prow-helper
BUILD_DIR=bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/prow-helper

test:
	go test -v ./...

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/ 2>/dev/null || cp $(BUILD_DIR)/$(BINARY_NAME) ~/go/bin/

clean:
	rm -rf $(BUILD_DIR)
	go clean -testcache
