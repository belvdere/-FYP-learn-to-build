#!/bin/bash
# Deploy Script - Full Build of Backend and Frontend
# Builds everything: Go server, graph-viz React app, and VS Code extension

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$SCRIPT_DIR/.."

# Parse arguments
SKIP_BACKEND=false
SKIP_FRONTEND=false
CLEAN_BUILD=false
BUILD_ALL_PLATFORMS=false
PACKAGE_VSIX=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --skip-backend) SKIP_BACKEND=true ;;
        --skip-frontend) SKIP_FRONTEND=true ;;
        --clean) CLEAN_BUILD=true ;;
        --all-platforms) BUILD_ALL_PLATFORMS=true ;;
        --package) PACKAGE_VSIX=true ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --skip-backend     Skip building the Go backend"
            echo "  --skip-frontend    Skip building graph-viz and VS Code extension"
            echo "  --clean            Clean build (remove build artifacts first)"
            echo "  --all-platforms    Build fypd for all platforms (darwin, linux, win32)"
            echo "  --package          Produce .vsix for distribution (implies --all-platforms)"
            echo "  -h, --help         Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
    shift
done

if [ "$PACKAGE_VSIX" = true ]; then
    BUILD_ALL_PLATFORMS=true
fi

# Calculate total steps
TOTAL_STEPS=3
[ "$PACKAGE_VSIX" = true ] && TOTAL_STEPS=$((TOTAL_STEPS + 1))
CURRENT_STEP=0
if [ "$SKIP_BACKEND" = true ]; then
    TOTAL_STEPS=$((TOTAL_STEPS - 1))
fi
if [ "$SKIP_FRONTEND" = true ]; then
    TOTAL_STEPS=$((TOTAL_STEPS - 2))
fi

echo ""
echo "========================================"
echo -e "${CYAN}  Learn-To-Build Full Deployment${NC}"
echo "========================================"
echo ""

# Clean if requested
if [ "$CLEAN_BUILD" = true ]; then
    echo -e "${YELLOW}Cleaning previous builds...${NC}"
    rm -rf "$ROOT_DIR/server/bin" 2>/dev/null || true
    rm -rf "$ROOT_DIR/build-with-me/bin" 2>/dev/null || true
    rm -rf "$ROOT_DIR/graph-viz/dist" 2>/dev/null || true
    rm -rf "$ROOT_DIR/build-with-me/out" 2>/dev/null || true
    rm -rf "$ROOT_DIR/build-with-me/media/graph-viz" 2>/dev/null || true
    echo -e "${GREEN}✓ Cleaned${NC}"
    echo ""
fi

# Step 1: Backend
if [ "$SKIP_BACKEND" = false ]; then
    CURRENT_STEP=$((CURRENT_STEP + 1))
    if [ "$BUILD_ALL_PLATFORMS" = true ]; then
        echo -e "${BLUE}[${CURRENT_STEP}/${TOTAL_STEPS}] Building Go backend (all platforms)...${NC}"
        cd "$ROOT_DIR/server"
        if make build-all; then
            echo -e "${GREEN}✓ Backend built for all platforms${NC}"
        else
            echo -e "${RED}✗ Backend build failed${NC}"
            exit 1
        fi
    else
        echo -e "${BLUE}[${CURRENT_STEP}/${TOTAL_STEPS}] Building Go backend...${NC}"
        cd "$ROOT_DIR/server"
        if make build; then
            echo -e "${GREEN}✓ Backend built successfully${NC}"
            if [ -f "bin/fypd" ]; then
                BINARY_SIZE=$(du -h bin/fypd | cut -f1)
                echo -e "${GREEN}  Binary: server/bin/fypd (${BINARY_SIZE})${NC}"
            fi
        else
            echo -e "${RED}✗ Backend build failed${NC}"
            exit 1
        fi
    fi
    echo ""
fi

