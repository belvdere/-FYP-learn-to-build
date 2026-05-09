package config

import (
	"os"
)

// LLMProvider represents the type of LLM provider
type LLMProvider string

const (
	ProviderOpenAI LLMProvider = "openai"
)

// GetLLMProvider returns the LLM provider from environment
// Returns empty string if not configured
func GetLLMProvider() LLMProvider {
	provider := os.Getenv("LLM_PROVIDER")
	return LLMProvider(provider)
}

// GetOpenAIAPIKey returns the OpenAI API key from environment
func GetOpenAIAPIKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

// GetOpenAIBaseURL returns the OpenAI API base URL, defaults to official API
func GetOpenAIBaseURL() string {
	url := os.Getenv("OPENAI_BASE_URL")
	if url == "" {
		return "https://api.openai.com/v1"
	}
	return url
}

// GetLLMModel returns the model name to use, with provider-specific defaults
func GetLLMModel() string {
	model := os.Getenv("LLM_MODEL")
	if model != "" {
		return model
	}

	// Default models per provider
	provider := GetLLMProvider()
	switch provider {
	case ProviderOpenAI:
		return "gpt-4o-mini" // Cost-effective default
	default:
		return ""
	}
}

// IsLLMAvailable checks if LLM is configured (not mock)
func IsLLMAvailable() bool {
	provider := GetLLMProvider()
	switch provider {
	case ProviderOpenAI:
		return GetOpenAIAPIKey() != ""
	default:
		return false
	}
}
