BINARY_NAME=philterscope
CMD_PATH=./cmd/philterscope
SOURCE_FILES=$(shell find . -name '*.go')
HTML_FILES=$(shell find . -name '*.html')

all: build

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

audit: build ## Run audit on example files
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75
	xdg-open ./examples/report.html

audit-mongodb-ai: build ## Run audit on example files with MongoDB storage and AI policy suggestions
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope PHILTERSCOPE_OLLAMA_URL=http://localhost:11434 ./philterscope audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --ai

audit-mongodb: build ## Run audit on example files with MongoDB storage
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope audit --golden ./examples/golden/ --input ./examples/raw/ --output ./examples/ --threshold 0.75 --ai

serve: build ## Launch Evaluation UI using MongoDB storage
	PHILTERSCOPE_MONGODB_CONNECTION_STRING=mongodb://localhost:27017/philterscope ./philterscope serve

deps: ## Download and tidy Go dependencies
	go mod download
	go mod tidy

build: deps $(SOURCE_FILES) $(HTML_FILES) ## Build the philterscope binary
	go build -o $(BINARY_NAME) $(CMD_PATH)

clean: ## Remove build artifacts and generated reports
	rm -f $(BINARY_NAME) $(BINARY_NAME)-* report.html report.json
	rm -rf examples/test_output
	find examples -name "report.html" -o -name "report.json" -delete

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run go vet on source files
	go vet ./...

# Cross-platform compilation
build-all: build-linux build-mac build-windows ## Build for all supported platforms

build-linux: deps ## Build for Linux (amd64)
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 $(CMD_PATH)

build-mac: deps ## Build for Mac (amd64 and arm64)
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

build-windows: deps ## Build for Windows (amd64)
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

test: ## Run unit tests
	go test ./...

.PHONY: all help audit audit-mongodb serve deps build clean build-all build-linux build-mac build-windows test fmt vet
