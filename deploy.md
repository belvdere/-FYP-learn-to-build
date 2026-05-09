# Deploy

## Build .vsix

```bash
./build-script/deploy.sh --package
```

Produces `build-with-me/build-with-me-0.0.1.vsix`. For all platforms (Mac/Linux/Windows), install [zig](https://ziglang.org/download/) first: `brew install zig`. Without zig, only the current platform is built.

## Install

1. VS Code → Extensions → `...` → **Install from VSIX...**
2. Select `build-with-me/build-with-me-0.0.1.vsix`
3. Reload VS Code

## Troubleshooting

**"Unable to read package.json" / "undefined_publisher"**

Uninstall any existing "build-with-me" or "Build With Me" extensions, then reinstall from VSIX. Conflicting or corrupted installs can cause this.

**"ENOENT: mcp.json"**

The extension now creates `.vscode/mcp.json` on first activation. If you still see this, run **FYP: Configure Copilot MCP** once, then reload the window.

## TextGrad (Optional)

To enable prompt optimization in `kg.getSnapshot`:

1. Install: `pip install textgrad`
2. Create `{workspace}/.fyp/.env`:

   ```
   FYP_OPTIMIZE_PROMPTS=1
   OPENAI_API_KEY=sk-...
   ```

See [docs/TEXTGRAD_INTEGRATION.md](docs/TEXTGRAD_INTEGRATION.md) for details.
