[private]
default:
  @just --list

build: (web-build)
  go run build.go

web-build:
  cd web && bunx oxfmt && bun run build

web-dev:
  cd web && bun run dev

run *args:
  CGO_ENABLED=0 go run ./cmd/runic {{args}}

server-dev:
  CGO_ENABLED=0 go run ./cmd/runic serve

clean:
  rm -rf bin/

# Run backend + frontend dev servers concurrently
[parallel]
dev: server-dev web-dev
