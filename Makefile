VERSION := $(shell git describe --tags --always)

GOLDFLAGS += -X aggregator/internal/version.Version=$(VERSION)
GOFLAGS = -ldflags "$(GOLDFLAGS)"

all: build

.PHONY: build
build:
	go build $(GOFLAGS) -o ./build/ ./cmd/aggregator
clean:
	rm -rf build/*
