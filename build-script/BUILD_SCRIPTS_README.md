# Build Scripts Guide

This directory contains convenience scripts for building and deploying the Learn-To-Build knowledge base system.

---

## 🚀 Quick Reference

| Script | Use When | Time | What It Does |
|--------|----------|------|--------------|
| `./deploy.sh` | **First time setup** or after major changes | ~2-3 min | Builds everything (backend + graph-viz + extension) |
| `./deploy.sh --package` | **Ship extension** (distributable .vsix) | ~3-5 min | Cross-platform binaries + .vsix package |
| `./quick-build.sh` | **Regular development** | ~30 sec | Fast build of backend + frontend |
| `./build-backend.sh` | Changed **Go code only** | ~15 sec | Builds Go server only |
| `./build-frontend.sh` | Changed **React/TypeScript only** | ~20 sec | Builds graph-viz + VS Code extension |

---

## 📋 Detailed Usage

### 1. `deploy.sh` - Full Deployment

**When to use**: 
- First time setup
- After pulling major changes
- Clean rebuild needed

**What it does**:
1. ✅ Builds Go backend
2. ✅ Builds graph-viz React application
3. ✅ Bundles graph-viz into VS Code extension
4. ✅ Compiles VS Code extension TypeScript

**Usage**:
```bash
./deploy.sh              # Full build (local development)
./deploy.sh --clean      # Clean build (removes old artifacts first)
./deploy.sh --skip-backend   # Skip backend, build frontend only
./deploy.sh --skip-frontend  # Skip frontend, build backend only
./deploy.sh --package    # Build + create .vsix for distribution
./deploy.sh --all-platforms  # Build binaries for all platforms (Mac/Linux/Windows)
```

**Time**: 2-3 minutes (5+ min with `--package` and cross-platform builds)

**Output**:
```
[1/3] Building Go backend...
✓ Backend built successfully
  Binary: server/bin/fypd (15M)

[2/3] Building graph-viz React app...
✓ graph-viz built successfully
✓ graph-viz bundled into extension

[3/3] Compiling VS Code extension...
✓ VS Code extension compiled successfully

✓ Deployment Complete!
```

---

### 2. `quick-build.sh` - Fast Development Build

**When to use**: 
- Regular development workflow
- Changed either backend or frontend code
- Need fast iteration

**What it does**:
1. ✅ Builds backend (minimal output)
2. ✅ Builds graph-viz React app
3. ✅ Compiles VS Code extension

**Usage**:
```bash
./quick-build.sh              # Build everything
./quick-build.sh -b           # Backend only
./quick-build.sh -f           # Frontend only (graph-viz + extension)
./quick-build.sh --skip-graphviz  # Skip graph-viz rebuild
```

**Time**: 20-30 seconds

**Output**:
```
⚡ Quick Build

[Backend] Building Go server...
✓ Backend built (15M)

[graph-viz] Building React app...
✓ graph-viz built & bundled

[Extension] Compiling TypeScript...
✓ Extension compiled

✓ Quick Build Complete! (25s)
```

---

### 3. `build-backend.sh` - Backend Only

**When to use**: 
- Changed Go code in `server/`
- Testing backend CLI commands
- Fastest iteration for backend development

**What it does**:
1. ✅ Builds Go backend

**Usage**:
```bash
./build-backend.sh           # Standard build
./build-backend.sh --clean   # Clean build
./build-backend.sh --test    # Build and run tests
```

**Time**: 10-15 seconds

**Output**:
```
======================================
  Backend Build
======================================

Building backend...
✓ Backend built successfully
  Binary: server/bin/fypd (15M)

======================================
✓ Backend Build Complete!
======================================
```

**Test immediately**:
```bash
./server/bin/fypd --version
./server/bin/fypd serve-api-stdio --workspace ./test-code
```

---

### 4. `build-frontend.sh` - Frontend Only

**When to use**: 
- Changed React code in `graph-viz/`
- Changed TypeScript code in `build-with-me/`
- Testing VS Code extension UI

