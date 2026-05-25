package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ctimbi/test/internal/api"
)

// CourseNode is a Moodle course entry shown in the sidebar tree.
type CourseNode struct {
	ID   string
	Name string
	URL  string
}

// ideSession is one chat tab: a title and a scrollable output view.
type ideSession struct {
	title  string
	output *tview.TextView // receives agent/tool output
}

// IDE is the VS Code–style terminal UI (sidebar + tabbed sessions).
type IDE struct {
	app    *tview.Application
	root   *tview.Flex  // full-screen root
	pages  *tview.Pages // one page per session
	tabBar *tview.TextView
	input  *tview.InputField
	status *tview.TextView

	courseTree *tview.TreeView
	fileTree   *tview.TreeView

	sessions []*ideSession
	active   int
	mu       sync.Mutex

	running  int32 // atomic: 1 while agent turn is in progress
	runner   AgentRunner
	usageFunc UsageFunc

	// MCPStatus carries transient messages shown in status bar.
	mcpStatus string
}

// NewIDE constructs the IDE. Call Run() to start the event loop.
func NewIDE(runner AgentRunner, usageFunc UsageFunc) *IDE {
	ide := &IDE{
		app:       tview.NewApplication(),
		pages:     tview.NewPages(),
		runner:    runner,
		usageFunc: usageFunc,
	}
	ide.build()
	return ide
}

// ── layout ────────────────────────────────────────────────────────────────

func (ide *IDE) build() {
	ide.tabBar = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)
	ide.tabBar.SetBackgroundColor(tcell.ColorDarkSlateGray)

	ide.status = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	ide.status.SetBackgroundColor(tcell.ColorDarkSlateGray)

	header := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ide.tabBar, 0, 1, false).
		AddItem(ide.status, 28, 0, false)

	ide.input = tview.NewInputField().
		SetLabel(" > ").
		SetLabelColor(tcell.ColorAqua).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite)
	ide.input.SetBorder(true).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitle(" Mensaje ").
		SetTitleColor(tcell.ColorAqua)

	ide.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			ide.submitInput()
		}
	})

	mainArea := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(ide.pages, 0, 1, false).
		AddItem(ide.input, 3, 0, true)

	ide.courseTree = ide.buildCourseTree()
	ide.fileTree = ide.buildFileTree(".")

	coursesBox := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(label("● ASIGNATURAS", tcell.ColorYellow), 1, 0, false).
		AddItem(ide.courseTree, 0, 1, false)

	filesBox := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(label("● ARCHIVOS", tcell.ColorYellow), 1, 0, false).
		AddItem(ide.fileTree, 0, 1, false)

	sidebar := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(coursesBox, 0, 1, false).
		AddItem(filesBox, 0, 1, false)
	sidebar.SetBorder(true).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitle(" Moodle Agent ").
		SetTitleColor(tcell.ColorAqua)

	ide.root = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(sidebar, 24, 0, false).
		AddItem(mainArea, 0, 1, true)

	// Global key bindings.
	ide.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			ide.newSession("Sesión")
			return nil
		case tcell.KeyCtrlW:
			ide.closeActiveSession()
			return nil
		case tcell.KeyCtrlL:
			ide.app.SetFocus(ide.input)
			return nil
		case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5:
			idx := int(event.Key()-tcell.KeyF1)
			ide.mu.Lock()
			n := len(ide.sessions)
			ide.mu.Unlock()
			if idx < n {
				ide.switchSession(idx)
			}
			return nil
		}
		return event
	})

	// Open the first session.
	ide.newSession("Sesión 1")
	ide.refreshStatus()
}

// label returns a non-focusable single-line text view used as a section header.
func label(text string, color tcell.Color) *tview.TextView {
	tv := tview.NewTextView().SetText(" " + text)
	tv.SetTextColor(color).SetBackgroundColor(tcell.ColorBlack)
	return tv
}

// ── sessions ──────────────────────────────────────────────────────────────

func (ide *IDE) newSession(title string) {
	ide.mu.Lock()
	defer ide.mu.Unlock()

	sess := &ideSession{title: title}

	sess.output = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetChangedFunc(func() { ide.app.Draw() })
	sess.output.SetBorder(false).
		SetBackgroundColor(tcell.ColorBlack)

	key := fmt.Sprintf("session-%d", len(ide.sessions))
	ide.sessions = append(ide.sessions, sess)
	ide.pages.AddPage(key, sess.output, true, false)

	idx := len(ide.sessions) - 1
	ide.active = idx
	ide.pages.SwitchToPage(key)
	ide.renderTabBar()
}

