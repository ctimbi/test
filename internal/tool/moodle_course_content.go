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
		Description: "Return the section and activity list for a Moodle course page. Pass the full course URL (e.g. .../course/view.php?id=42).",
		InputSchema: map[string]any{
			"course_url": map[string]any{
				"type":        "string",
				"description": "Full URL of the course page.",
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

	// Covers Moodle 3.x (.section.main) and 4.x (li[data-sectionid])
	const extractContent = `
(function() {
  // Section containers differ between Moodle versions
  const secSelectors = [
    'li[data-sectionid]',
    '.course-content .section.main',
    '.course-content li[id^="section-"]',
  ];
  let sections = [];
  for (const sel of secSelectors) {
    sections = Array.from(document.querySelectorAll(sel));
    if (sections.length) break;
  }

  return sections.map(sec => {
    const nameEl = sec.querySelector(
      '.sectionname,.section-title h3,.section-title h4,.section-title a,h3[class*="section"]'
    );
    const activities = Array.from(sec.querySelectorAll('.activity,[data-activityname]')).map(act => {
      const link = act.querySelector('a[href]');
      const typeMatch = act.className.match(/modtype_(\w+)/);
      const nameEl2 = act.querySelector('.instancename,.activityname,[data-activityname]');
      return {
        id: act.id || null,
        type: act.dataset.type || (typeMatch ? typeMatch[1] : null),
        name: (nameEl2 || link)?.textContent?.trim()?.replace(/\s+/g, ' ') || null,
        url: link?.href || null,
      };
    }).filter(a => a.name);

    return {
      id: sec.id || sec.dataset.sectionid || null,
      name: nameEl?.textContent?.trim()?.replace(/\s+/g, ' ') || null,
      activities,
    };
  }).filter(s => s.activities.length > 0 || s.name);
})()
`

	ctx, cancel := context.WithTimeout(browser.Get(), 60*time.Second)
	defer cancel()

	var result any
	err := chromedp.Run(ctx,
		chromedp.Navigate(in.CourseURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(extractContent, &result),
	)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	out := string(b)
	if out == "null" || out == "[]" {
		return "no sections found — check that the URL is a course page and you are logged in", true
	}
	return out, false
}