**What it does**:
1. ✅ Builds graph-viz React application
2. ✅ Bundles graph-viz into VS Code extension
3. ✅ Compiles VS Code extension TypeScript

**Usage**:
```bash
./build-frontend.sh                # Build everything
./build-frontend.sh --skip-graphviz    # Skip graph-viz, compile extension only
./build-frontend.sh --graphviz-only    # Build graph-viz only
```

**Time**: 15-25 seconds

**Output**:
```
======================================
  Frontend Build
======================================

[1/2] Building graph-viz React app...
✓ graph-viz built successfully
  Copying to VS Code extension...
✓ graph-viz bundled into extension

[2/2] Compiling VS Code extension...
✓ VS Code extension compiled

======================================
✓ Frontend Build Complete!
======================================
```

**Test immediately**:
- Press `F5` in VS Code to launch Extension Development Host
- Or: `Cmd+Shift+P` → "Developer: Reload Window"

---

## 🏠 Local Development

### Initial Setup (Once)
```bash
# Clone repo, install dependencies, build everything
./build-script/deploy.sh
```

### Run Extension in Development
1. Open the repo in VS Code
2. Press **F5** to launch the Extension Development Host
3. The extension uses the binary at `server/bin/fypd` (monorepo layout)

### Regular Development Loop

#### Backend Changes
```bash
# Make changes to server/*.go
./build-script/build-backend.sh

# Test immediately
./server/bin/fypd serve-api-stdio --workspace ./test-code
```

#### graph-viz Changes
```bash
# Make changes to graph-viz/src/*
./build-script/build-frontend.sh

# Press F5 in VS Code to test
```

#### VS Code Extension Changes
```bash
# Make changes to build-with-me/src/*.ts
./build-script/build-frontend.sh --skip-graphviz

# Press F5 in VS Code to test
```

#### Both Changed
```bash
# Make changes to both backend and frontend
./build-script/quick-build.sh

# Test backend
./server/bin/fypd serve-api-stdio --workspace ./test-code

# Press F5 in VS Code to test frontend
```

---

## 📦 Deployment (Ship a .vsix)

### Full Package (All Platforms)

Produces a `.vsix` that can be installed or published:

```bash
./build-script/deploy.sh --package
```

