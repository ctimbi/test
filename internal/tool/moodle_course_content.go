package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/ctimbi/test/internal/api"
	"github.com/ctimbi/test/internal/browser"
)

type MoodleCourseContentTool struct{}

func init() { Default.Register(&MoodleCourseContentTool{}) }

func (MoodleCourseContentTool) Definition() api.ToolDef {
	return api.ToolDef{
		Name:        "moodle_course_content",
		Description: "Return the section structure and activity list for a Moodle course. Navigate to the course first or pass its URL directly.",
		InputSchema: map[string]any{
			"course_url": map[string]any{
				"type":        "string",
				"description": "Full URL of the course page, e.g. https://moodle.example.com/course/view.php?id=42",
			},
		},
		Required: []string{"course_url"},
	}
}

func (MoodleCourseContentTool) Execute(_ context.Context, rawInput string) (string, bool) {
	var in struct {
		CourseURL string `json:"course_url"`
	}
	if err := json.Unmarshal([]byte(rawInput), &in); err != nil {
		return fmt.Sprintf("invalid input: %v", err), true
	}

	ctx, cancel := context.WithTimeout(browser.Get(), 30*time.Second)
	defer cancel()

	const extractContent = `
(function() {
  const sections = Array.from(document.querySelectorAll(
    '.course-content .section.main, .course-content li[id^="section-"]'
  ));
  return sections.map(sec => {
    const activities = Array.from(sec.querySelectorAll('.activity')).map(act => {
      const link = act.querySelector('a');
      const typeMatch = act.className.match(/modtype_(\w+)/);
      return {
        id: act.id,
        type: act.dataset.type || (typeMatch ? typeMatch[1] : null),
        name: (act.querySelector('.instancename, .activityname') || link)?.textContent?.trim()?.replace(/\s+/g, ' '),
        url: link?.href || null,
        completion: act.querySelector('[data-completion-state]')?.dataset?.completionState || null,
      };
    });
    return {
      id: sec.id,
      name: sec.querySelector('.sectionname, .section-title')?.textContent?.trim() || null,
      activities,
    };
  }).filter(s => s.activities.length > 0 || s.name);
})()
`

	err := chromedp.Run(ctx,
		chromedp.Navigate(in.CourseURL),
		chromedp.WaitReady(".course-content, #page-content", chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	var result any
	if err := chromedp.Run(ctx, chromedp.Evaluate(extractContent, &result)); err != nil {
		return fmt.Sprintf("extract error: %v", err), true
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b), false
}
