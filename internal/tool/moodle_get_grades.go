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
		Description: "Fetch the grade report for the logged-in student in a given course. Returns a JSON array of {item, grade, feedback} objects.",
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

	ctx, cancel := context.WithTimeout(browser.Get(), 30*time.Second)
	defer cancel()

	const extractGrades = `
(function() {
  const rows = Array.from(document.querySelectorAll('table.user-grade tr'));
  return rows.slice(1).map(row => {
    const cells = row.querySelectorAll('td, th');
    return {
      item: cells[0]?.textContent?.trim(),
      grade: cells[1]?.textContent?.trim(),
      range: cells[2]?.textContent?.trim(),
      percentage: cells[3]?.textContent?.trim(),
      feedback: cells[4]?.textContent?.trim(),
    };
  }).filter(r => r.item);
})()
`

	err := chromedp.Run(ctx,
		chromedp.Navigate(gradesURL),
		chromedp.WaitReady("table.user-grade, #page-grade-report-user-index", chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	var result any
	if err := chromedp.Run(ctx, chromedp.Evaluate(extractGrades, &result)); err != nil {
		return fmt.Sprintf("extract error: %v", err), true
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), false
}
