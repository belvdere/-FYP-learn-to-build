# TextGrad Integration

Optional prompt optimization via [TextGrad](https://textgrad.com/) at the MCP layer.
When enabled, snapshot context returned by `kg.getSnapshot` is refined before being sent to Copilot.

## Requirements

- Python 3.9+
- `pip install textgrad` (or `pip install -r prompt-optimizer/requirements.txt`)
- **OPENAI_API_KEY** (TextGrad uses it for the backward/feedback pass)

## API Key and Configuration

Both `OPENAI_API_KEY` and `FYP_OPTIMIZE_PROMPTS` can be set via:

1. **Environment variables**: `export OPENAI_API_KEY=sk-...` and `export FYP_OPTIMIZE_PROMPTS=1`
2. **`.fyp/.env` file**: Create `{workspace}/.fyp/.env` with:
   ```
   OPENAI_API_KEY=sk-...
   FYP_OPTIMIZE_PROMPTS=1
   ```
   The MCP server loads this at startup. The `.fyp` folder is gitignored, so your keys stay local.

For standalone scripts (`optimize_tool_descriptions.py`, `optimize_copilot_instructions.py`), they also load from `.fyp/.env` in the current directory or any parent.

## Enabling Per-Call Optimization

1. Ensure `prompt-optimizer` is available (it lives at repo root for local dev; bundled in the extension for packaged installs).
2. Set `FYP_OPTIMIZE_PROMPTS=1` and provide the API key. Easiest: create `{workspace}/.fyp/.env` with:
   ```
   FYP_OPTIMIZE_PROMPTS=1
   OPENAI_API_KEY=sk-...
   ```
   Or use environment variables.
3. When Copilot calls `kg.getSnapshot`, the MCP server will run the optimizer on the snapshot markdown and return the refined content. If the optimizer fails (missing Python, timeout, etc.), it falls back to the original content and logs a warning.

### Configuration

| Env | Description | Default |
|-----|-------------|---------|
| `FYP_OPTIMIZE_PROMPTS` | Set to `1` to enable | Off |
| `OPENAI_API_KEY` | Required for TextGrad | - |
| `FYP_PROMPT_OPTIMIZER_PATH` | Path to `optimize.py` | Auto-resolved |
| `FYP_OPTIMIZE_TIMEOUT` | Subprocess timeout (seconds) | 30 |

### Path Resolution

The MCP server looks for `optimize.py` in this order:

1. `FYP_PROMPT_OPTIMIZER_PATH` (if set)
2. `{workspace}/prompt-optimizer/optimize.py` (monorepo dev)
3. `{workspace}/.fyp/prompt-optimizer/optimize.py`
4. Relative to `fypd` executable (packaged extension)

## References

- [TextGrad paper](https://arxiv.org/abs/2406.07496)
- [TextGrad GitHub](https://github.com/zou-group/textgrad)
- [MCP.md](MCP.md) – MCP architecture and tools
