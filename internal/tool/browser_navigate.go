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

type BrowserNavigateTool struct{}

func init() { Default.Register(&BrowserNavigateTool{}) }

func (BrowserNavigateTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_navigate",
		Description: "Navigate the browser to a URL and wait for the page body to load.",
		InputSchema: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to navigate to.",
			},
		},
		Required: []string{"url"},
	}
}

func (BrowserNavigateTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	ctx, cancel := context.WithTimeout(browser.Get(), 30*time.Second)
	defer cancel()
	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(in.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}
	return fmt.Sprintf("navigated to: %s", currentURL), false
}
