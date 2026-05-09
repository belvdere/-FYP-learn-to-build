package main

import (
	"fmt"
	"os"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve-mcp":
		serveMCP(args)
	case "serve-api-stdio":
		serveAPIStdioCommand(args)
	case "version":
		fmt.Printf("fypd version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `fypd - Local knowledge database daemon for FYP

Usage:
  fypd <command> [options]

Commands:
  serve-mcp       Start MCP server (stdio JSON-RPC for GitHub Copilot)
  serve-api-stdio Start API server over stdio (for VS Code webview)
  version         Print version
  help            Show this help

Examples:
  fypd serve-mcp --workspace /path/to/workspace
  fypd serve-api-stdio --workspace /path/to/workspace

`)
}
