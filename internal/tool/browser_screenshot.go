package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ctimbi/test/internal/api"
	"github.com/ctimbi/test/internal/browser"
)

type BrowserScreenshotTool struct{}

func init() { Default.Register(&BrowserScreenshotTool{}) }

func (BrowserScreenshotTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "browser_screenshot",
		Description: "Take a full-page screenshot of the current browser tab and save it to disk. Returns the path to the saved PNG.",
		InputSchema: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Optional file path to save the screenshot. Defaults to .harness/screenshots/screenshot-<timestamp>.png",
			},
		},
		Required: []string{},
	}
}

func (BrowserScreenshotTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal([]byte(rawInput), &in)

	if in.Path == "" {
		dir := ".harness/screenshots"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Sprintf("mkdir error: %v", err), true
		}
		in.Path = filepath.Join(dir, fmt.Sprintf("screenshot-%d.png", time.Now().UnixMilli()))
	}

	ctx, cancel := context.WithTimeout(browser.Get(), 20*time.Second)
	defer cancel()

	var buf []byte
	err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
	if err != nil {
		return fmt.Sprintf("screenshot error: %v", err), true
	}
	if err := os.WriteFile(in.Path, buf, 0o644); err != nil {
		return fmt.Sprintf("write error: %v", err), true
	}
	return fmt.Sprintf("screenshot saved: %s (%d bytes)", in.Path, len(buf)), false
}
