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
		Description: "List all courses for the logged-in user by navigating to /my/courses.php. Returns a JSON array with id, name, and url for each course.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site, e.g. https://moodle.example.com",
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
	coursesURL := in.BaseURL + "/my/courses.php"

	// Extract every distinct course link on the page.
	// Works on Moodle 3.x, 4.x and any theme because it targets the stable
	// href pattern (/course/view.php?id=N) rather than theme-specific classes.
	const extractCourses = `
(function() {
  const seen = new Set();
  const results = [];
  document.querySelectorAll('a[href*="/course/view.php"]').forEach(a => {
    try {
      const u = new URL(a.href);
      const id = u.searchParams.get('id');
      if (!id || seen.has(id)) return;
      seen.add(id);

      // Try to find a meaningful name from the nearest card/box ancestor
      let name = '';
      const card = a.closest(
        '[data-courseid],[data-course-id],.card,.coursebox,.course-card,.dashboard-card'
      );
      if (card) {
        const title = card.querySelector(
          'h4,h3,h2,.coursename,.card-title,.dashboard-card-title,[data-region="course-name"]'
        );
        name = (title || a).textContent.trim().replace(/\s+/g, ' ');
      } else {
        name = a.textContent.trim().replace(/\s+/g, ' ');
      }
      if (name) results.push({ id, name, url: a.href });
    } catch(_) {}
  });
  return results;
})()
`

	ctx, cancel := context.WithTimeout(browser.Get(), 60*time.Second)
	defer cancel()

	var result any
	err := chromedp.Run(ctx,
		chromedp.Navigate(coursesURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // let JS render the course cards
		chromedp.Evaluate(extractCourses, &result),
	)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	out := string(b)
	if out == "null" || out == "[]" {
		return "no courses found — make sure you are logged in and the page loaded correctly", true
	}
	return out, false
}
