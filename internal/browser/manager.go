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
	context.Context        // deadline / Done from the caller (Background = never)
	vals            context.Context // chromedp target values from tabCtx
}

func (c mergedCtx) Value(key any) any { return c.vals.Value(key) }

func ensureAlloc() {
	allocOnce.Do(func() {
		if err := os.MkdirAll(".harness/chrome-profile", 0o755); err == nil {
			// best-effort
		}
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
		// Warm up the tab so Chrome is visible immediately.
		_ = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return nil
		}))
	}
}

// Get returns a "view" of the persistent tab.  Tools should call
// context.WithTimeout(browser.Get(), d) to bound their operations — when
// that timeout fires the tab is NOT closed; only the tool's action is
// aborted.
func Get() context.Context {
	ensureTab()
	// Return a merged context whose Done/Deadline come from Background
	// (i.e. never cancel on their own) but whose Value() chain reaches
	// through to tabCtx so chromedp can locate the browser target.
	return mergedCtx{Context: context.Background(), vals: tabCtx}
}

// Reset closes the current tab and opens a fresh one.  Call this when the
// tab is in an unrecoverable state (e.g. after a hard navigation abort).
func Reset() {
	tabMu.Lock()
	defer tabMu.Unlock()
	tabCtx = nil // next ensureTab() call will recreate
	tabMu.Unlock()
	ensureTab()
	tabMu.Lock()
}

// Close shuts down the Chrome process.  Call this at program exit.
func Close() {
	if allocStop != nil {
		allocStop()
	}
}
