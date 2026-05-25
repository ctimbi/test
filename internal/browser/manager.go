// Package browser manages a singleton Chrome instance shared across all
// browser_* and moodle_* tool calls. The browser runs in headed (visible)
// mode so the user can watch and interact with it. Session data (cookies,
// localStorage) persists between CLI restarts via --user-data-dir.
package browser

import (
	"context"
	"os"
	"sync"

	"github.com/chromedp/chromedp"
)

var (
	allocOnce sync.Once
	allocCtx  context.Context
	allocStop context.CancelFunc

	tabMu  sync.Mutex
	tabCtx context.Context
)

// mergedCtx provides chromedp's target values (from the persistent tabCtx)
// but uses context.Background() as its parent so that tool-level timeouts
// created via context.WithTimeout(browser.Get(), d) never propagate
// cancellation back to tabCtx and never close the Chrome tab.
type mergedCtx struct {
	context.Context        // deadline / Done from caller (Background = never closes)
	vals            context.Context // chromedp target values from tabCtx
}

func (c mergedCtx) Value(key any) any { return c.vals.Value(key) }

func ensureAlloc() {
	allocOnce.Do(func() {
		_ = os.MkdirAll(".harness/chrome-profile", 0o755)
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserDataDir(".harness/chrome-profile"),
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
			chromedp.Flag("start-maximized", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.Flag("disable-infobars", true),
		)
		allocCtx, allocStop = chromedp.NewExecAllocator(context.Background(), opts...)
	})
}

func ensureTab() {
	tabMu.Lock()
	defer tabMu.Unlock()
	if tabCtx == nil || tabCtx.Err() != nil {
		ensureAlloc()
		tabCtx, _ = chromedp.NewContext(allocCtx)
		// Warm up: establish the CDP connection so Chrome is ready.
		_ = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return nil
		}))
	}
}

// Get returns a "view" of the persistent tab. Tools call
// context.WithTimeout(browser.Get(), d) to bound their actions — when
// the timeout fires the tab is NOT closed; only the current action aborts.
func Get() context.Context {
	ensureTab()
	return mergedCtx{Context: context.Background(), vals: tabCtx}
}

// Reset discards the current tab and opens a fresh one in the same Chrome
// window. Call this after an unrecoverable navigation error.
func Reset() {
	tabMu.Lock()
	tabCtx = nil
	tabMu.Unlock()
	ensureTab()
}

// Close shuts down the Chrome process. Call this at program exit.
func Close() {
	if allocStop != nil {
		allocStop()
	}
}
