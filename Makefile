BINARY  := passauto
GOFLAGS := -trimpath

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
	-X github.com/lifefinity/passauto/cmd.Version=$(VERSION) \
	-X github.com/lifefinity/passauto/cmd.Commit=$(COMMIT) \
	-X github.com/lifefinity/passauto/cmd.BuildDate=$(DATE)

.PHONY: all deps build run test fmt vet lint sec vuln check clean install

all: check build

## deps: download and tidy Go modules
deps:
	go mod download
	go mod tidy

## build: compile binary to bin/
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY).exe .

## run: run via go run
run:
	go run .

## test: run all tests
test:
	go test ./... -v -count=1

## fmt: format code
fmt:
	go fmt ./...

## vet: static analysis
vet:
	go vet ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## sec: security scan
sec:
	gosec ./...

## vuln: vulnerability check
vuln:
	govulncheck ./...

## check: run all checks (fmt + vet + lint + sec + vuln + test)
check: fmt vet lint sec vuln test

## clean: remove build artifacts
clean:
	rm -rf bin/
	rm -f $(BINARY).exe

## install: install binary to GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .
