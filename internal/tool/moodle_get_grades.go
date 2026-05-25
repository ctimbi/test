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

type MoodleGetGradesTool struct{}

func init() { Default.Register(&MoodleGetGradesTool{}) }

func (MoodleGetGradesTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "moodle_get_grades",
		Description: "Fetch the grade report for the logged-in student in a course. Returns a JSON array of grade items.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site.",
			},
			"course_id": map[string]any{
				"type":        "integer",
				"description": "Numeric Moodle course ID.",
			},
		},
		Required: []string{"base_url", "course_id"},
	}
}

func (MoodleGetGradesTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		BaseURL  string `json:"base_url"`
		CourseID int    `json:"course_id"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}
	in.BaseURL = strings.TrimRight(in.BaseURL, "/")
	gradesURL := fmt.Sprintf("%s/grade/report/user/index.php?id=%d", in.BaseURL, in.CourseID)

	// Works for both the classic table (table.user-grade) and the newer
	// Moodle 4.x layout which uses a different table structure.
	const extractGrades = `
(function() {
  // Try both old and new grade table selectors
  const table = document.querySelector('table.user-grade, table[id*="grade"], .gradereport-user-wrapper table');
  if (!table) return [];

  const rows = Array.from(table.querySelectorAll('tr'));
  return rows.slice(1).map(row => {
    const cells = Array.from(row.querySelectorAll('td,th'));
    if (cells.length < 2) return null;
    return {
      item:       cells[0]?.textContent?.trim()?.replace(/\s+/g, ' ') || null,
      grade:      cells[1]?.textContent?.trim() || null,
      range:      cells[2]?.textContent?.trim() || null,
      percentage: cells[3]?.textContent?.trim() || null,
      feedback:   cells[4]?.textContent?.trim() || null,
    };
  }).filter(r => r && r.item);
})()
`

	ctx, cancel := context.WithTimeout(browser.Get(), 60*time.Second)
	defer cancel()

	var result any
	err := chromedp.Run(ctx,
		chromedp.Navigate(gradesURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(extractGrades, &result),
	)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	out := string(b)
	if out == "null" || out == "[]" {
		return "no grade data found — check course_id and that you are logged in", true
	}
	return out, false
}
