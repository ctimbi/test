package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ctimbi/test/internal/api"
	"github.com/ctimbi/test/internal/browser"
)

type BrowserEvalJSTool struct{}

func init() { Default.Register(&BrowserEvalJSTool{}) }

func (BrowserEvalJSTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_eval_js",
		Description: "Evaluate a JavaScript expression in the current page context and return the JSON-serialized result.",
		InputSchema: map[string]any{
			"expression": map[string]any{
				"type":        "string",
				"description": "JavaScript expression to evaluate. Must return a JSON-serializable value.",
			},
		},
		Required: []string{"expression"},
	}
}

func (BrowserEvalJSTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	ctx, cancel := context.WithTimeout(browser.Get(), 10*time.Second)
	defer cancel()
	var result any
	err := chromedp.Run(ctx, chromedp.Evaluate(in.Expression, &result))
	if err != nil {
		return fmt.Sprintf("eval error: %v", err), true
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("%v", result), false
	}
	return string(b), false
}
