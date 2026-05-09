package indexer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/lang"
	"example.com/fyp/pkg/types"
)

// Scanner performs tree-sitter symbol scanning with no database access.
// Use this for index.scan / index.scanFile to avoid opening a competing DB connection.
type Scanner struct {
	controller *lang.LanguageController
}

// NewScanner creates a Scanner backed by the default language plugins.
func NewScanner() *Scanner {
	return &Scanner{controller: lang.NewLanguageControllerWithDefaults()}
}

// Close releases parser resources held by language plugins.
func (sc *Scanner) Close() {
	for _, language := range sc.controller.GetSupportedLanguages() {
		if plugin, err := sc.controller.GetPluginByLanguage(language); err == nil && plugin != nil {
			plugin.Parser().Close()
		}
	}
}

// ScanSymbols scans all supported source files under workspaceRoot.
func (sc *Scanner) ScanSymbols(ctx context.Context, workspaceRoot string) ([]types.ScannedSymbol, error) {
	return scanSymbolsWithController(ctx, workspaceRoot, sc.controller)
}

// ScanFileSymbols scans a single source file.
func (sc *Scanner) ScanFileSymbols(ctx context.Context, filePath string) ([]types.ScannedSymbol, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return scanFileSymbolsWithController(filePath, sc.controller)
}

// Indexer provides scan/query functionality for the local code graph database.
type Indexer struct {
	store      *db.Store
	controller *lang.LanguageController
}

// NewIndexer creates a new indexer instance.
func NewIndexer(workspaceRoot string) (*Indexer, error) {
	store, err := db.NewStore(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	controller := lang.NewLanguageControllerWithDefaults()

	return &Indexer{
		store:      store,
		controller: controller,
	}, nil
}

// Close releases resources.
func (idx *Indexer) Close() error {
	for _, language := range idx.controller.GetSupportedLanguages() {
		if plugin, err := idx.controller.GetPluginByLanguage(language); err == nil && plugin != nil {
			plugin.Parser().Close()
		}
	}
	return idx.store.Close()
}

// Store returns the database store for direct access.
func (idx *Indexer) Store() *db.Store {
	return idx.store
}

// ScanSymbols scans workspace files and returns callable symbols for LSP-based resolution.
func (idx *Indexer) ScanSymbols(ctx context.Context, workspaceRoot string) ([]types.ScannedSymbol, error) {
	return scanSymbolsWithController(ctx, workspaceRoot, idx.controller)
}

// ScanFileSymbols scans a single source file and returns callable symbols for LSP-based resolution.
func (idx *Indexer) ScanFileSymbols(ctx context.Context, filePath string) ([]types.ScannedSymbol, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return scanFileSymbolsWithController(filePath, idx.controller)
}

func pathToFileURI(filePath string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(filePath),
	}
	return u.String()
}

func scanSymbolsWithController(ctx context.Context, workspaceRoot string, controller *lang.LanguageController) ([]types.ScannedSymbol, error) {
	sourceFiles, err := findSourceFiles(workspaceRoot, controller)
	if err != nil {
		return nil, fmt.Errorf("find source files: %w", err)
	}

	result := make([]types.ScannedSymbol, 0, len(sourceFiles)*8)
	for _, filePath := range sourceFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		symbols, err := scanFileSymbolsWithController(filePath, controller)
		if err != nil {
			continue
		}
		result = append(result, symbols...)
	}

	return result, nil
}

func scanFileSymbolsWithController(filePath string, controller *lang.LanguageController) ([]types.ScannedSymbol, error) {
	plugin, err := controller.GetPlugin(filePath)
	if err != nil {
		return nil, err
	}

	tree, content, err := plugin.Parser().ParseFile(filePath)
	if err != nil {
		return nil, err
	}

	symbols := plugin.SymbolExtractor().ExtractSymbols(tree, content, filePath)
	result := make([]types.ScannedSymbol, 0, len(symbols))
	for _, sym := range symbols {
		if sym.Kind != "method" && sym.Kind != "constructor" && sym.Kind != "function" {
			continue
		}
		uri := pathToFileURI(sym.FilePath)
		result = append(result, types.ScannedSymbol{
			ID:        sym.ID,
			Name:      sym.Name,
			Kind:      sym.Kind,
			FilePath:  sym.FilePath,
			URI:       uri,
			Line:      sym.Line,
			EndLine:   sym.EndLine,
			Col:       sym.Col,
			Signature: sym.Signature,
			BodyHash:  sym.BodyHash,
		})
	}

	return result, nil
}

// findSourceFiles recursively finds all source files for supported languages in a directory.
func findSourceFiles(root string, controller *lang.LanguageController) ([]string, error) {
	var sourceFiles []string

	supportedExts := make(map[string]bool)
	for _, language := range controller.GetSupportedLanguages() {
		plugin, err := controller.GetPluginByLanguage(language)
		if err != nil {
			continue
		}
		for _, ext := range plugin.Extensions() {
			supportedExts[ext] = true
		}
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "target" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if supportedExts[ext] {
			sourceFiles = append(sourceFiles, path)
		}

		return nil
	})

	return sourceFiles, err
}
