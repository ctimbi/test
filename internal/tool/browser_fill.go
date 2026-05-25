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

type BrowserFillTool struct{}

func init() { Default.Register(&BrowserFillTool{}) }

func (BrowserFillTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_fill",
		Description: "Type text into a form input or textarea selected by a CSS selector. Clears existing content first by default.",
		InputSchema: map[string]any{
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector of the input element.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text to type into the element.",
			},
			"clear": map[string]any{
				"type":        "boolean",
				"description": "Clear the field before typing. Defaults to true.",
			},
		},
		Required: []string{"selector", "text"},
	}
}

func (BrowserFillTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Selector string `json:"selector"`
		Text     string `json:"text"`
		Clear    *bool  `json:"clear"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}

	doClear := in.Clear == nil || *in.Clear
	ctx, cancel := context.WithTimeout(browser.Get(), 15*time.Second)
	defer cancel()

	actions := []chromedp.Action{
		chromedp.WaitVisible(in.Selector, chromedp.ByQuery),
	}
	if doClear {
		actions = append(actions, chromedp.Clear(in.Selector, chromedp.ByQuery))
	}
	actions = append(actions, chromedp.SendKeys(in.Selector, in.Text, chromedp.ByQuery))

	if err := chromedp.Run(ctx, actions...); err != nil {
		return fmt.Sprintf("fill error: %v", err), true
	}
	return fmt.Sprintf("filled %s with %q", in.Selector, in.Text), false
}
