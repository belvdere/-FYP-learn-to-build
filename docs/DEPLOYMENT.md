# Deployment

## Building a Distributable .vsix

To produce a VS Code extension package (.vsix) that can be installed or published:

```bash
# Full package: builds all platform binaries + extension + .vsix
./build-script/deploy.sh --package
```

This runs:

1. **Cross-platform build** (darwin-arm64, darwin-x64, linux-x64, win32-x64) — requires [zig](https://ziglang.org/download/) for cross-compilation: `brew install zig`
2. **Bundle binaries** into `build-with-me/bin/<platform>/`
3. **Build graph-viz** and extension
4. **Create .vsix** via `vsce package`

Without zig, only the current platform is built; the .vsix will work on that platform only.

### Quick Package (Current Platform Only)

If you only need a .vsix for your current machine:

```bash
# 1. Build backend for current platform
cd server && make build

# 2. Copy binary to extension (darwin-arm64 on Apple Silicon)
mkdir -p ../build-with-me/bin/darwin-arm64
cp bin/fypd ../build-with-me/bin/darwin-arm64/

# 3. Package (skips backend build)
./build-script/deploy.sh --skip-backend --package
```

Replace `darwin-arm64` with `darwin-x64`, `linux-x64`, or `win32-x64` as appropriate.

## Testing the .vsix

1. Build the .vsix (see above)
2. Open VS Code
3. Go to Extensions view (Cmd+Shift+X / Ctrl+Shift+X)
4. Click the `...` menu → **Install from VSIX...**
5. Select `build-with-me/build-with-me-0.0.1.vsix`
6. Reload VS Code when prompted
7. Open a workspace with supported source files (`.java`, `.py`, `.ts/.tsx`, `.js/.jsx`) and verify:
   - **FYP: Open Index Panel** works (and Build/Rebuild can run from panel)
   - **FYP: Open Graph Editor** loads the graph
   - **FYP: Configure Copilot MCP** creates mcp.json
   - Masking flow (with Copilot) and validation (CodeLens) work

## Development Build (No Package)

For local development:

```bash
./build-script/deploy.sh
```

Then press F5 in VS Code to launch the Extension Development Host. The extension finds the fypd binary at `../server/bin/fypd` (monorepo layout).
