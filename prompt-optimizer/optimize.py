#!/usr/bin/env python3
"""
Optimize snapshot markdown content via TextGrad.
Reads from stdin, outputs optimized content to stdout.
Used by the MCP server when FYP_OPTIMIZE_PROMPTS=1.
"""
import sys
import argparse
from pathlib import Path


def _load_env():
    """Load .env from .fyp/.env or .env (for standalone runs)."""
    try:
        from dotenv import load_dotenv
    except ImportError:
        return
    # Try .fyp/.env in cwd and parent dirs
    for base in [Path.cwd()] + list(Path.cwd().parents):
        env_file = base / ".fyp" / ".env"
        if env_file.exists():
            load_dotenv(env_file)
            return
    load_dotenv()


def main():
    _load_env()
    parser = argparse.ArgumentParser(description="Optimize snapshot markdown via TextGrad")
    parser.add_argument("--steps", type=int, default=1, help="Number of TGD steps (default: 1)")
    args = parser.parse_args()

    content = sys.stdin.read()
    if not content.strip():
        sys.stderr.write("optimize.py: empty input\n")
        sys.exit(1)

    try:
        import textgrad as tg
    except ImportError:
        sys.stderr.write("optimize.py: textgrad not installed. Run: pip install textgrad\n")
        sys.exit(1)

    # Requires OPENAI_API_KEY; TextGrad will raise if missing
    tg.set_backward_engine("gpt-4o", override=True)

    markdown = tg.Variable(
        content,
        requires_grad=True,
        role_description="code generation context for an AI coding assistant: virtual nodes to implement, context nodes for reference, relationships",
    )

    loss_fn = tg.TextLoss(
        "Evaluate this code generation context. Does it clearly specify what to implement? "
        "Are virtual nodes and file locations unambiguous? Is there sufficient context for integration? "
        "Identify gaps or ambiguities. Provide concise feedback. Do not rewrite the content yourself."
    )

    optimizer = tg.TGD(parameters=[markdown])

    for _ in range(args.steps):
        loss = loss_fn(markdown)
        loss.backward()
        optimizer.step()

    sys.stdout.write(markdown.value)


if __name__ == "__main__":
    main()
