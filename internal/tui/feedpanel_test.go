package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/activity"
	"github.com/sadmachine/asanamate/internal/agents"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/feed"
	"github.com/sadmachine/asanamate/internal/ticket"
)

// feedModel is agentModel with one feed, sized so the panel has a place.
func feedModel(t *testing.T) *Model {
	t.Helper()
	m := agentModel(t, config.SymbolsUnicode, true)
	m.deps.Config.Feed.Commands = []string{"ci-feed --json"}
	m.feed = newFeedPanel(m.deps.Config.Feed)
	for _, t := range m.tasks {
		m.details[t.GID] = ticket.Ticket{Task: t}
	}
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return m
}

func itemTexts(items []feedRow) []string {
	var texts []string
	for _, it := range items {
		texts = append(texts, it.Text)
	}
	return texts
}

func TestFeedOrdersUpcomingThenNewest(t *testing.T) {
	m := feedModel(t)
	at := func(d time.Duration) time.Time { return testToday.Add(d) }
	m.Update(feedMsg{reply: feed.Reply{Source: "ci", Items: []feed.Item{
		{Time: at(-2 * time.Hour), Text: "older"},
		{Time: at(2 * time.Hour), Text: "later meeting"},
		{Time: at(-time.Hour), Text: "newer"},
		{Time: at(25 * time.Minute), Text: "soon meeting"},
	}}})
	items := m.feedRows()
	if got := itemTexts(items); !slices.Equal(got, []string{"soon meeting", "later meeting", "newer", "older"}) {
		t.Fatalf("order = %q", got)
	}
	lines := strings.Split(ansi.Strip(m.feedLines(100, 10)), "\n")
	if !strings.Contains(lines[0], "Next (2)") || !strings.Contains(lines[1], "in 25m") || !strings.Contains(lines[2], "in 2h") ||
		!strings.Contains(lines[3], "Earlier (2)") || !strings.Contains(lines[4], "11:00") || !strings.HasSuffix(strings.TrimSpace(lines[4]), "ci") {
		t.Fatalf("lines = %q", lines)
	}

	m.Update(feedMsg{reply: feed.Reply{Source: "ci", Items: []feed.Item{{Time: at(-time.Hour), Text: "only past"}}}})
	if out := ansi.Strip(m.feedLines(100, 10)); strings.Contains(out, "Next") || strings.Contains(out, "Earlier") {
		t.Fatalf("headers without upcoming items: %q", out)
	}
}

func TestFeedLinksItemsToTickets(t *testing.T) {
	m := feedModel(t)
	now := testToday.Add(-time.Minute)
	m.Update(feedMsg{reply: feed.Reply{Source: "ci", Items: []feed.Item{
		{Time: now, Text: "by gid", TicketGID: "2"},
		{Time: now, Text: "by url", URL: "https://app.asana.com/0/p1/2"},
		{Time: now, Text: "by branch", Branch: "feat/x"},
		{Time: now, Text: "by branch and repo", Branch: "feat/x", Repo: "/code/web"},
		{Time: now, Text: "other repo", Branch: "feat/x", Repo: "/other"},
		{Time: now, Text: "plain link", URL: "https://example.com/build/7"},
	}}})
	want := map[string]string{"by gid": "2", "by url": "2", "by branch": "1", "by branch and repo": "1", "other repo": "", "plain link": ""}
	for _, it := range m.feedRows() {
		got := ""
		if it.task != nil {
			got = it.task.GID
		}
		if got != want[it.Text] {
			t.Errorf("%q links to %q, want %q", it.Text, got, want[it.Text])
		}
	}
	if line := ansi.Strip(m.feedLine(m.feedRows()[0], false, 100)); !strings.Contains(line, "by gid · Other") {
		t.Fatalf("linked line = %q", line)
	}

	press(m, "3")
	if !m.focusFeed || !m.feed.open {
		t.Fatalf("3 must open and focus the panel: focus %v, open %v", m.focusFeed, m.feed.open)
	}
	if name, _, _ := m.mode(); name != "FEED" {
		t.Fatalf("mode = %q", name)
	}
	press(m, "G")
	if cmd := m.updateFeed("open"); cmd == nil || !m.focusFeed {
		t.Fatal("enter on a plain link must open it and keep the focus")
	}
	press(m, "g", "enter")
	if t1, _ := m.selected(); m.focusFeed || t1.GID != "2" {
		t.Fatalf("enter on a linked item: focus %v, selected %q", m.focusFeed, t1.GID)
	}
}

