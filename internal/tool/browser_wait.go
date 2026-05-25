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

type BrowserCurrentURLTool struct{}

func init() { Default.Register(&BrowserCurrentURLTool{}) }

func (BrowserCurrentURLTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_current_url",
		Description: "Return the current URL of the browser tab.",
		InputSchema: map[string]any{},
		Required:    []string{},
	}
}

func (BrowserCurrentURLTool) Execute(_ context.Context, _ string) (string, bool) {
	ctx, cancel := context.WithTimeout(browser.Get(), 5*time.Second)
	defer cancel()
	var u string
	if err := chromedp.Run(ctx, chromedp.Location(&u)); err != nil {
		return fmt.Sprintf("location error: %v", err), true
	}
	return u, false
}

type BrowserWaitTool struct{}

func init() { Default.Register(&BrowserWaitTool{}) }

func (BrowserWaitTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_wait",
		Description: "Wait until a CSS selector becomes visible on the page. Useful after navigation or async actions.",
		InputSchema: map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector to wait for.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Maximum seconds to wait. Default 15.",
			},
		},
		Required: []string{"selector"},
	}
}

func (BrowserWaitTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Selector       string `json:"selector"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 15
	}
	ctx, cancel := context.WithTimeout(browser.Get(), time.Duration(in.TimeoutSeconds)*time.Second)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.WaitVisible(in.Selector, chromedp.ByQuery)); err != nil {
		return fmt.Sprintf("wait error: %v", err), true
	}
	return fmt.Sprintf("element visible: %s", in.Selector), false
}
