VERSION := $(shell git describe --tags --always)

GOLDFLAGS += -X aggregator/internal/version.Version=$(VERSION)
GOFLAGS = -ldflags "$(GOLDFLAGS)"

all: build

.PHONY: build

build:
	go build $(GOFLAGS) -o ./out/ ./cmd/aggregator

clean:
	rm -rf build/*

static:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./out/aggregator ./cmd/aggregator
