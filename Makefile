BINARY_NAME=philterscope
CMD_PATH=./cmd/philterscope

all: build

build:
	go build -o $(BINARY_NAME) $(CMD_PATH)

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe report.html report.json

# Cross-platform compilation
build-all: build-linux build-mac build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 $(CMD_PATH)

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

test:
	go test ./...

.PHONY: all build clean build-all build-linux build-mac build-windows test