func TestFeedFailedFeedKeepsItems(t *testing.T) {
	m := feedModel(t)
	m.Update(feedMsg{reply: feed.Reply{Source: "ci", Summary: "2 runs", Items: []feed.Item{{Time: testToday, Text: "run"}}}})
	if s := m.feedSummary(); s != "2 runs" {
		t.Fatalf("summary = %q", s)
	}
	m.Update(feedMsg{err: errors.New("exit status 1")})
	if s := m.feedSummary(); s != "ci failed" || !slices.Contains(itemTexts(m.feedRows()), "run") {
		t.Fatalf("after failure: summary %q, items %q", s, itemTexts(m.feedRows()))
	}
	fresh := feedModel(t)
	fresh.Update(feedMsg{err: errors.New("exit status 1")})
	if s := fresh.feedSummary(); s != "ci-feed failed" {
		t.Fatalf("never-answered feed summary = %q", s)
	}
}

func TestFeedAgentsShowStateChangesAndUnlinked(t *testing.T) {
	m := feedModel(t)
	stray := agents.Agent{Path: "/code/api", State: agents.Idle, Branch: "dev", Repo: "/code/api"}
	m.Update(agentsMsg{list: []agents.Agent{onBranch(agents.Working, ""), stray}})
	items := m.feedRows()
	if len(items) != 2 {
		t.Fatalf("items = %q", itemTexts(items))
	}
	for _, it := range items {
		if !it.Time.IsZero() {
			t.Errorf("first-poll %q has time %v, want none", it.Text, it.Time)
		}
	}
	if line := ansi.Strip(m.feedLine(items[1], false, 120)); !strings.Contains(line, "api · dev · idle · no ticket") {
		t.Fatalf("unlinked line = %q", line)
	}

	m.now = func() time.Time { return testToday.Add(time.Minute) }
	m.Update(agentsMsg{list: []agents.Agent{onBranch(agents.Completed, ""), stray}})
	first := m.feedRows()[0]
	if first.Text != "🤖 wt · completed" || first.task == nil || first.task.GID != "1" || !first.Time.Equal(testToday.Add(time.Minute)) {
		t.Fatalf("state change = %+v", first)
	}
	if line := ansi.Strip(m.feedLine(first, false, 120)); !strings.Contains(line, "12:01") {
		t.Fatalf("state change line = %q", line)
	}
	m.Update(agentsMsg{list: []agents.Agent{stray}})
	if _, ok := m.feed.since["/code/web/wt"]; ok || len(m.feedRows()) != 1 {
		t.Fatal("a stopped agent must leave the panel")
	}
}

func TestFeedShowsTodaysRelatedActivity(t *testing.T) {
	m := feedModel(t)
	task := asana.Task{GID: "9", Name: "Elsewhere"}
	comment := asana.Story{GID: "s1", ResourceSubtype: "comment_added", CreatedBy: &asana.Ref{Name: "Amy"}}
	m.activity.cache.Entries = []activity.Entry{
		{Task: task, Story: comment, At: testToday.Add(-time.Hour), FoundAt: testToday},
		{Task: task, Story: asana.Story{GID: "s2"}, At: testToday.Add(-time.Hour), Mine: true},
		{Task: task, Story: asana.Story{GID: "s3"}, At: testToday.Add(-48 * time.Hour)},
	}
	items := m.feedRows()
	if len(items) != 1 || items[0].Text != "Amy commented" || items[0].task.GID != "9" || items[0].source != "asana" {
		t.Fatalf("items = %+v", items)
	}
	if s := m.feedSummary(); s != "1 new" {
		t.Fatalf("summary = %q", s)
	}
}

