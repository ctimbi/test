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

type BrowserGetTextTool struct{}

func init() { Default.Register(&BrowserGetTextTool{}) }

func (BrowserGetTextTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_get_text",
		Description: "Return the visible text content of a DOM element (equivalent to element.innerText). Use 'body' as the selector to get all page text.",
		InputSchema: map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the element to read.",
			},
		},
		Required: []string{"selector"},
	}
}

func (BrowserGetTextTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	ctx, cancel := context.WithTimeout(browser.Get(), 10*time.Second)
	defer cancel()
	var text string
	err := chromedp.Run(ctx,
		chromedp.WaitReady(in.Selector, chromedp.ByQuery),
		chromedp.Text(in.Selector, &text, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Sprintf("get_text error: %v", err), true
	}
	return text, false
}