# Step 1b: Bundle binaries into extension (when building all platforms)
if [ "$SKIP_BACKEND" = false ] && [ "$BUILD_ALL_PLATFORMS" = true ]; then
    echo -e "${BLUE}  Bundling fypd binaries into extension...${NC}"
    EXT_BIN="$ROOT_DIR/build-with-me/bin"
    mkdir -p "$EXT_BIN"
    for platform in darwin-arm64 darwin-x64 linux-x64 win32-x64; do
        SRC="$ROOT_DIR/server/bin/$platform"
        DST="$EXT_BIN/$platform"
        if [ -d "$SRC" ]; then
            mkdir -p "$DST"
            cp -r "$SRC"/* "$DST/" 2>/dev/null || true
            if [ -n "$(ls -A "$DST" 2>/dev/null)" ]; then
                echo -e "${GREEN}    ✓ $platform${NC}"
            fi
        fi
    done
    echo -e "${GREEN}✓ Binaries bundled${NC}"
    echo ""
fi

# Step 2: graph-viz React app
if [ "$SKIP_FRONTEND" = false ]; then
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${BLUE}[${CURRENT_STEP}/${TOTAL_STEPS}] Building graph-viz React app...${NC}"
    cd "$ROOT_DIR/graph-viz"

    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}  Installing graph-viz dependencies...${NC}"
        npm install
    fi

    if npm run build; then
        echo -e "${GREEN}✓ graph-viz built successfully${NC}"
        
        # Copy to VS Code extension media folder
        echo -e "${BLUE}  Bundling into VS Code extension...${NC}"
        mkdir -p "$ROOT_DIR/build-with-me/media/graph-viz"
        cp -r dist/* "$ROOT_DIR/build-with-me/media/graph-viz/"
        echo -e "${GREEN}✓ graph-viz bundled into extension${NC}"
    else
        echo -e "${RED}✗ graph-viz build failed${NC}"
        exit 1
    fi
    echo ""

    # Step 3: VS Code extension
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo -e "${BLUE}[${CURRENT_STEP}/${TOTAL_STEPS}] Compiling VS Code extension...${NC}"
    cd "$ROOT_DIR/build-with-me"

    # Bundle prompt-optimizer (optional TextGrad scripts for FYP_OPTIMIZE_PROMPTS)
    if [ -d "$ROOT_DIR/prompt-optimizer" ]; then
        mkdir -p "$ROOT_DIR/build-with-me/prompt-optimizer"
        cp -r "$ROOT_DIR/prompt-optimizer/"* "$ROOT_DIR/build-with-me/prompt-optimizer/" 2>/dev/null || true
    fi

    # Try pnpm first, fall back to npm
    if command -v pnpm &> /dev/null; then
        if [ ! -d "node_modules" ]; then
            echo -e "${YELLOW}  Installing extension dependencies...${NC}"
            pnpm install
        fi
        
        # Use production build when packaging, otherwise dev build
        if [ "$PACKAGE_VSIX" = true ]; then
            BUILD_CMD="pnpm run package"
        else
            BUILD_CMD="pnpm run check-types && pnpm run lint && node esbuild.js"
        fi
        if eval "$BUILD_CMD"; then
            echo -e "${GREEN}✓ VS Code extension compiled successfully${NC}"
        else
            echo -e "${RED}✗ VS Code extension compilation failed${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}  pnpm not found, using npm...${NC}"
        if [ ! -d "node_modules" ]; then
            npm install
        fi
        [ "$PACKAGE_VSIX" = true ] && npm run package || npm run compile
    fi
    echo ""

    # Step 4: Package .vsix (when --package)
    if [ "$PACKAGE_VSIX" = true ]; then
        CURRENT_STEP=$((CURRENT_STEP + 1))
        echo -e "${BLUE}[${CURRENT_STEP}/${TOTAL_STEPS}] Packaging .vsix...${NC}"
        cd "$ROOT_DIR/build-with-me"
        if (pnpm run package:vsix 2>/dev/null) || (npx vsce package --no-dependencies 2>/dev/null); then
            VSIX=$(ls -t *.vsix 2>/dev/null | head -1)
            if [ -n "$VSIX" ]; then
                echo -e "${GREEN}✓ Created $VSIX${NC}"
                echo -e "${CYAN}  Install: Extensions → ⋮ → Install from VSIX${NC}"
            else
                echo -e "${GREEN}✓ .vsix package created${NC}"
            fi
        else
            echo -e "${RED}✗ vsce package failed${NC}"
            exit 1
        fi
        echo ""
    fi
fi

# Summary
echo "========================================"
echo -e "${GREEN}✓ Deployment Complete!${NC}"
echo "========================================"
echo ""
echo -e "${CYAN}What was built:${NC}"
if [ "$SKIP_BACKEND" = false ]; then
    if [ "$BUILD_ALL_PLATFORMS" = true ]; then
        echo "  ✓ Go backend (server/bin/<platform>/)"
        echo "  ✓ Binaries bundled into extension (build-with-me/bin/)"
    else
        echo "  ✓ Go backend server (server/bin/fypd)"
    fi
fi
if [ "$SKIP_FRONTEND" = false ]; then
    echo "  ✓ graph-viz React app (bundled into extension)"
    echo "  ✓ VS Code extension (build-with-me/dist/)"
    [ "$PACKAGE_VSIX" = true ] && echo "  ✓ .vsix package (build-with-me/*.vsix)"
fi
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo ""
if [ "$PACKAGE_VSIX" = true ]; then
    VSIX_FILE=$(ls -t "$ROOT_DIR/build-with-me"/*.vsix 2>/dev/null | head -1)
    if [ -n "$VSIX_FILE" ]; then
        echo "1. Install the extension:"
        echo "   - Open VS Code: Extensions view (Cmd+Shift+X)"
        echo "   - Click '...' → 'Install from VSIX...'"
        echo "   - Select: $VSIX_FILE"
        echo ""
        echo "2. Or distribute the .vsix for others to install."
        echo ""
    fi
else
    echo "1. Launch VS Code extension:"
    echo "   - Press F5 in VS Code to launch Extension Development Host"
    echo "   - Or: Cmd+Shift+P -> 'Developer: Reload Window'"
    echo ""
    echo "2. Open Graph Editor:"
    echo "   - In VS Code: Cmd+Shift+P -> 'FYP: Open Graph Editor'"
    echo ""
    echo "3. For .vsix package: run ./build-script/deploy.sh --package"
    echo ""
fi

