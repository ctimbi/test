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
		Description: "Open the Moodle login page in the browser and wait for the user to log in manually. Detects login automatically via the body.loggedin CSS class that Moodle adds after authentication.",
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

	// Navigate to login page and wait for it to load
	if err := chromedp.Run(ctx,
		chromedp.Navigate(loginURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	fmt.Printf("\n[moodle_login] Browser is open — please log in. Waiting for up to %d minutes...\n", in.TimeoutMinutes)

	// JS that returns true via multiple signals (handles themes that omit body.loggedin)
	const isLoggedIn = `(function(){
  if (document.body.classList.contains('loggedin')) return true;
  if (window.M && window.M.cfg && M.cfg.userid && M.cfg.userid > 0) return true;
  var bd = document.body.dataset;
  if (bd && bd.userid && bd.userid !== '0') return true;
  if (document.querySelector('.usermenu,[data-region="user-menu"],.userinfo')) return true;
  return false;
})()`

	deadline := time.Now().Add(timeout)
	lastPrint := time.Now()
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)

		remaining := time.Until(deadline).Round(time.Second)
		if time.Since(lastPrint) >= 30*time.Second {
			fmt.Printf("[moodle_login] still waiting... %v remaining\n", remaining)
			lastPrint = time.Now()
		}

		var loggedIn bool
		var currentURL string
		err := chromedp.Run(ctx,
			chromedp.Location(&currentURL),
			chromedp.Evaluate(isLoggedIn, &loggedIn),
		)
		if err != nil {
			// transient CDP error — keep trying unless context is done
			if ctx.Err() != nil {
				return fmt.Sprintf("browser error: %v", err), true
			}
			continue
		}

		if loggedIn {
			var username string
			_ = chromedp.Run(ctx, chromedp.Evaluate(
				`(document.querySelector('.usermenu .usertext,[data-username],.userinfo')?.textContent||'').trim().split('\n')[0]`,
				&username,
			))
			username = strings.TrimSpace(username)
			if username != "" {
				return fmt.Sprintf("logged in as %s", username), false
			}
			return fmt.Sprintf("login detected — current page: %s", currentURL), false
		}
	}

	return fmt.Sprintf("timeout after %d minutes — if you are already logged in, just tell me and I will continue", in.TimeoutMinutes), true
}