This runs:
1. **Cross-platform build** — darwin-arm64, darwin-x64, linux-x64, win32-x64 (requires [zig](https://ziglang.org/download/) for cross-compilation: `brew install zig`)
2. **Bundle binaries** into `build-with-me/bin/<platform>/`
3. **Build graph-viz** and extension
4. **Create .vsix** — `build-with-me/build-with-me-0.0.1.vsix`

Without zig, only the current platform is built.

### Quick Package (Current Platform Only)

For a .vsix that works only on your machine:

```bash
# 1. Build backend
cd server && make build

# 2. Copy binary (use darwin-arm64, darwin-x64, linux-x64, or win32-x64)
mkdir -p ../build-with-me/bin/darwin-arm64
cp bin/fypd ../build-with-me/bin/darwin-arm64/

# 3. Package
./build-script/deploy.sh --skip-backend --package
```

### Test the .vsix

1. VS Code → Extensions view → `...` menu → **Install from VSIX...**
2. Select `build-with-me/build-with-me-0.0.1.vsix`
3. Reload VS Code and verify: indexing, graph editor, MCP config, masking, validation

---

## 🛠️ Manual Build Commands

If you prefer manual control:

### Backend
```bash
cd server

# Build
make build

# Run tests
make test

# Clean build
make clean
```

### graph-viz
```bash
cd graph-viz

# Install dependencies (first time)
npm install

# Build production bundle
npm run build

# Development mode with hot reload
npm run dev
```

### VS Code Extension
```bash
cd build-with-me

# Install dependencies (first time)
pnpm install

# Compile TypeScript (includes graph-viz rebuild)
pnpm compile

# Watch mode (auto-recompile on changes)
pnpm watch

# Run tests
pnpm test
```

---

## 📊 Build Time Comparison

| Scenario | Script | Time |
|----------|--------|------|
| First time / full rebuild | `./deploy.sh` | 2-3 min |
| Package .vsix (all platforms) | `./deploy.sh --package` | 3-5 min |
| Full quick rebuild | `./quick-build.sh` | 30 sec |
| Backend only | `./build-backend.sh` | 15 sec |
| Frontend only | `./build-frontend.sh` | 20 sec |
| Extension only | `./build-frontend.sh --skip-graphviz` | 10 sec |
| Watch mode | `cd build-with-me && pnpm watch` | Instant |

---

## 🐛 Troubleshooting

### Error: "pnpm not found"
```bash
npm install -g pnpm
```

### Error: "make: command not found"
```bash
# macOS
xcode-select --install

# Linux
sudo apt-get install build-essential
```

### Backend build fails
```bash
# Check Go installation
go version

# Clean build
./build-script/build-backend.sh --clean
```

### Frontend compile fails
```bash
# Reinstall dependencies
cd graph-viz && rm -rf node_modules && npm install
cd ../build-with-me && rm -rf node_modules && pnpm install

# Try building again
./build-script/build-frontend.sh
```

### graph-viz not loading in VS Code
```bash
# Ensure graph-viz is bundled
ls -la build-with-me/media/graph-viz/

# If empty, rebuild frontend
./build-script/build-frontend.sh
```

---

## 📁 Project Structure

```
learn-to-build/
├── build-script/
│   ├── deploy.sh              ← Full deployment / package .vsix
│   ├── build-binaries.sh      ← Cross-platform fypd builds
│   ├── quick-build.sh         ← Fast rebuild
│   ├── build-backend.sh       ← Backend only
│   ├── build-frontend.sh      ← Frontend only
│   └── BUILD_SCRIPTS_README.md
│
├── server/                    ← Backend (Go)
│   ├── cmd/fypd/              ← CLI entry point
│   ├── pkg/                   ← Core logic
│   ├── bin/fypd               ← Built binary
│   └── Makefile               ← Build targets
│
├── graph-viz/                 ← Graph Visualization (React)
│   ├── src/                   ← React components
│   ├── dist/                  ← Built bundle
│   └── package.json
│
└── build-with-me/             ← VS Code Extension (TypeScript)
    ├── src/                   ← Extension source
    ├── media/graph-viz/       ← Bundled graph-viz
    ├── out/                   ← Compiled JS
    └── package.json
```

---

## 🎯 Pro Tips

### 1. Use Watch Mode for Extension Development
```bash
cd build-with-me
pnpm watch
# Now changes auto-compile! Just reload VS Code (Cmd+Shift+P -> Reload Window)
```

### 2. graph-viz Development Server
```bash
cd graph-viz
npm run dev
# Opens at http://localhost:5173 with hot reload
# Note: Works standalone, but masking features require VS Code
```

### 3. Parallel Builds
```bash
# Build backend and frontend simultaneously
./build-script/build-backend.sh & ./build-script/build-frontend.sh & wait
```

### 4. Check What Changed
```bash
# See what needs rebuilding
git status

# Backend changes?
git diff server/

# graph-viz changes?
git diff graph-viz/

# Extension changes?
git diff build-with-me/
```

---

## ✅ Summary

- **First time / local dev?** Run `./build-script/deploy.sh`, then press F5 in VS Code
- **Regular dev?** Use `./build-script/quick-build.sh`
- **Backend only?** Use `./build-script/build-backend.sh`
- **Frontend only?** Use `./build-script/build-frontend.sh`
- **Active dev?** Use watch mode: `cd build-with-me && pnpm watch`
- **Ship extension?** Run `./build-script/deploy.sh --package` for a distributable .vsix

**All scripts are safe to run multiple times!** 🚀

For deployment details, see [DEPLOYMENT.md](../DEPLOYMENT.md) at the project root.
