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
		Description: "Log in to a Moodle instance. Session cookies are persisted in the browser profile so subsequent calls don't need to re-authenticate.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site, e.g. https://moodle.example.com",
			},
			"username": map[string]any{
				"type":        "string",
				"description": "Moodle username.",
			},
			"password": map[string]any{
				"type":        "string",
				"description": "Moodle password.",
			},
		},
		Required: []string{"base_url", "username", "password"},
	}
}

func (MoodleLoginTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		BaseURL  string `json:"base_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	in.BaseURL = strings.TrimRight(in.BaseURL, "/")
	loginURL := in.BaseURL + "/login/index.php"

	ctx, cancel := context.WithTimeout(browser.Get(), 30*time.Second)
	defer cancel()

	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible("#username", chromedp.ByQuery),
		chromedp.Clear("#username", chromedp.ByQuery),
		chromedp.SendKeys("#username", in.Username, chromedp.ByQuery),
		chromedp.Clear("#password", chromedp.ByQuery),
		chromedp.SendKeys("#password", in.Password, chromedp.ByQuery),
		chromedp.Click("#loginbtn", chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		return fmt.Sprintf("login error: %v", err), true
	}

	// Detect login failure: Moodle stays on /login/index.php with an error
	if strings.Contains(currentURL, "/login/index.php") {
		var errText string
		_ = chromedp.Run(ctx, chromedp.Text(".loginerrors, #loginerrormessage", &errText, chromedp.ByQuery))
		if errText == "" {
			errText = "login failed (still on login page)"
		}
		return errText, true
	}
	return fmt.Sprintf("logged in as %s — current page: %s", in.Username, currentURL), false
}
