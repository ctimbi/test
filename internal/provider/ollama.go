package provider

import (
	"os"

	"github.com/openai/openai-go/option"
)

// OllamaProvider connects to a local Ollama instance via its OpenAI-compatible
// /v1 endpoint. No API key or external account required — Ollama runs entirely
// on your machine.
//
// Configure via environment:
//
//	OLLAMA_BASE_URL  base URL of the Ollama server (default: http://localhost:11434/v1)
//	OLLAMA_MODEL     model to use (default: qwen2.5-coder:7b)
//
// Switch at runtime with: /provider ollama [model]
type OllamaProvider struct {
	*OpenAIProvider
}

func NewOllamaProvider(model, system string, maxTokens int64) *OllamaProvider {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	if model == "" {
		model = ollamaDefaultModel
	}
	p := NewOpenAICompatProvider(model, system, maxTokens,
		option.WithBaseURL(baseURL),
		option.WithAPIKey("ollama"),
	)
	return &OllamaProvider{p}
}

// EstimatedCostUSD returns 0 — Ollama runs locally at no token cost.
func (p *OllamaProvider) EstimatedCostUSD() float64 { return 0 }

const ollamaDefaultModel = "qwen2.5-coder:7b"

// OllamaModels returns suggestions for /model when the ollama provider is active.
// Any model installed locally can be used — this list is just a starting point.
func OllamaModels() []string { return ollamaModels }

var ollamaModels = []string{
	"qwen2.5-coder:7b",
	"qwen2.5-coder:14b",
	"llama3.1:8b",
	"llama3.1:70b",
	"deepseek-coder-v2:16b",
	"codellama:13b",
	"mistral:7b",
}