func TestFeedHeightFollowsTheListNotRefreshes(t *testing.T) {
	m := feedModel(t)
	if !m.feed.open || m.feedHeight() != 12 {
		t.Fatalf("short list: open %v, rows %d", m.feed.open, m.feedHeight())
	}
	long := make([]asana.Task, 40)
	for i := range long {
		long[i] = asana.Task{GID: string(rune('a' + i)), Name: "Task"}
	}
	// A background refresh keeps the height, so the list does not jump.
	m.background = true
	m.Update(tasksMsg{tasks: long})
	if !m.feed.open {
		t.Fatal("a background refresh changed the panel")
	}
	m.background = false
	m.Update(tasksMsg{tasks: long})
	if m.feed.open || m.feedHeight() != 2 {
		t.Fatalf("long list: open %v, rows %d", m.feed.open, m.feedHeight())
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "╭─ [3] Feed") })
	if len(lines) != 40 || i < 0 || i+1 >= len(lines) || !strings.Contains(lines[i], "F expand") {
		t.Fatalf("collapsed panel missing:\n%s", strings.Join(lines, "\n"))
	}
	// The bottom edge sits under the panel's own top-left corner.
	corner := len([]rune(lines[i][:strings.Index(lines[i], "╭─ [3] Feed")]))
	if []rune(lines[i+1])[corner] != '╰' {
		t.Fatalf("collapsed panel has no bottom edge:\n%s\n%s", lines[i], lines[i+1])
	}

	press(m, "F")
	if !m.feed.open {
		t.Fatal("F must expand the panel")
	}
	m.Update(tasksMsg{tasks: long})
	if !m.feed.open {
		t.Fatal("the toggle must hold until the view changes")
	}

	m.deps.Config.Feed.Expand = config.ExpandCollapsed
	m.feed = newFeedPanel(m.deps.Config.Feed)
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	if m.feed.open {
		t.Fatal("expand = collapsed opened the panel")
	}
}

func TestGitItemsReportPendingWork(t *testing.T) {
	dir := t.TempDir()
	main, wt := filepath.Join(dir, "main"), filepath.Join(dir, "wt")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", main},
		{"-C", main, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", main, "worktree", "add", "-q", "-b", "feat/x", wt},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	targets := []gitTarget{{gid: "1", branch: "feat/x", repos: []string{main}}, {gid: "2", branch: "feat/x", repos: []string{main}}, {gid: "3", branch: "none", repos: []string{main}}}
	if items := gitItems(context.Background(), targets); len(items) != 1 || items[0].Text != "not pushed" || items[0].TicketGID != "1" {
		t.Fatalf("clean branch without upstream = %+v", items)
	}
	os.WriteFile(filepath.Join(wt, "a.txt"), []byte("x"), 0o600)
	items := gitItems(context.Background(), targets)
	if got := itemTexts(wrapRows(items)); !slices.Equal(got, []string{"1 uncommitted file", "not pushed"}) {
		t.Fatalf("dirty = %q", got)
	}
	m := feedModel(t)
	m.Update(gitMsg{seq: m.feed.gitSeq, items: append(items, feed.Item{TicketGID: "2", Text: "2 unpushed commits"})})
	if s := m.feedSummary(); s != "2 unpushed" {
		t.Fatalf("summary = %q", s)
	}
	m.Update(gitMsg{seq: m.feed.gitSeq - 1})
	if len(m.feed.git) == 0 {
		t.Fatal("a stale git check replaced a newer one")
	}
}

func wrapRows(items []feed.Item) []feedRow {
	var out []feedRow
	for _, it := range items {
		out = append(out, feedRow{Item: it})
	}
	return out
}
