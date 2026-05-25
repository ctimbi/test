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
		Description: "List all enrolled courses for the logged-in user. Navigates to /my/courses.php and calls the Moodle AJAX API using the browser session (no external token needed). Falls back to DOM scraping if the API is unavailable.",
		InputSchema: map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"description": "Root URL of the Moodle site, e.g. https://moodle.example.com",
			},
		},
		Required: []string{"base_url"},
	}
}

// apiScript calls Moodle's internal AJAX service using synchronous XHR.
// window.M.cfg is injected by every Moodle page and contains the sesskey
// needed to authenticate the request. No external token is required.
const apiScript = `
(function() {
  var cfg = window.M && window.M.cfg;
  if (!cfg || !cfg.sesskey) return null;

  var url = cfg.wwwroot + '/lib/ajax/service.php'
            + '?sesskey=' + cfg.sesskey
            + '&info=core_course_get_enrolled_courses_by_timeline_classification';

  var body = JSON.stringify([{
    index: 0,
    methodname: 'core_course_get_enrolled_courses_by_timeline_classification',
    args: {
      offset: 0,
      limit: 0,
      classification: 'all',
      sort: 'fullname',
      customfieldname: '',
      customfieldvalue: ''
    }
  }]);

  try {
    var xhr = new XMLHttpRequest();
    xhr.open('POST', url, false);                      // false = synchronous
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.send(body);

    if (xhr.status !== 200) return { error: 'HTTP ' + xhr.status };

    var resp = JSON.parse(xhr.responseText);
    if (!resp[0] || resp[0].error) return { error: JSON.stringify(resp[0]) };
    if (!resp[0].data || !resp[0].data.courses) return { error: 'no courses key in response' };

    return resp[0].data.courses.map(function(c) {
      return {
        id:       String(c.id),
        name:     c.fullname,
        short:    c.shortname,
        category: c.coursecategory || null,
        url:      cfg.wwwroot + '/course/view.php?id=' + c.id,
        progress: typeof c.progress === 'number' ? c.progress : null
      };
    });
  } catch(e) {
    return { error: e.message };
  }
})()
`

// domScript is the fallback when the API call fails.
// Uses data-course-id containers and reads the full name from
// span.multiline[title] (the visible span truncates with '…').
const domScript = `
(function() {
  var seen = {};
  var results = [];
  document.querySelectorAll('[data-region="course-content"][data-course-id]').forEach(function(card) {
    var id = card.dataset.courseId;
    if (!id || seen[id]) return;
    seen[id] = true;

    var nameEl = card.querySelector('span.multiline[title]');
    var name = nameEl ? nameEl.getAttribute('title') : null;
    if (!name) {
      var link = card.querySelector('a.coursename, a.aalink');
      name = link ? link.textContent.trim().replace(/\s+/g, ' ') : null;
    }
    var link = card.querySelector('a[href*="/course/view.php"]');
    var category = card.querySelector('.categoryname');

    if (name && link) {
      results.push({
        id:       id,
        name:     name,
        category: category ? category.textContent.trim() : null,
        url:      link.href
      });
    }
  });
  return results.length ? results : null;
})()
`

// CourseEntry is a successfully parsed Moodle course.
type CourseEntry struct {
	ID   string
	Name string
	URL  string
}

// OnCoursesLoaded is called after moodle_list_courses succeeds with at least
// one course. Wire this up in main.go to push courses into the IDE sidebar.
var OnCoursesLoaded func([]CourseEntry)

// parseCourseList extracts a []CourseEntry from the raw any value that
// chromedp.Evaluate returns (a []interface{} of map[string]interface{}).
func parseCourseList(raw any) []CourseEntry {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]CourseEntry, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		url, _ := m["url"].(string)
		if name != "" {
			out = append(out, CourseEntry{ID: id, Name: name, URL: url})
		}
	}
	return out
}

func notifyCourses(raw any) {
	if OnCoursesLoaded == nil {
		return
	}
	if entries := parseCourseList(raw); len(entries) > 0 {
		OnCoursesLoaded(entries)
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

	ctx, cancel := context.WithTimeout(browser.Get(), 60*time.Second)
	defer cancel()

	// Navigate to the courses page so window.M.cfg is available.
	err := chromedp.Run(ctx,
		chromedp.Navigate(in.BaseURL+"/my/courses.php"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		return fmt.Sprintf("navigate error: %v", err), true
	}

	// Try the AJAX API first.
	var apiResult any
	if err := chromedp.Run(ctx, chromedp.Evaluate(apiScript, &apiResult)); err == nil {
		if m, ok := apiResult.(map[string]any); ok {
			if errMsg, hasErr := m["error"]; hasErr {
				// API returned an error — log and fall through to DOM.
				_ = errMsg
			} else {
				notifyCourses(apiResult)
				b, _ := json.MarshalIndent(apiResult, "", "  ")
				return string(b), false
			}
		} else if apiResult != nil {
			notifyCourses(apiResult)
			b, _ := json.MarshalIndent(apiResult, "", "  ")
			return string(b), false
		}
	}

	// Fallback: scrape the DOM.
	var domResult any
	if err := chromedp.Run(ctx, chromedp.Evaluate(domScript, &domResult)); err != nil {
		return fmt.Sprintf("dom extract error: %v", err), true
	}
	if domResult == nil {
		return "no courses found — check that you are logged in and the page loaded correctly", true
	}
	notifyCourses(domResult)
	b, _ := json.MarshalIndent(domResult, "", "  ")
	return string(b), false
}