func (ide *IDE) switchSession(idx int) {
	ide.mu.Lock()
	defer ide.mu.Unlock()
	if idx < 0 || idx >= len(ide.sessions) {
		return
	}
	ide.active = idx
	key := fmt.Sprintf("session-%d", idx)
	ide.pages.SwitchToPage(key)
	ide.renderTabBar()
}

func (ide *IDE) closeActiveSession() {
	ide.mu.Lock()
	defer ide.mu.Unlock()
	if len(ide.sessions) <= 1 {
		return
	}
	key := fmt.Sprintf("session-%d", ide.active)
	ide.pages.RemovePage(key)
	ide.sessions = append(ide.sessions[:ide.active], ide.sessions[ide.active+1:]...)
	if ide.active >= len(ide.sessions) {
		ide.active = len(ide.sessions) - 1
	}
	// Rebuild page keys to match indices.
	ide.pages.AddPage(fmt.Sprintf("session-%d", ide.active), ide.sessions[ide.active].output, true, true)
	ide.renderTabBar()
}

func (ide *IDE) renderTabBar() {
	var b strings.Builder
	for i, s := range ide.sessions {
		if i == ide.active {
			fmt.Fprintf(&b, `["%d"][black:aqua:b] ● %s [-:-:-][""]  `, i, s.title)
		} else {
			fmt.Fprintf(&b, `["%d"][white:darkslategray] ○ %s [-]  `, i, s.title)
		}
	}
	b.WriteString(`[aqua:darkslategray] [+] [-]`)
	ide.tabBar.SetText(b.String())
	// clicking on highlighted regions switches session
	ide.tabBar.SetHighlightedFunc(func(added, removed, remaining []string) {
		if len(added) > 0 {
			var idx int
			fmt.Sscanf(added[0], "%d", &idx)
			go func() {
				ide.app.QueueUpdateDraw(func() { ide.switchSession(idx) })
			}()
		}
	})
}

// ── input submission ──────────────────────────────────────────────────────

func (ide *IDE) submitInput() {
	text := strings.TrimSpace(ide.input.GetText())
	if text == "" {
		return
	}
	if !atomic.CompareAndSwapInt32(&ide.running, 0, 1) {
		return // already busy
	}
	ide.input.SetText("")

	// Echo user message to active session.
	ide.mu.Lock()
	sess := ide.sessions[ide.active]
	ide.mu.Unlock()
	fmt.Fprintf(tview.ANSIWriter(sess.output), "\n\x1b[1;36mTú:\x1b[0m %s\n\n", text)

	ide.status.SetText("[yellow]⏳ pensando…[-]")

	go func() {
		err := ide.runner(context.Background(), text)
		atomic.StoreInt32(&ide.running, 0)
		ide.app.QueueUpdateDraw(func() {
			if err != nil {
				fmt.Fprintf(tview.ANSIWriter(sess.output), "\x1b[31m[error] %v\x1b[0m\n", err)
			}
			ide.refreshStatus()
			ide.app.SetFocus(ide.input)
		})
	}()
}

// ── output from agent/tools ───────────────────────────────────────────────

// Append writes text (from the stdout pipe) to the active session's output.
func (ide *IDE) Append(text string) {
	ide.mu.Lock()
	sess := ide.sessions[ide.active]
	ide.mu.Unlock()
	ide.app.QueueUpdateDraw(func() {
		fmt.Fprint(tview.ANSIWriter(sess.output), text)
	})
}

// Confirm shows a modal dialog asking for y/n approval. Blocks until answered.
func (ide *IDE) Confirm(prompt, detail string) bool {
	reply := make(chan bool, 1)
	ide.app.QueueUpdateDraw(func() {
		body := prompt
		if detail != "" {
			lines := strings.Split(detail, "\n")
			if len(lines) > 6 {
				lines = lines[:6]
			}
			body += "\n\n" + strings.Join(lines, "\n")
			if len(strings.Split(detail, "\n")) > 6 {
				body += "\n…"
			}
		}
		modal := tview.NewModal().
			SetText(body).
			AddButtons([]string{"Sí (y)", "No (n)"}).
			SetDoneFunc(func(_ int, label string) {
				reply <- strings.HasPrefix(label, "Sí")
				ide.app.SetRoot(ide.root, true)
				ide.app.SetFocus(ide.input)
			})
		ide.app.SetRoot(modal, false)
	})
	return <-reply
}

