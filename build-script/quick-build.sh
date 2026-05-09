#!/bin/bash
# Quick Build Script - Fast Development Build
# Builds backend + frontend without full dependency checks
# Use this for rapid iteration during development

set -e  # Exit on error

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$SCRIPT_DIR/.."

# Parse arguments
BACKEND_ONLY=false
FRONTEND_ONLY=false
SKIP_GRAPHVIZ=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --backend|-b) BACKEND_ONLY=true ;;
        --frontend|-f) FRONTEND_ONLY=true ;;
        --skip-graphviz) SKIP_GRAPHVIZ=true ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -b, --backend      Build backend only"
            echo "  -f, --frontend     Build frontend only (graph-viz + extension)"
            echo "  --skip-graphviz    Skip graph-viz, only compile extension"
            echo "  -h, --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                  Build everything"
            echo "  $0 -b               Build backend only"
            echo "  $0 -f               Build frontend only"
            echo "  $0 --skip-graphviz  Build backend + extension (skip graph-viz)"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
    shift
done

echo ""
echo -e "${CYAN}⚡ Quick Build${NC}"
echo ""

START_TIME=$(date +%s)

# Build backend
if [ "$FRONTEND_ONLY" = false ]; then
    echo -e "${BLUE}[Backend]${NC} Building Go server..."
    cd "$ROOT_DIR/server"
    
    if make build 2>&1 | grep -E "(error|Error|ERROR)" ; then
        echo -e "${RED}✗ Backend build failed${NC}"
        exit 1
    fi
    
    BINARY_SIZE=$(du -h bin/fypd 2>/dev/null | cut -f1 || echo "?")
    echo -e "${GREEN}✓ Backend built (${BINARY_SIZE})${NC}"
fi

# Build graph-viz
if [ "$BACKEND_ONLY" = false ] && [ "$SKIP_GRAPHVIZ" = false ]; then
    echo -e "${BLUE}[graph-viz]${NC} Building React app..."
    cd "$ROOT_DIR/graph-viz"
    
    if npm run build --silent 2>&1 | grep -E "(error|Error|ERROR)" ; then
        echo -e "${RED}✗ graph-viz build failed${NC}"
        exit 1
    fi
    
    # Copy to extension
    mkdir -p "$ROOT_DIR/build-with-me/media/graph-viz"
    cp -r dist/* "$ROOT_DIR/build-with-me/media/graph-viz/"
    echo -e "${GREEN}✓ graph-viz built & bundled${NC}"
fi

# Build VS Code extension
if [ "$BACKEND_ONLY" = false ]; then
    echo -e "${BLUE}[Extension]${NC} Compiling TypeScript..."
    cd "$ROOT_DIR/build-with-me"
    
    if command -v pnpm &> /dev/null; then
        pnpm run check-types 2>&1 | grep -E "(error|Error)" || true
        node esbuild.js 2>&1 | grep -v "build started\|build finished" || true
    else
        npm run compile 2>&1 | grep -E "(error|Error)" || true
    fi
    
    echo -e "${GREEN}✓ Extension compiled${NC}"
fi

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo -e "${GREEN}✓ Quick Build Complete!${NC} (${ELAPSED}s)"
echo ""

if [ "$BACKEND_ONLY" = false ]; then
    echo "Press F5 in VS Code to reload extension"
fi
