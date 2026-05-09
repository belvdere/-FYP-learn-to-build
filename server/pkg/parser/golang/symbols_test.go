package golang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSymbols_Functions(t *testing.T) {
	code := `package main

func HelloWorld() {
}

func Add(a, b int) int {
	return a + b
}
`
	p := NewGoParser()
	tree, err := p.Parse([]byte(code))
	require.NoError(t, err)
	defer tree.Close()

	extractor := NewGoSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(code), "main.go")

	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}

	assert.Contains(t, names, "HelloWorld()")
	assert.Contains(t, names, "Add(int,int)")
}

func TestExtractSymbols_Methods(t *testing.T) {
	code := `package main

type Server struct{}

func (s *Server) Start() error {
	return nil
}

func (s Server) Stop() {
}
`
	p := NewGoParser()
	tree, err := p.Parse([]byte(code))
	require.NoError(t, err)
	defer tree.Close()

	extractor := NewGoSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(code), "server.go")

	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}

	assert.Contains(t, names, "Server.Start()")
	assert.Contains(t, names, "Server.Stop()")
}

func TestExtractSymbols_StructAndInterface(t *testing.T) {
	code := `package main

type Handler interface {
	Handle() error
}

type Config struct {
	Port int
}
`
	p := NewGoParser()
	tree, err := p.Parse([]byte(code))
	require.NoError(t, err)
	defer tree.Close()

	extractor := NewGoSymbolExtractor()
	symbols := extractor.ExtractSymbols(tree, []byte(code), "types.go")

	kindByName := map[string]string{}
	for _, s := range symbols {
		kindByName[s.Name] = s.Kind
	}

	assert.Equal(t, "interface", kindByName["Handler"])
	assert.Equal(t, "struct", kindByName["Config"])
}
