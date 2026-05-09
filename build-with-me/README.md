# Build With Me

A VS Code extension that adds structure to AI-assisted coding with call graph exploration, graph snapshots, and masking/validation workflows.

## Getting Started

1. Run `FYP: Configure Copilot MCP`.
2. Run `FYP: Open Index Panel` and click **Build Index** (or **Rebuild Index**).
3. Run `FYP: Open Graph Editor`.

Indexing uses language-server call hierarchy resolution (Java/JDTLS, Python/Pylance, TypeScript+JavaScript/tsserver, Go/gopls).

## Commands

| Command | Description |
|---------|-------------|
| `FYP: Open Index Panel` | Open indexing panel and run Build/Rebuild indexing |
| `FYP: Open Graph Editor` | Open graph visualization panel |
| `FYP: Configure Copilot MCP` | Configure MCP integration and write agent reference file |
| `FYP: Validate Masks` | Validate filled masks in the active file |

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `fyp.autoStartMCP` | `true` | Show MCP configuration status |
| `fyp.autoStartBackend` | `true` | Start API stdio server on activation |
| `fyp.indexOnSave` | `true` | Incrementally update call graph when supported files are saved |

The backend communicates over stdio. `fyp.backendPort` is currently unused.

## Masking

Add `--mask` to any prompt:

```text
Create a UserService class with CRUD methods --mask
```

The AI writes masked code with `[MASK]` placeholders. Fill them in, then validate via CodeLens or `FYP: Validate Masks`.

## Using MCP Tools with Other AI Agents

When the extension activates it writes an MCP tools reference to your workspace:

```
.fyp/build-with-me-mcp.md
```

This file documents all three MCP tools (`kg.getSnapshot`, `kg.maskCode`, `kg.validateFilledCode`). Point your agent's auto-read file at it so it knows the MCP tools this extension provides. 

### GitHub Copilot

Add to your workspaces `.github/copilot-instructions.md` 

### Claude Code

Add to your workspace `CLAUDE.md`:

```md
@.fyp/build-with-me-mcp.md
```

Claude Code inlines the file on every session.

### OpenAI Codex

Add to your workspace `AGENTS.md`:

```md
## Build-With-Me MCP Tools

See `.fyp/build-with-me-mcp.md` for the full reference, or paste its contents here.
```

Codex and Copilot does not support file includes, so paste the contents of `.fyp/build-with-me-mcp.md` directly.

## Troubleshooting

| Issue | Fix |
|-------|-----|
| `fypd not found` | Rebuild/package extension binaries |
| `MCP not configured` | Run `FYP: Configure Copilot MCP` |
| Index button disabled | Wait for language server readiness in Index Panel |
| Indexing fails | Ensure required language servers are installed and ready (check Index Panel status) |

## Privacy

- Local-first: no cloud indexing pipeline.
- Workspace-scoped data in `.fyp/index.db`.
