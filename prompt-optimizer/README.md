# Prompt Optimizer (TextGrad)

Optional prompt optimization via [TextGrad](https://textgrad.com/) for the FYP MCP layer.
Requires Python 3.9+ and an OpenAI API key.

## Setup

```bash
pip install -r requirements.txt
```

Provide `OPENAI_API_KEY` via environment variable or `.fyp/.env`:
```bash
export OPENAI_API_KEY=sk-...
# Or create .fyp/.env with: OPENAI_API_KEY=sk-...
```

## Scripts

### optimize.py

Optimizes snapshot markdown on each `kg.getSnapshot` call (when invoked by the MCP server).

**Standalone usage:**
```bash
echo "Your snapshot markdown here" | python optimize.py
```
