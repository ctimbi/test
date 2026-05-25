package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ctimbi/test/internal/api"
	"github.com/ctimbi/test/internal/browser"
)

type MoodleLoginTool struct{}

func init() { Default.Register(&MoodleLoginTool{}) }

func (MoodleLoginTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "moodle_login",
		Description: "Open the Moodle login page in the browser and wait for the user to log in manually. Returns once the user is logged in. Session cookies are persisted so subsequent calls are not needed.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site, e.g. https://moodle.example.com",
			},
			"timeout_minutes": map[string]any{
				"type":        "integer",
				"description": "Minutes to wait for the user to log in. Default 5.",
			},
		},
		Required: []string{"base_url"},
	}
}

func (MoodleLoginTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		BaseURL        string `json:"base_url"`
		TimeoutMinutes int    `json:"timeout_minutes"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	in.BaseURL = strings.TrimRight(in.BaseURL, "/")
	if in.TimeoutMinutes <= 0 {
		in.TimeoutMinutes = 5
	}

	loginURL := in.BaseURL + "/login/index.php"
	timeout := time.Duration(in.TimeoutMinutes) * time.Minute

	ctx, cancel := context.WithTimeout(browser.Get(), timeout)
	defer cancel()

	// Navigate to login page
	if err := chromedp.Run(ctx, chromedp.Navigate(loginURL)); err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	// Poll every second until the URL moves away from /login/index.php
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)

		var currentURL string
		if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err != nil {
			// browser may have been closed
			return fmt.Sprintf("browser error: %v", err), true
		}

		if !strings.Contains(currentURL, "/login/index.php") {
			// Extract logged-in username if available
			var username string
			_ = chromedp.Run(ctx, chromedp.Text(
				".usermenu .usertext, [data-username], .userinfo .username",
				&username, chromedp.ByQuery,
			))
			username = strings.TrimSpace(username)
			if username != "" {
				return fmt.Sprintf("logged in as %s — current page: %s", username, currentURL), false
			}
			return fmt.Sprintf("login detected — current page: %s", currentURL), false
		}
	}

	return fmt.Sprintf("timeout: user did not log in within %d minutes", in.TimeoutMinutes), true
}
