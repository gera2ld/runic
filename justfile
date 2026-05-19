[private]
default:
  @just --list

build:
  go run build.go

dev *args:
  CGO_ENABLED=0 go run ./cmd/runic {{args}}

clean:
  rm -rf bin/
