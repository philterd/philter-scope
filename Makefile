BINARY_AUDIT=philterscope-audit
BINARY_SERVE=philterscope-serve
CMD_AUDIT_PATH=./cmd/philterscope-audit
CMD_SERVE_PATH=./cmd/philterscope-serve
SOURCE_FILES=$(shell find . -name '*.go')
HTML_FILES=$(shell find . -name '*.html')

# Reported by the server's health endpoint. Derived from the current git
# tag/commit; override with `make build VERSION=1.2.3`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS=-X github.com/philterd/philterscope/internal/server.Version=$(VERSION)

all: build

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

audit: build ## Run audit on example files
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./$(BINARY_AUDIT) --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --group "CLI-Audit"
	xdg-open ./examples/report.html

audit-mongodb-ai: build ## Run audit on example files with MongoDB storage and AI policy suggestions
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope PHILTERSCOPE_OLLAMA_URL=http://localhost:11434 ./$(BINARY_AUDIT) --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --ai

audit-mongodb: build ## Run audit on example files using Docker Compose
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./$(BINARY_AUDIT) --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75

audit-mongodb-thresholds: build ## Run audit on example files with MongoDB storage and specific entity thresholds
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./$(BINARY_AUDIT) --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --thresholds "NAME=0.9,SSN=1.0"

serve: build ## Launch Evaluation UI using MongoDB storage
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./$(BINARY_SERVE) #--privacy

deps: ## Download and tidy Go dependencies
	go mod download
	go mod tidy

build: deps $(SOURCE_FILES) $(HTML_FILES) ## Build the philterscope binaries
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUDIT) $(CMD_AUDIT_PATH)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVE) $(CMD_SERVE_PATH)

clean: ## Remove build artifacts and generated reports
	rm -f $(BINARY_AUDIT) $(BINARY_SERVE) $(BINARY_AUDIT)-* $(BINARY_SERVE)-* report.html report.json
	rm -rf examples/test_output
	find examples -name "report.html" -o -name "report.json" -delete

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run go vet on source files
	go vet ./...

# Cross-platform compilation
build-all: build-linux build-mac build-windows ## Build for all supported platforms

build-linux: deps ## Build for Linux (amd64)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUDIT)-linux-amd64 $(CMD_AUDIT_PATH)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVE)-linux-amd64 $(CMD_SERVE_PATH)

build-mac: deps ## Build for Mac (amd64 and arm64)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUDIT)-darwin-amd64 $(CMD_AUDIT_PATH)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUDIT)-darwin-arm64 $(CMD_AUDIT_PATH)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVE)-darwin-amd64 $(CMD_SERVE_PATH)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVE)-darwin-arm64 $(CMD_SERVE_PATH)

build-windows: deps ## Build for Windows (amd64)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_AUDIT)-windows-amd64.exe $(CMD_AUDIT_PATH)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVE)-windows-amd64.exe $(CMD_SERVE_PATH)

test: ## Run unit tests
	go test ./...

.PHONY: all help audit audit-mongodb audit-mongodb-thresholds serve deps build clean build-all build-linux build-mac build-windows test fmt vet docker-build docker-run compose-up compose-down
