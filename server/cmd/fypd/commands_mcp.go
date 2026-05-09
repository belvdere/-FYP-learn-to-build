package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/fyp/pkg/mcp"
)

// serveMCP starts the MCP server for GitHub Copilot integration
func serveMCP(args []string) {
	fs := flag.NewFlagSet("serve-mcp", flag.ExitOnError)
	workspace := fs.String("workspace", ".", "workspace directory to index")
	fs.Parse(args)

	fmt.Fprintf(os.Stderr, "fypd: MCP server starting (workspace: %s)\n", *workspace)

	// Start MCP server
	server, err := mcp.NewServer(*workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fypd: failed to create MCP server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "fypd: MCP server error: %v\n", err)
		os.Exit(1)
	}
}
