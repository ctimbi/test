// Package browser manages a singleton Chrome instance shared across all
// browser_* and moodle_* tool calls. The browser runs in headed (visible)
// mode so the user can watch and interact with it. Session data (cookies,
// localStorage) persists between CLI restarts via --user-data-dir.
package browser

import (
	"context"
	"sync"

	"github.com/chromedp/chromedp"
)

var (
	once     sync.Once
	allocCtx context.Context
	tabCtx   context.Context
	cancel   context.CancelFunc
	tabOnce  sync.Once
)

// Get returns the shared browser tab context. The allocator (Chrome process)
// and a single tab are created on first call; subsequent calls return the
// same tab context.
//
// The profile directory is .harness/chrome-profile — Chrome stores cookies
// and session data there, so Moodle (and other sites) stay logged in between
// CLI restarts.
func Get() context.Context {
	once.Do(func() {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserDataDir(".harness/chrome-profile"),
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
			chromedp.Flag("start-maximized", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
		)
		allocCtx, cancel = chromedp.NewExecAllocator(context.Background(), opts...)
	})
	tabOnce.Do(func() {
		tabCtx, _ = chromedp.NewContext(allocCtx)
	})
	return tabCtx
}

// Close shuts down the Chrome process. Call this at program exit.
func Close() {
	if cancel != nil {
		cancel()
	}
}
