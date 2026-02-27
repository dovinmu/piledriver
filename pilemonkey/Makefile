.PHONY: build install clean test run

BINARY := pilemonkey
BINDIR := bin
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w"

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BINDIR)
	go build $(LDFLAGS) -o $(BINDIR)/$(BINARY) .

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) .

# Install to /usr/local/bin (requires sudo)
install-global: build
	sudo cp $(BINDIR)/$(BINARY) /usr/local/bin/

# Clean build artifacts
clean:
	rm -rf $(BINDIR)
	go clean

# Run tests
test:
	go test -v ./...

# Run the application (watches current directory)
run: build
	./$(BINDIR)/$(BINARY)

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
release:
	@mkdir -p $(BINDIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINDIR)/$(BINARY)-darwin-arm64 .
