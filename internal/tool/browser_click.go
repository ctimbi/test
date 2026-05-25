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

type BrowserClickTool struct{}

func init() { Default.Register(&BrowserClickTool{}) }

func (BrowserClickTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_click",
		Description: "Click on a DOM element selected by a CSS selector. Waits for the element to be visible first.",
		InputSchema: map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the element to click.",
			},
		},
		Required: []string{"selector"},
	}
}

func (BrowserClickTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Selector string `json:"selector"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	ctx, cancel := context.WithTimeout(browser.Get(), 15*time.Second)
	defer cancel()
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(in.Selector, chromedp.ByQuery),
		chromedp.Click(in.Selector, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Sprintf("click error: %v", err), true
	}
	return fmt.Sprintf("clicked: %s", in.Selector), false
}
