.PHONY: build test lint install clean

APP = isdomaintaken
BUILD_DIR = bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP) ./cmd/$(APP)

test:
	go test ./...

lint:
	golangci-lint run ./...

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/$(APP)

clean:
	rm -rf $(BUILD_DIR)
