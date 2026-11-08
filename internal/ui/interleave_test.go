package ui

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// interleaveSeeds are the seeds every terminal size is driven with: fixed, so a failure names a
// seed that replays it, and few, so the whole test stays within seconds on a Raspberry Pi.
var interleaveSeeds = []uint64{1, 2, 3, 4, 5}

// interleaveSteps is how many steps one seed drives before the pool is drained: each step either
// hands the model a random input or delivers one held command or message.
const interleaveSteps = 200

// interleaveDrainLimit bounds the deliveries a drain makes, so a command chain that never ends
// fails the test rather than hanging it.
const interleaveDrainLimit = 10000

// interleaveSizes are the terminal sizes driven: split and stacked at 120 columns, and at 60 the
// stacked layout and the one-pane layout, where the sidebar fills the screen until Enter.
var interleaveSizes = []struct {
	width  int
	height int
}{
	{120, 40},
	{120, 12},
	{60, 40},
	{60, 12},
}

// interleaveBetaOpen and interleaveBetaDone are beta's frontmatter fields while it is open and
// once an edit closed it; an edit swaps one for the other, so beta moves between the Ready and the
// Closed tab.
const (
	interleaveBetaOpen = "title: Beta\nstatus: open\npriority: 2\n"
	interleaveBetaDone = "title: Beta\nstatus: done\npriority: 2\nclosed: 2026-10-12T09:00:00Z\nreason: shipped\n"
)

// interleaveBody returns the body of an issue or Note: its marker line, which names the kind and
// the id so a check can tell which of the two the pane shows, a line of links, and enough items
// that the pane scrolls on a short terminal.
func interleaveBody(kind, id string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "marker-%s-%s\n\nSee `alpha`, `beta` and `shared`.\n\n", kind, id)
	for item := range 20 {
		fmt.Fprintf(&body, "- item %d of %s\n", item+1, id)
	}
	return body.String()
}

// interleaveFiles returns the store the interleavings run over: issues waiting on one another, an
// epic with a child, a closed issue, and an issue and a Note that share the id shared.
func interleaveFiles() map[string]string {
	return map[string]string{
		"issues/alpha.md":      issueFile("title: Alpha\nstatus: open\npriority: 1\ndeps: [beta]\n", interleaveBody("issue", "alpha")),
		"issues/beta.md":       issueFile(interleaveBetaOpen, interleaveBody("issue", "beta")),
		"issues/epic.md":       issueFile("title: Epic\nstatus: open\npriority: 3\n", interleaveBody("issue", "epic")),
		"issues/epic/child.md": issueFile("title: Child\nstatus: open\npriority: 2\n", interleaveBody("issue", "child")),
		"issues/finished.md": issueFile(
			"title: Finished\nstatus: done\npriority: 2\nclosed: 2026-10-12T09:00:00Z\nreason: shipped\n",
			interleaveBody("issue", "finished"),
		),
		"issues/shared.md":    issueFile("title: Shared issue\nstatus: open\npriority: 0\n", interleaveBody("issue", "shared")),
		"knowledge/shared.md": noteFile("title: Shared Note\n", interleaveBody("note", "shared")),
		"knowledge/guide.md":  noteFile("title: Guide\n", interleaveBody("note", "guide")),
	}
}

// interleaveKeys are the keys a step presses: moves, tree folds, tabs, the dependency view, Esc,
// Enter, help and the layout toggle. No key writes, edits or quits.
var interleaveKeys = []struct {
	name  string
	press tea.KeyPressMsg
}{
	{"up", specialKey(tea.KeyUp)},
	{"down", specialKey(tea.KeyDown)},
	{"left", specialKey(tea.KeyLeft)},
	{"right", specialKey(tea.KeyRight)},
	{"j", runeKey('j')},
	{"k", runeKey('k')},
	{"h", runeKey('h')},
	{"l", runeKey('l')},
	{"tab", specialKey(tea.KeyTab)},
	{"1", runeKey('1')},
	{"2", runeKey('2')},
	{"3", runeKey('3')},
	{"4", runeKey('4')},
	{"d", runeKey('d')},
	{"esc", specialKey(tea.KeyEscape)},
	{"enter", specialKey(tea.KeyEnter)},
	{"?", runeKey('?')},
	{"v", runeKey('v')},
}

// pooled is one entry of an interleaving's pool: a command not yet run, or the message a run
// command returned, not yet delivered.
type pooled struct {
	cmd tea.Cmd
	msg tea.Msg
}

