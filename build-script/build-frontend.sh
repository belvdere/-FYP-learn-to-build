#!/bin/bash
# Frontend Build Script
# Builds graph-viz React app and VS Code extension
# Use this when you changed TypeScript/React code

set -e  # Exit on error

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$SCRIPT_DIR/.."

# Parse arguments
SKIP_GRAPHVIZ=false
GRAPHVIZ_ONLY=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --skip-graphviz) SKIP_GRAPHVIZ=true ;;
        --graphviz-only) GRAPHVIZ_ONLY=true ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --skip-graphviz   Skip building graph-viz, only compile VS Code extension"
            echo "  --graphviz-only   Only build graph-viz React app"
            echo "  -h, --help        Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
    shift
done

echo ""
echo "========================================"
echo "  Frontend Build"
echo "========================================"
echo ""

# Step 1: Build graph-viz React app
if [ "$SKIP_GRAPHVIZ" = false ]; then
    echo -e "${BLUE}[1/2] Building graph-viz React app...${NC}"
    cd "$ROOT_DIR/graph-viz"

    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}  Installing graph-viz dependencies...${NC}"
        npm install
    fi

    if npm run build; then
        echo -e "${GREEN}✓ graph-viz built successfully${NC}"
        
        # Copy to VS Code extension media folder
        echo -e "${BLUE}  Copying to VS Code extension...${NC}"
        mkdir -p "$ROOT_DIR/build-with-me/media/graph-viz"
        cp -r dist/* "$ROOT_DIR/build-with-me/media/graph-viz/"
        echo -e "${GREEN}✓ graph-viz bundled into extension${NC}"
    else
        echo -e "${RED}✗ graph-viz build failed${NC}"
        exit 1
    fi
    echo ""
fi

# Step 2: Compile VS Code extension
if [ "$GRAPHVIZ_ONLY" = false ]; then
    STEP_NUM="2/2"
    if [ "$SKIP_GRAPHVIZ" = true ]; then
        STEP_NUM="1/1"
    fi
    
    echo -e "${BLUE}[${STEP_NUM}] Compiling VS Code extension...${NC}"
    cd "$ROOT_DIR/build-with-me"

    # Check if pnpm is installed
    if ! command -v pnpm &> /dev/null; then
        echo -e "${YELLOW}  pnpm not found, using npm instead...${NC}"
        
        if [ ! -d "node_modules" ]; then
            npm install
        fi
        npm run compile
    else
        if [ ! -d "node_modules" ]; then
            echo -e "${YELLOW}  Installing extension dependencies...${NC}"
            pnpm install
        fi
        pnpm compile
    fi
    
    echo -e "${GREEN}✓ VS Code extension compiled${NC}"
fi

echo ""
echo "========================================"
echo -e "${GREEN}✓ Frontend Build Complete!${NC}"
echo "========================================"
echo ""
echo "Next steps:"
echo "  Press F5 in VS Code to launch Extension Development Host"
echo "  Or: Cmd+Shift+P -> 'Developer: Reload Window'"
echo ""

