package llm

import (
	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"go.uber.org/zap"
)

// Config holds LLM client configuration
type Config struct {
	Provider    string
	APIKey      string
	Model       string
	Temperature float32
	MaxTokens   int
}

// NewGeminiClientFromConfig creates a Gemini client from config
// This avoids import cycles by not defining a Client interface that depends on agent types
func NewGeminiClientFromConfig(config Config, mcpClient *mcp.Client, logger *zap.Logger) (*GeminiClient, error) {
	return NewGeminiClient(config, mcpClient, logger)
}

// ErrUnsupportedProvider is returned when an unsupported LLM provider is specified
type ErrUnsupportedProvider struct {
	Provider string
}

func (e ErrUnsupportedProvider) Error() string {
	return "unsupported LLM provider: " + e.Provider
}