// interleaving drives one model with random inputs while every command it returns waits in a pool
// and runs, and every message those commands return is delivered, in random order, as a program
// whose reads finish out of order would see them.
type interleaving struct {
	t      *testing.T
	random *rand.Rand
	ts     *testStore
	m      model
	pool   []pooled
	// files is each store file's content as last written, keyed as newTestStore keys them.
	files map[string]string
	edits int
	// label names the seed and the size, and steps the steps taken so far, which a failure prints.
	label string
	steps []string
}

// TestInterleaveDetailPane drives each seed at each size through random keys, clicks and wheel
// notches, polls and on-disk edits, with commands and their messages delivered in random order.
// After every step a selection whose pane shows nothing has its read pending, and one it shows is
// shown as the right kind; after the pool is drained the pane shows the selection, the right kind of it, and the frame draws it.
func TestInterleaveDetailPane(t *testing.T) {
	for _, size := range interleaveSizes {
		for _, seed := range interleaveSeeds {
			name := fmt.Sprintf("%dx%d/seed-%d", size.width, size.height, seed)
			t.Run(name, func(t *testing.T) {
				files := interleaveFiles()
				ts := newTestStore(t, files)
				run := &interleaving{
					t:      t,
					random: rand.New(rand.NewPCG(seed, seed)),
					ts:     ts,
					m:      newTestModel(t, ts, size.width, size.height),
					files:  files,
					label:  name,
				}
				run.m.detailTick = immediateDetailTick
				for range interleaveSteps {
					run.step()
				}
				run.drain()
				run.checkSettled()
			})
		}
	}
}

// immediateDetailTick stands in for tea.Tick: its message is ready at once, so the checks on a
// pending detail read join the pool with the reads themselves.
func immediateDetailTick(_ time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return fn(time.Now())
	}
}

// step takes one step: half the time, while the pool holds anything, it delivers one entry of the
// pool; otherwise it hands the model a random input.
func (r *interleaving) step() {
	if len(r.pool) > 0 && r.random.IntN(2) == 0 {
		r.deliver()
		return
	}
	switch choice := r.random.IntN(10); {
	case choice < 5:
		key := interleaveKeys[r.random.IntN(len(interleaveKeys))]
		r.update("key "+key.name, key.press)
	case choice < 7:
		x, y := r.random.IntN(r.m.width), r.random.IntN(r.m.height)
		r.update(fmt.Sprintf("click %d,%d", x, y), click(x, y))
		r.update(fmt.Sprintf("release %d,%d", x, y), tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	case choice < 8:
		x, y := r.random.IntN(r.m.width), r.random.IntN(r.m.height)
		button := tea.MouseWheelDown
		if r.random.IntN(2) == 0 {
			button = tea.MouseWheelUp
		}
		r.update(fmt.Sprintf("wheel %v %d,%d", button, x, y), wheel(button, x, y))
	case choice < 9:
		r.update("tick", tickMsg{})
	default:
		r.edit()
	}
}

// update hands msg to the model's Update, records the step as what, pools the command it returns
// and checks the pending read.
func (r *interleaving) update(what string, msg tea.Msg) {
	r.t.Helper()
	r.steps = append(r.steps, what)
	updated, cmd := r.m.Update(msg)
	next, isModel := updated.(model)
	if !isModel {
		r.fail("Update returned %T, want model", updated)
	}
	r.m = next
	if cmd != nil {
		r.pool = append(r.pool, pooled{cmd: cmd})
	}
	r.checkStep()
}

// deliver takes one random entry out of the pool: a command runs, its batch's commands each
// joining the pool and any other message joining it to be delivered later; a message is handed to
// the model.
func (r *interleaving) deliver() {
	r.t.Helper()
	at := r.random.IntN(len(r.pool))
	entry := r.pool[at]
	r.pool = append(r.pool[:at], r.pool[at+1:]...)
	if entry.cmd == nil {
		r.update(fmt.Sprintf("deliver %T", entry.msg), entry.msg)
		return
	}
	switch msg := entry.cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, batched := range msg {
			if batched != nil {
				r.pool = append(r.pool, pooled{cmd: batched})
			}
		}
	default:
		r.pool = append(r.pool, pooled{msg: msg})
	}
}

// drain delivers the pool's entries in random order until it is empty.
func (r *interleaving) drain() {
	r.t.Helper()
	for deliveries := 0; len(r.pool) > 0; deliveries++ {
		if deliveries == interleaveDrainLimit {
			r.fail("the pool still holds %d entries after %d deliveries", len(r.pool), deliveries)
		}
		r.deliver()
	}
}

