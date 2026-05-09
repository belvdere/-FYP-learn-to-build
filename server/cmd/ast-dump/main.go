// ast-dump parses Java code and prints its AST structure.
// Usage:
//   go run ./cmd/ast-dump < file.java
//   echo 'public class Foo { void bar() {} }' | go run ./cmd/ast-dump
//   go run ./cmd/ast-dump --code 'public class Foo { void bar() {} }'
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

func main() {
	codeFlag := flag.String("code", "", "Java code to parse (optional; otherwise reads from stdin)")
	flag.Parse()

	var input []byte
	var err error

	if *codeFlag != "" {
		input = []byte(*codeFlag)
	} else {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
	}

	if len(input) == 0 {
		fmt.Fprintln(os.Stderr, "No input. Provide code via --code or stdin.")
		os.Exit(1)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	defer parser.Close()

	tree, err := parser.ParseCtx(context.Background(), nil, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}
	defer tree.Close()

	root := tree.RootNode()
	fmt.Println("AST (tree-sitter):")
	fmt.Println(strings.Repeat("=", 60))
	printNode(root, input, 0)
	if root.HasError() {
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println("⚠ Parse tree contains errors")
	}
}

func printNode(node *sitter.Node, content []byte, depth int) {
	if node == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	nodeType := node.Type()
	if nodeType == "" {
		nodeType = "(anonymous)"
	}

	// Byte range and position
	startPt := node.StartPoint()
	endPt := node.EndPoint()
	lineRange := fmt.Sprintf("L%d:%d–L%d:%d",
		startPt.Row+1, startPt.Column,
		endPt.Row+1, endPt.Column,
	)

	// Text slice (truncate if long)
	text := string(content[node.StartByte():node.EndByte()])
	if len(text) > 50 {
		text = strings.ReplaceAll(text[:47]+"...", "\n", " ")
	} else {
		text = strings.ReplaceAll(text, "\n", " ")
	}

	fmt.Printf("%s%s [%s] %q\n", indent, nodeType, lineRange, text)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		printNode(child, content, depth+1)
	}
}
