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

type MoodleListCoursesTool struct{}

func init() { Default.Register(&MoodleListCoursesTool{}) }

func (MoodleListCoursesTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "moodle_list_courses",
		Description: "List all courses visible on the Moodle dashboard for the logged-in user. Returns a JSON array with id, name, and url for each course.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site.",
			},
		},
		Required: []string{"base_url"},
	}
}

func (MoodleListCoursesTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	in.BaseURL = strings.TrimRight(in.BaseURL, "/")

	ctx, cancel := context.WithTimeout(browser.Get(), 30*time.Second)
	defer cancel()

	const extractCourses = `
(function() {
  // Try multiple selectors used across Moodle themes
  const selectors = [
    '[data-courseid]',
    '.courseoverview-course',
    '.coursebox',
    '.dashboard-card',
  ];
  let items = [];
  for (const sel of selectors) {
    items = Array.from(document.querySelectorAll(sel));
    if (items.length > 0) break;
  }
  return items.map(el => {
    const link = el.querySelector('a[href*="/course/view.php"]') || el.querySelector('a');
    return {
      id: el.dataset.courseid || null,
      name: (
        el.querySelector('.media-body h4, h4.media-heading, .course-info-container h4, .coursename, .dashboard-card-title')
        || link
      )?.textContent?.trim(),
      url: link?.href || null,
    };
  }).filter(c => c.name);
})()
`

	err := chromedp.Run(ctx,
		chromedp.Navigate(in.BaseURL+"/my/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	var result any
	if err := chromedp.Run(ctx, chromedp.Evaluate(extractCourses, &result)); err != nil {
		return fmt.Sprintf("extract error: %v", err), true
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), false
}
