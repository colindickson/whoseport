.PHONY: build install clean test

# Build binary for local testing
build:
	go build -o whoseport ./cmd/whoseport

# Install binary to $GOBIN (or $GOPATH/bin or $HOME/go/bin)
install:
	go install ./cmd/whoseport

# Remove locally built binary
clean:
	rm -f whoseport

# Run tests
test:
	go vet ./...
	go test ./...
