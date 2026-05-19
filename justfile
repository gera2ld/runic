BINARY := "runic"

[private]
default:
  @just --list

# Build the binary
build:
  CGO_ENABLED=0 go build -ldflags="-s -w" -o {{BINARY}} .

# Cross-compile for linux/amd64
linux:
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o {{BINARY}}-linux-amd64 .

# Cross-compile for linux/arm64
linux-arm64:
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o {{BINARY}}-linux-arm64 .

# Cross-compile for darwin/arm64
darwin:
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o {{BINARY}}-darwin-arm64 .

# Run the server directly
dev:
  CGO_ENABLED=0 go run .

# Remove build artifacts
clean:
  rm -f {{BINARY}} {{BINARY}}-*