// SetMCPStatus updates the transient MCP loading message in the status bar.
func (ide *IDE) SetMCPStatus(text string) {
	ide.app.QueueUpdateDraw(func() {
		ide.mcpStatus = text
		ide.refreshStatus()
	})
}

func (ide *IDE) refreshStatus() {
	if ide.mcpStatus != "" {
		ide.status.SetText("[gray]" + ide.mcpStatus + "[-]")
		return
	}
	if ide.usageFunc == nil {
		ide.status.SetText("")
		return
	}
	usage, cost := ide.usageFunc()
	if cost < 0 {
		ide.status.SetText(fmt.Sprintf("[gray]in:%d out:%d[-]", usage.InputTokens, usage.OutputTokens))
	} else {
		ide.status.SetText(fmt.Sprintf("[gray]in:%d out:%d $%.4f[-]", usage.InputTokens, usage.OutputTokens, cost))
	}
}

// SetCourses populates the course tree after moodle_list_courses runs.
func (ide *IDE) SetCourses(courses []CourseNode) {
	ide.app.QueueUpdateDraw(func() {
		root := ide.courseTree.GetRoot()
		root.ClearChildren()
		if len(courses) == 0 {
			root.AddChild(tview.NewTreeNode("(sin cursos)").SetColor(tcell.ColorGray))
			return
		}
		for _, c := range courses {
			c := c
			node := tview.NewTreeNode(c.Name).
				SetColor(tcell.ColorAqua).
				SetReference(&c)
			root.AddChild(node)
		}
	})
}

// ── trees ─────────────────────────────────────────────────────────────────

func (ide *IDE) buildCourseTree() *tview.TreeView {
	root := tview.NewTreeNode("Moodle").SetColor(tcell.ColorYellow)
	placeholder := tview.NewTreeNode("(login para cargar)").SetColor(tcell.ColorGray)
	root.AddChild(placeholder)

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root).
		SetTopLevel(1)
	tree.SetBackgroundColor(tcell.ColorBlack)
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		if c, ok := ref.(*CourseNode); ok {
			ide.newSession(c.Name)
			intro := fmt.Sprintf("Hola. Hablemos sobre el curso: %s", c.Name)
			ide.input.SetText(intro)
			ide.app.SetFocus(ide.input)
		}
	})
	return tree
}

func (ide *IDE) buildFileTree(root string) *tview.TreeView {
	rootNode := tview.NewTreeNode(root).
		SetColor(tcell.ColorYellow).
		SetReference(root)
	ide.addFileChildren(rootNode, root)

	tree := tview.NewTreeView().
		SetRoot(rootNode).
		SetCurrentNode(rootNode).
		SetTopLevel(1)
	tree.SetBackgroundColor(tcell.ColorBlack)
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		ref := node.GetReference()
		if ref == nil {
			return
		}
		path := ref.(string)
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.IsDir() {
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				node.ClearChildren()
				ide.addFileChildren(node, path)
				node.SetExpanded(true)
			}
		} else {
			// Open file: pre-fill input with a read request.
			ide.input.SetText("lee el archivo " + path)
			ide.app.SetFocus(ide.input)
		}
	})
	return tree
}

func (ide *IDE) addFileChildren(parent *tview.TreeNode, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	skip := map[string]bool{".git": true, "vendor": true}
	for _, e := range entries {
		name := e.Name()
		if skip[name] {
			continue
		}
		full := filepath.Join(dir, name)
		node := tview.NewTreeNode(name).SetReference(full)
		if e.IsDir() {
			node.SetColor(tcell.ColorBlue)
			// A dummy child makes the expand triangle appear; cleared on actual expand.
			node.AddChild(tview.NewTreeNode("…").SetColor(tcell.ColorGray))
		} else {
			node.SetColor(tcell.ColorWhite)
			node.SetIndent(1)
		}
		parent.AddChild(node)
	}
}

// ── run ───────────────────────────────────────────────────────────────────

// Run starts the tview event loop (blocks until quit).
func (ide *IDE) Run() error {
	return ide.app.SetRoot(ide.root, true).EnableMouse(true).Run()
}

// Stop shuts down the tview application (called on /exit).
func (ide *IDE) Stop() { ide.app.Stop() }

// keep api import used
var _ api.Usage
