#!/bin/bash
# Build fypd for all platforms (darwin-arm64, darwin-x64, linux-x64, win32-x64)
# CGO is required for go-sqlite3. Uses zig for cross-compilation when available.
# Alternative: Run on each OS, or use GitHub Actions (see .github/workflows/build-binaries.yml)

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$SCRIPT_DIR/.."
SERVER_DIR="$ROOT_DIR/server"
BIN_DIR="$SERVER_DIR/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

mkdir -p "$BIN_DIR"

# Platform list: (GOOS, GOARCH, zig-target, output-dir-name, exe-suffix)
# output-dir-name matches Node.js: darwin-arm64, darwin-x64, linux-x64, win32-x64
build_one() {
    local goos=$1
    local goarch=$2
    local zig_target=$3
    local out_name=$4
    local exe_suffix=${5:-}
    local out_dir="$BIN_DIR/$out_name"
    local exe="fypd${exe_suffix}"

    mkdir -p "$out_dir"
    local out_path="$out_dir/$exe"

    echo -e "${BLUE}  Building $out_name...${NC}"

    if command -v zig &> /dev/null; then
        # Use zig for cross-compilation
        CGO_ENABLED=1 GOOS=$goos GOARCH=$goarch \
            CC="zig cc -target $zig_target" \
            go build -o "$out_path" ./cmd/fypd
    else
        # Native build only - must match current platform
        local current_os=$(go env GOOS)
        local current_arch=$(go env GOARCH)
        if [ "$goos" = "$current_os" ] && [ "$goarch" = "$current_arch" ]; then
            CGO_ENABLED=1 go build -o "$out_path" ./cmd/fypd
        else
            echo -e "${YELLOW}    Skipped (zig not installed, cross-compile unavailable)${NC}"
            return 1
        fi
    fi

    if [ -f "$out_path" ]; then
        local size=$(du -h "$out_path" | cut -f1)
        echo -e "${GREEN}    ✓ $out_name ($size)${NC}"
        return 0
    fi
    return 1
}

echo ""
echo -e "${BLUE}Building fypd for all platforms...${NC}"
echo ""

cd "$SERVER_DIR"

SUCCESS=0
TOTAL=4

# darwin-arm64 (Apple Silicon)
if build_one darwin arm64 aarch64-macos darwin-arm64 ""; then
    SUCCESS=$((SUCCESS + 1))
fi

# darwin-x64 (Intel Mac)
if build_one darwin amd64 x86_64-macos darwin-x64 ""; then
    SUCCESS=$((SUCCESS + 1))
fi

# linux-x64
if build_one linux amd64 x86_64-linux-gnu linux-x64 ""; then
    SUCCESS=$((SUCCESS + 1))
fi

# win32-x64
if build_one windows amd64 x86_64-windows-gnu win32-x64 ".exe"; then
    SUCCESS=$((SUCCESS + 1))
fi

echo ""
if [ $SUCCESS -eq 0 ]; then
    echo -e "${RED}No binaries built. Install zig for cross-compilation:${NC}"
    echo "  brew install zig   # macOS"
    echo "  Or run this script on each target OS for native builds."
    exit 1
fi

echo -e "${GREEN}Built $SUCCESS/$TOTAL platforms${NC}"
if [ $SUCCESS -lt $TOTAL ] && ! command -v zig &> /dev/null; then
    echo -e "${YELLOW}Install zig (brew install zig) to build all platforms from one machine.${NC}"
fi
echo ""
