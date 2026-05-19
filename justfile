[private]
default:
  @just --list

build:
  go run build.go

dev:
  CGO_ENABLED=0 go run ./cmd/runic

clean:
  rm -rf bin/
