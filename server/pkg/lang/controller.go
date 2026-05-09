package lang

import (
	"fmt"
	"path/filepath"
	"strings"
)

// LanguageController manages language plugins and routes files to appropriate handlers.
type LanguageController struct {
	plugins map[string]LanguagePlugin
}

// NewLanguageController creates a new empty language controller.
// Use RegisterPlugin to add language support.
func NewLanguageController() *LanguageController {
	return &LanguageController{
		plugins: make(map[string]LanguagePlugin),
	}
}

// RegisterPlugin registers a language plugin.
func (c *LanguageController) RegisterPlugin(plugin LanguagePlugin) {
	lang := plugin.Language()
	c.plugins[lang] = plugin
}

// GetPlugin returns the appropriate plugin for a file based on extension.
func (c *LanguageController) GetPlugin(filePath string) (LanguagePlugin, error) {
	// First, try detection by file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	for _, plugin := range c.plugins {
		for _, pluginExt := range plugin.Extensions() {
			if ext == pluginExt {
				return plugin, nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported language for file: %s", filePath)
}

// GetPluginByLanguage returns the plugin for a specific language name.
func (c *LanguageController) GetPluginByLanguage(langName string) (LanguagePlugin, error) {
	plugin, exists := c.plugins[langName]
	if !exists {
		return nil, fmt.Errorf("no plugin registered for language: %s", langName)
	}
	return plugin, nil
}

// GetSupportedLanguages returns a list of supported language names.
func (c *LanguageController) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(c.plugins))
	for lang := range c.plugins {
		languages = append(languages, lang)
	}
	return languages
}

// HasLanguage checks if a language is supported.
func (c *LanguageController) HasLanguage(langName string) bool {
	_, exists := c.plugins[langName]
	return exists
}

// GetPluginForFile is an alias for GetPlugin for consistency.
func (c *LanguageController) GetPluginForFile(filePath string) (LanguagePlugin, error) {
	return c.GetPlugin(filePath)
}

// GetDefaultPlugin returns the first registered plugin (useful when no file context is available).
// Returns nil if no plugins are registered.
func (c *LanguageController) GetDefaultPlugin() LanguagePlugin {
	for _, plugin := range c.plugins {
		return plugin
	}
	return nil
}
