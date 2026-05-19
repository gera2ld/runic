BINARY := "runic"

[private]
default:
  @just --list

build:
  CGO_ENABLED=0 go build -ldflags="-s -w" -o {{BINARY}} .

linux:
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o {{BINARY}}-linux-amd64 .

linux-arm64:
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o {{BINARY}}-linux-arm64 .

darwin:
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o {{BINARY}}-darwin-arm64 .

dev:
  CGO_ENABLED=0 go run .

clean:
  rm -f {{BINARY}} {{BINARY}}-*
