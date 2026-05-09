package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/fyp/pkg/api"
	"example.com/fyp/pkg/config"
	"example.com/fyp/pkg/db"
)

// serveAPIStdioCommand handles the "serve-api-stdio" subcommand to start the stdio API server.
func serveAPIStdioCommand(args []string) {
	fs := flag.NewFlagSet("serve-api-stdio", flag.ExitOnError)
	workspace := fs.String("workspace", "", "Path to the workspace (required)")

	fs.Parse(args)

	// Validate workspace
	if *workspace == "" {
		fmt.Fprintf(os.Stderr, "Error: --workspace is required\n")
		fs.Usage()
		os.Exit(1)
	}

	// Check if workspace exists
	if _, err := os.Stat(*workspace); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: workspace does not exist: %s\n", *workspace)
		os.Exit(1)
	}

	// Open database
	store, err := db.NewStore(*workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Fprintf(os.Stderr, "fypd: Starting API stdio server\n")
	fmt.Fprintf(os.Stderr, "fypd: Workspace: %s\n", *workspace)
	fmt.Fprintf(os.Stderr, "fypd: Database: %s\n", config.GetDBPath(*workspace))

	// Create and start stdio server
	server := api.NewStdioServer(store, *workspace)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: server failed: %v\n", err)
		os.Exit(1)
	}
}