// edit changes one store file on disk: it closes beta while it is open and opens it again once
// closed, or appends a line to the body of another issue or Note. Its marker line stays.
func (r *interleaving) edit() {
	r.t.Helper()
	r.edits++
	names := []string{"issues/beta.md", "issues/alpha.md", "issues/epic/child.md", "issues/shared.md", "knowledge/shared.md"}
	name := names[r.random.IntN(len(names))]
	content := r.files[name]
	switch {
	case name != "issues/beta.md":
		content += fmt.Sprintf("- edit %d\n", r.edits)
	case strings.Contains(content, interleaveBetaOpen):
		content = strings.Replace(content, interleaveBetaOpen, interleaveBetaDone, 1)
	default:
		content = strings.Replace(content, interleaveBetaDone, interleaveBetaOpen, 1)
	}
	r.files[name] = content
	path := filepath.Join(r.ts.store.Root, filepath.FromSlash(name))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.fail("write %s: %v", name, err)
	}
	r.steps = append(r.steps, "edit "+name)
}

// checkStep checks the detail pane after one step. A selection the pane shows nothing for, no
// issue nor Note, has its read pending, of the right kind, and the pane says it is loading while
// the dependency view does not cover it. A selection the pane shows with no read of it pending is
// shown as the kind the tab lists, so a Note never stands in for the issue sharing its id.
func (r *interleaving) checkStep() {
	r.t.Helper()
	id, isSelected := r.m.sidebarSelection()
	if !isSelected {
		return
	}
	pending := r.m.detailPending
	isNote := r.m.tab == TabKnowledge
	isPendingSelection := pending.isPending() && pending.id == id && pending.isNote == isNote
	switch {
	case r.m.issue.id == "" && !isPendingSelection:
		r.fail("the pane is empty with %q (note %v) selected, pending read %+v", id, isNote, pending)
	case r.m.issue.id == "" && !r.m.deps.isOpen && len(r.m.detail) == 0:
		r.fail("the pane draws nothing while the read of %q is pending", id)
	case r.m.issue.id == id && !isPendingSelection:
		r.checkMarker(id)
	}
}

// checkMarker checks that the pane shows id as the kind the open tab lists: its marker line names
// a Note on the Knowledge tab and an issue elsewhere.
func (r *interleaving) checkMarker(id string) {
	r.t.Helper()
	kind := "issue"
	if r.m.tab == TabKnowledge {
		kind = "note"
	}
	marker := fmt.Sprintf("marker-%s-%s", kind, id)
	if !strings.Contains(r.m.issue.source, marker) {
		r.fail("the pane shows %q without its marker %s:\n%s", id, marker, r.m.issue.source)
	}
}

// checkSettled checks the model once the pool is drained: no read pending, the pane showing the
// selection, or nothing without one, its marker naming the kind the tab shows, and, where the
// detail pane is on screen and neither help nor the dependency view covers it, the frame drawing
// the pane's rows.
func (r *interleaving) checkSettled() {
	r.t.Helper()
	if r.m.detailPending.isPending() {
		r.fail("a read is still pending after the drain: %+v", r.m.detailPending)
	}
	id, isSelected := r.m.sidebarSelection()
	if !isSelected {
		if r.m.issue.id != "" {
			r.fail("the pane shows %q with nothing selected", r.m.issue.id)
		}
		return
	}
	if r.m.issue.id != id {
		r.fail("the pane shows %q with %q selected", r.m.issue.id, id)
	}
	r.checkMarker(id)
	isDetailOnScreen := !r.m.isSinglePane() || r.m.isDetailOpen
	if !isDetailOnScreen || r.m.isHelpOpen || r.m.deps.isOpen {
		return
	}
	frame := ansi.Strip(r.m.View().Content)
	rows := paneRows(r.m.detail, r.m.detailOffset, r.m.detailPaneRows(), r.m.detailWidth())
	isDrawn := false
	for _, row := range rows {
		text := strings.TrimRight(ansi.Strip(row), " ")
		if text == "" {
			continue
		}
		isDrawn = true
		if !strings.Contains(frame, text) {
			r.fail("the frame lacks the detail row %q of %q:\n%s", text, id, frame)
		}
	}
	if !isDrawn {
		r.fail("the detail pane of %q draws no text:\n%s", id, frame)
	}
}

// fail ends the test with the seed, the size and every step taken.
func (r *interleaving) fail(format string, args ...any) {
	r.t.Helper()
	r.t.Fatalf("%s: %s\nsteps:\n  %s", r.label, fmt.Sprintf(format, args...), strings.Join(r.steps, "\n  "))
}
