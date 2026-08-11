package settings

import (
	"fmt"
	"net/url"
	"strings"
)

// LLM provider kinds (chat completions).
const (
	LLMProviderDisabled         = "disabled"
	LLMProviderOpenAICompatible = "openai_compatible"
)

// LLMConfig is stored under settings keys llm.*.
// OpenAI-compatible Chat Completions API (OpenAI, DeepSeek, Ollama, etc.).
type LLMConfig struct {
	Provider    string  `json:"provider"`
	BaseURL     string  `json:"baseUrl"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"` // 0–2, default 0.3
	MaxTokens   int     `json:"maxTokens"`   // completion budget; 0 = provider default
	// SystemPrompt is an optional default system message for future features.
	SystemPrompt string `json:"systemPrompt"`
}

// DefaultLLMConfig returns disabled LLM.
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Provider:    LLMProviderDisabled,
		Temperature: 0.3,
		MaxTokens:   2048,
	}
}

// IsConfigured reports whether chat can be used.
func (c LLMConfig) IsConfigured() bool {
	if c.Provider == "" || c.Provider == LLMProviderDisabled {
		return false
	}
	if c.Provider != LLMProviderOpenAICompatible {
		return false
	}
	if strings.TrimSpace(c.Model) == "" {
		return false
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return false
	}
	return true
}

// Validate checks fields when enabling the LLM.
func (c LLMConfig) Validate() error {
	if c.Provider == "" || c.Provider == LLMProviderDisabled {
		return nil
	}
	if c.Provider != LLMProviderOpenAICompatible {
		return fmt.Errorf("unsupported llm provider %q", c.Provider)
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("llm model is required")
	}
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return fmt.Errorf("base URL is required")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid base URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL must be http(s)")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if c.MaxTokens < 0 || c.MaxTokens > 128000 {
		return fmt.Errorf("max tokens out of range")
	}
	return nil
}

// Normalize fills defaults and trims strings.
func (c LLMConfig) Normalize() LLMConfig {
	if c.Provider == "" {
		c.Provider = LLMProviderDisabled
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model = strings.TrimSpace(c.Model)
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.SystemPrompt = strings.TrimSpace(c.SystemPrompt)
	if c.Temperature < 0 {
		c.Temperature = 0
	}
	if c.Temperature > 2 {
		c.Temperature = 2
	}
	// 0 max tokens = leave to provider; UI default is 2048 when enabling.
	if c.MaxTokens < 0 {
		c.MaxTokens = 0
	}
	return c
}

// Masked returns a copy safe for UI (api key redacted).
func (c LLMConfig) Masked() LLMConfig {
	out := c
	if out.APIKey != "" {
		if len(out.APIKey) <= 8 {
			out.APIKey = "***"
		} else {
			out.APIKey = out.APIKey[:3] + "***" + out.APIKey[len(out.APIKey)-2:]
		}
	}
	return out
}
