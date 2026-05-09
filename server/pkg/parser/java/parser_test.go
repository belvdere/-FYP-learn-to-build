package java

import (
	"os"
	"path/filepath"
	"testing"

	"example.com/fyp/pkg/parser"
)

const sampleJavaCode = `
package com.example;

import java.util.List;
import java.util.ArrayList;

public class HelloWorld {
    private String message;
    
    public HelloWorld(String message) {
        this.message = message;
    }
    
    public void sayHello() {
        System.out.println(message);
    }
    
    public static void main(String[] args) {
        HelloWorld hw = new HelloWorld("Hello, World!");
        hw.sayHello();
    }
}
`

func TestJavaParser(t *testing.T) {
	parser := NewJavaParser()
	defer parser.Close()

	tree, err := parser.Parse([]byte(sampleJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tree == nil {
		t.Fatal("tree is nil")
	}

	root := tree.RootNode()
	if root == nil {
		t.Fatal("root node is nil")
	}

	// Check that we have a program node
	if root.Type() != "program" {
		t.Errorf("expected root type 'program', got '%s'", root.Type())
	}
}

func TestFindNodesByType(t *testing.T) {
	javaParser := NewJavaParser()
	defer javaParser.Close()

	tree, err := javaParser.Parse([]byte(sampleJavaCode))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := tree.RootNode()

	// Find class declarations
	classes := parser.FindNodesByType(root, "class_declaration")
	if len(classes) != 1 {
		t.Errorf("expected 1 class, found %d", len(classes))
	}

	// Find method declarations
	methods := parser.FindNodesByType(root, "method_declaration")
	if len(methods) < 2 {
		t.Errorf("expected at least 2 methods, found %d", len(methods))
	}

	// Find import declarations
	imports := parser.FindNodesByType(root, "import_declaration")
	if len(imports) != 2 {
		t.Errorf("expected 2 imports, found %d", len(imports))
	}
}

func TestGetNodeText(t *testing.T) {
	javaParser := NewJavaParser()
	defer javaParser.Close()

	content := []byte(sampleJavaCode)
	tree, err := javaParser.Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := tree.RootNode()

	// Find class and check name
	classes := parser.FindNodesByType(root, "class_declaration")
	if len(classes) == 0 {
		t.Fatal("no classes found")
	}

	// The class node should contain "class HelloWorld"
	classText := parser.GetNodeText(classes[0], content)
	if len(classText) == 0 {
		t.Error("class text is empty")
	}
}

func TestParseFile(t *testing.T) {
	// Create a temporary Java file
	tmpDir, err := os.MkdirTemp("", "parser-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	javaFile := filepath.Join(tmpDir, "Test.java")
	err = os.WriteFile(javaFile, []byte(sampleJavaCode), 0644)
	if err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewJavaParser()
	defer parser.Close()

	tree, content, err := parser.ParseFile(javaFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if tree == nil {
		t.Fatal("tree is nil")
	}

	if len(content) == 0 {
		t.Fatal("content is empty")
	}

	root := tree.RootNode()
	if root.Type() != "program" {
		t.Errorf("expected root type 'program', got '%s'", root.Type())
	}
}
