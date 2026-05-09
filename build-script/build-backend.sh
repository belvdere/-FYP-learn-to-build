#!/bin/bash
# Backend Build Script
# Builds the Go server
# Use this when you only changed Go code

set -e  # Exit on error

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$SCRIPT_DIR/.."

# Parse arguments
CLEAN_BUILD=false
RUN_TESTS=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --clean) CLEAN_BUILD=true ;;
        --test) RUN_TESTS=true ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --clean    Clean build (remove bin/ first)"
            echo "  --test     Run tests after building"
            echo "  -h, --help Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
    shift
done

echo ""
echo "========================================"
echo "  Backend Build"
echo "========================================"
echo ""

cd "$ROOT_DIR/server"

# Clean if requested
if [ "$CLEAN_BUILD" = true ]; then
    echo -e "${YELLOW}Cleaning previous build...${NC}"
    make clean 2>/dev/null || rm -rf bin/
    echo -e "${GREEN}✓ Cleaned${NC}"
fi

# Build
echo -e "${BLUE}Building backend...${NC}"

if make build; then
    if [ -f "bin/fypd" ]; then
        BINARY_SIZE=$(du -h bin/fypd | cut -f1)
        echo -e "${GREEN}✓ Backend built successfully${NC}"
        echo -e "${GREEN}  Binary: server/bin/fypd (${BINARY_SIZE})${NC}"
    fi
else
    echo -e "${RED}✗ Backend build failed${NC}"
    exit 1
fi

# Run tests if requested
if [ "$RUN_TESTS" = true ]; then
    echo ""
    echo -e "${BLUE}Running tests...${NC}"
    if make test; then
        echo -e "${GREEN}✓ All tests passed${NC}"
    else
        echo -e "${RED}✗ Some tests failed${NC}"
        exit 1
    fi
fi

echo ""
echo "========================================"
echo -e "${GREEN}✓ Backend Build Complete!${NC}"
echo "========================================"
echo ""
echo "Test the binary:"
echo "  ./server/bin/fypd --version"
echo "  ./server/bin/fypd serve-api-stdio --workspace /path/to/code"
echo ""

