package provider

import (
	"os"

	"github.com/openai/openai-go/option"
)

// OpenRouterProvider routes LLM requests through openrouter.ai, which
// provides a single API key that unlocks hundreds of models from Anthropic,
// OpenAI, Meta, Mistral, Google, and others — billed per token from a shared
// balance.
//
// Configure via environment:
//
//	OPENROUTER_API_KEY  your key from https://openrouter.ai/keys (required)
//
// Model names use the provider/model format, e.g.:
//
//	anthropic/claude-sonnet-4-5
//	openai/gpt-4o
//	meta-llama/llama-3.1-70b-instruct
//	mistralai/mistral-7b-instruct
//
// Switch at runtime with: /provider openrouter [model]
type OpenRouterProvider struct {
	*OpenAIProvider
}

func NewOpenRouterProvider(model, system string, maxTokens int64) *OpenRouterProvider {
	if model == "" {
		model = openrouterDefaultModel
	}
	p := NewOpenAICompatProvider(model, system, maxTokens,
		option.WithBaseURL("https://openrouter.ai/api/v1"),
		option.WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		option.WithHeader("HTTP-Referer", "https://github.com/ctimbi/test"),
		option.WithHeader("X-Title", "BYO Coding Agent"),
	)
	return &OpenRouterProvider{p}
}

const openrouterDefaultModel = "anthropic/claude-sonnet-4-5"

// OpenRouterModels returns popular model suggestions for /model when openrouter
// is active. Any model slug from openrouter.ai/models can be used.
func OpenRouterModels() []string { return openrouterModels }

var openrouterModels = []string{
	"anthropic/claude-sonnet-4-5",
	"anthropic/claude-opus-4-7",
	"anthropic/claude-haiku-4-5",
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"openai/gpt-5-codex",
	"meta-llama/llama-3.1-70b-instruct",
	"mistralai/mistral-7b-instruct",
	"deepseek/deepseek-coder",
}
