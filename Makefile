.PHONY: build test clean lint run scan

APP_NAME := sysscope
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X sysscope/cmd.version=$(VERSION) -X sysscope/cmd.buildDate=$(BUILD_DATE)

# Build for current platform
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) .

# Build all platforms
build-all: build-windows build-linux build-darwin

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-windows-amd64.exe .

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-linux-amd64 .

build-darwin:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-darwin-arm64 .

test:
	go test -race -coverprofile=coverage.out ./...

test-verbose:
	go test -v -race ./...

coverage:
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ coverage.out coverage.html sysscope_report_*.json sysscope_report_*.html

run: build
	./bin/$(APP_NAME) scan --format json

scan-html: build
	./bin/$(APP_NAME) scan --format html

compare:
	@echo "Usage: make compare FILE1=report1.json FILE2=report2.json"
	./bin/$(APP_NAME) compare $(FILE1) $(FILE2)
