# Makefile for fypd

.PHONY: build build-all clean test ast-dump

# Build the fypd binary for current platform (CGO required for SQLite)
build:
	CGO_ENABLED=1 go build -o bin/fypd ./cmd/fypd

# Build AST visualization tool (no CGO needed)
ast-dump:
	go build -o bin/ast-dump ./cmd/ast-dump

# Build for all platforms (darwin-arm64, darwin-x64, linux-x64, win32-x64)
# Requires zig for cross-compilation: brew install zig
build-all:
	@bash ../build-script/build-binaries.sh

# Clean build artifacts
clean:
	rm -f bin/fypd bin/ast-dump
	rm -rf bin/darwin-arm64 bin/darwin-x64 bin/linux-x64 bin/win32-x64

# Run tests
test:
	CGO_ENABLED=1 go test ./...

# Default target
all: build
