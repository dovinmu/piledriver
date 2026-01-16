.PHONY: build install clean test run

BINARY := pilemonkey
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w"

# Default target
all: build

# Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) .

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) .

# Install to /usr/local/bin (requires sudo)
install-global: build
	sudo cp $(BINARY) /usr/local/bin/

# Clean build artifacts
clean:
	rm -f $(BINARY)
	go clean

# Run tests
test:
	go test -v ./...

# Run the application (watches current directory)
run: build
	./$(BINARY)

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .
