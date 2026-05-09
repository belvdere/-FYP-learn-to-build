package lang

import (
	"example.com/fyp/pkg/parser/golang"
	"example.com/fyp/pkg/parser/java"
	"example.com/fyp/pkg/parser/javascript"
	"example.com/fyp/pkg/parser/python"
	"example.com/fyp/pkg/parser/typescript"
)

// NewLanguageControllerWithDefaults creates a language controller with all built-in plugins registered.
func NewLanguageControllerWithDefaults() *LanguageController {
	ctrl := NewLanguageController()

	// Register built-in languages
	javaPlugin := java.NewJavaPlugin()
	ctrl.RegisterPlugin(javaPlugin)
	pythonPlugin := python.NewPythonPlugin()
	ctrl.RegisterPlugin(pythonPlugin)
	typeScriptPlugin := typescript.NewTypeScriptPlugin()
	ctrl.RegisterPlugin(typeScriptPlugin)
	javaScriptPlugin := javascript.NewJavaScriptPlugin()
	ctrl.RegisterPlugin(javaScriptPlugin)
	goPlugin := golang.NewGoPlugin()
	ctrl.RegisterPlugin(goPlugin)

	return ctrl
}
