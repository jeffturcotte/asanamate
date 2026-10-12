package tui

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/action"
	"github.com/sadmachine/asanamate/internal/agents"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/feed"
	"github.com/sadmachine/asanamate/internal/repo"
	"github.com/sadmachine/asanamate/internal/ticket"
)

// feedPanel is the Feed panel under the ticket list: what feed commands,
// Asana activity, agents, and the git check report.
type feedPanel struct {
	expand   string        // feed.expand
	height   int           // rows when expanded, borders included
	interval time.Duration // feed and git refresh interval; 0 is off
	feeds    []feedResult  // by index into feed.commands
	git      []feed.Item   // the last git check's items
	gitSeq   uint64        // drops git checks older than the newest
	since    map[string]agentMark
	polled   bool   // agents were polled before, so new marks are state changes
	open     bool   // expanded
	manual   bool   // the toggle chose open; auto waits for the view to change
	viewKey  string // the view the toggle chose open for
	cursor   int    // selected item while the panel has focus
}

// feedResult is a feed command's last reply and whether its last run failed.
type feedResult struct {
	reply feed.Reply
	err   string
}

// agentMark is the state an agent was last seen in and since when.
type agentMark struct {
	state agents.State
	at    time.Time
}

// feedRow is one row of the panel. task is the ticket it links to, if any.
type feedRow struct {
	feed.Item
	source string
	task   *asana.Task
}

// toneAccent colors built-in Asana items; feeds have only the protocol tones.
const toneAccent = "accent"

type (
	feedTickMsg struct{}
	feedMsg     struct {
		index int
		reply feed.Reply
		err   error
	}
	gitMsg struct {
		seq   uint64
		items []feed.Item
	}
)

func newFeedPanel(cfg config.Feed) feedPanel {
	def := config.Default().Feed
	interval, _ := config.Feed{RefreshInterval: cmp.Or(cfg.RefreshInterval, def.RefreshInterval)}.Interval()
	return feedPanel{
		expand:   cmp.Or(cfg.Expand, def.Expand),
		height:   cmp.Or(cfg.Height, def.Height),
		interval: interval,
		feeds:    make([]feedResult, len(cfg.Commands)),
		since:    map[string]agentMark{},
	}
}

// startFeed runs the feeds and starts their timer. The git check waits for
// the tickets to load.
func (m *Model) startFeed() tea.Cmd {
	return tea.Batch(m.runFeeds(), m.scheduleFeed())
}

func (m *Model) scheduleFeed() tea.Cmd {
	if m.feed.interval == 0 {
		return nil
	}
	return tea.Tick(m.feed.interval, func(time.Time) tea.Msg { return feedTickMsg{} })
}

// refreshFeed reruns the feeds and the git check.
func (m *Model) refreshFeed() tea.Cmd { return tea.Batch(m.runFeeds(), m.checkGit()) }

func (m *Model) runFeeds() tea.Cmd {
	var cmds []tea.Cmd
	day := m.now()
	for i, command := range m.deps.Config.Feed.Commands {
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), feed.Timeout)
			defer cancel()
			reply, err := feed.Fetch(ctx, command, day)
			return feedMsg{index: i, reply: reply, err: err}
		})
	}
	return tea.Batch(cmds...)
}

func (m *Model) gotFeed(msg feedMsg) {
	if msg.index >= len(m.feed.feeds) {
		return
	}
	r := &m.feed.feeds[msg.index]
	if msg.err != nil {
		r.err = msg.err.Error()
		return
	}
	*r = feedResult{reply: msg.reply}
}

// gitTarget is a loaded ticket's branch and the repos to look for it in.
type gitTarget struct {
	gid    string
	branch string
	repos  []string
}

// checkGit looks for uncommitted files and unpushed commits in the working
// trees checked out on the loaded tickets' branches.
func (m *Model) checkGit() tea.Cmd {
	var targets []gitTarget
	preferred := m.projectFields[gidOf(m.viewProject)]
	for _, t := range m.tasks {
		repos := m.deps.State.LinkedRepos(t)
		if len(repos) == 0 {
			continue
		}
		branch := action.Branch(t, m.deps.State.TaskBranches[t.GID], m.deps.Config.BranchField, preferred)
		targets = append(targets, gitTarget{gid: t.GID, branch: branch, repos: repos})
	}
	m.feed.gitSeq++
	seq := m.feed.gitSeq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), feed.Timeout)
		defer cancel()
		return gitMsg{seq: seq, items: gitItems(ctx, targets)}
	}
}

// gitItems reports each target's pending work, once per working tree.
func gitItems(ctx context.Context, targets []gitTarget) []feed.Item {
	var items []feed.Item
	seen := map[string]bool{}
	for _, t := range targets {
		for _, r := range t.repos {
			wt := repo.Worktree(r, t.branch)
			if wt == "" || seen[wt] {
				continue
			}
			seen[wt] = true
			p, err := repo.PendingWork(ctx, wt)
			if err != nil {
				continue
			}
			item := feed.Item{TicketGID: t.gid, Tone: feed.ToneWarn}
			if p.Uncommitted > 0 {
				item.Time, item.Text = p.ChangedAt, plural(p.Uncommitted, "uncommitted file")
				items = append(items, item)
			}
			switch {
			case p.Unpushed > 0:
				item.Time, item.Text = p.CommitAt, plural(p.Unpushed, "unpushed commit")
				items = append(items, item)
			case p.NoUpstream:
				item.Time, item.Text = p.CommitAt, "not pushed"
				items = append(items, item)
			}
		}
	}
	return items
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (m *Model) gotGit(msg gitMsg) {
	if msg.seq == m.feed.gitSeq {
		m.feed.git = msg.items
	}
}

// markAgents records when each agent entered its state. Agents already
// running at the first poll have no known time.
func (m *Model) markAgents(list []agents.Agent) {
	now, live := m.now(), map[string]bool{}
	if !m.feed.polled {
		now, m.feed.polled = time.Time{}, true
	}
	for _, a := range list {
		live[a.Path] = true
		if mark, ok := m.feed.since[a.Path]; !ok || mark.state != a.State {
			m.feed.since[a.Path] = agentMark{state: a.State, at: now}
		}
	}
	for path := range m.feed.since {
		if !live[path] {
			delete(m.feed.since, path)
		}
	}
}

// feedRows collects the panel's items: upcoming ones first, soonest first,
// then the rest newest first.
func (m *Model) feedRows() []feedRow {
	byGID := map[string]*asana.Task{}
	for i := range m.tasks {
		byGID[m.tasks[i].GID] = &m.tasks[i]
	}
	var items []feedRow
	add := func(source string, it feed.Item) {
		items = append(items, feedRow{Item: it, source: source, task: m.itemTicket(it, byGID)})
	}
	for _, r := range m.feed.feeds {
		for _, it := range r.reply.Items {
			add(r.reply.Source, it)
		}
	}
	for _, it := range m.feed.git {
		add("git", it)
	}
	now := m.now()
	for _, e := range m.activity.cache.Entries {
		if e.Mine || e.Automatic || !sameDay(e.At.In(now.Location()), now) {
			continue
		}
		task := e.Task
		if loaded, ok := byGID[task.GID]; ok {
			task = *loaded
		}
		items = append(items, feedRow{Item: feed.Item{Time: e.At, Text: activitySummary(e), Tone: toneAccent}, source: "asana", task: &task})
	}
	items = append(items, m.agentItems(byGID)...)
	slices.SortStableFunc(items, func(a, b feedRow) int {
		au, bu := a.Time.After(now), b.Time.After(now)
		switch {
		case au && bu:
			return a.Time.Compare(b.Time)
		case au != bu:
			if au {
				return -1
			}
			return 1
		}
		return b.Time.Compare(a.Time)
	})
	return items
}

// itemTicket is the loaded ticket an item links to: by its ticket GID, its
// Asana task URL, or its branch, narrowed to tickets linked to its repo.
func (m *Model) itemTicket(it feed.Item, byGID map[string]*asana.Task) *asana.Task {
	for _, gid := range []string{it.TicketGID, asana.TaskGIDFromURL(it.URL)} {
		if t, ok := byGID[gid]; ok && gid != "" {
			return t
		}
	}
	if it.Branch == "" {
		return nil
	}
	preferred := m.projectFields[gidOf(m.viewProject)]
	for i, t := range m.tasks {
		if action.Branch(t, m.deps.State.TaskBranches[t.GID], m.deps.Config.BranchField, preferred) != it.Branch {
			continue
		}
		if it.Repo == "" || slices.Contains(m.deps.State.LinkedRepos(t), it.Repo) {
			return &m.tasks[i]
		}
	}
	return nil
}

// agentItems lists each agent once, from when it entered its state: linked
// to its ticket, or marked "no ticket" when it links to no loaded ticket.
func (m *Model) agentItems(byGID map[string]*asana.Task) []feedRow {
	linked := map[string]*asana.Task{}
	for _, t := range m.tasks {
		for _, a := range m.viewAgents(t) {
			if _, ok := linked[a.Path]; !ok {
				linked[a.Path] = byGID[t.GID]
			}
		}
	}
	var items []feedRow
	for _, a := range m.agents {
		it := feed.Item{Time: m.feed.since[a.Path].at, Text: m.sym.robot + " " + filepath.Base(a.Path) + " · " + string(a.State)}
		switch a.State {
		case agents.Waiting:
			it.Tone = feed.ToneWarn
		case agents.Completed:
			it.Tone = feed.ToneOK
		case agents.Idle, agents.Unknown:
			it.Tone = feed.ToneMuted
		}
		task := linked[a.Path]
		if task == nil {
			parts := []string{m.sym.robot + " " + filepath.Base(a.Path)}
			if a.Branch != "" {
				parts = append(parts, a.Branch)
			}
			it.Text, it.Tone = strings.Join(append(parts, string(a.State), "no ticket"), " · "), feed.ToneMuted
		}
		items = append(items, feedRow{Item: it, source: "agents", task: task})
	}
	return items
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// feedSummary is the collapsed panel's line: each source's summary, or
// "<source> failed".
func (m *Model) feedSummary() string {
	var parts []string
	for i, r := range m.feed.feeds {
		switch {
		case r.err != "":
			parts = append(parts, feedName(r.reply.Source, m.deps.Config.Feed.Commands[i])+" failed")
		case r.reply.Summary != "":
			parts = append(parts, r.reply.Summary)
		}
	}
	if n := m.activity.newActivity(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new", n))
	}
	branches := map[string]bool{}
	for _, it := range m.feed.git {
		if strings.Contains(it.Text, "unpushed") || it.Text == "not pushed" {
			branches[it.TicketGID] = true
		}
	}
	if len(branches) > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed", len(branches)))
	}
	return strings.Join(parts, " · ")
}

// feedName names a feed that never answered by its command's program.
func feedName(source, command string) string {
	if source != "" {
		return source
	}
	if f := strings.Fields(command); len(f) > 0 {
		return ansi.Truncate(filepath.Base(f[0]), feed.MaxSource, "…")
	}
	return "feed"
}

// feedShown reports whether the panel has a place: under the list panel.
func (m *Model) feedShown() bool {
	_, _, split := m.paneWidths()
	return !m.activityOpen && (split || m.showNav())
}

// feedHeight is the panel's height, borders included: 0 when it has no place,
// 2 collapsed, feed.height expanded, leaving the list at least 3 rows.
func (m *Model) feedHeight() int {
	if !m.feedShown() {
		return 0
	}
	if !m.feed.open {
		return 2
	}
	return max(min(m.feed.height, m.paneHeight()-3), 2)
}

// listHeight is the list's content height above the panel.
func (m *Model) listHeight() int { return max(m.paneHeight()-m.feedHeight(), 1) }

// decideFeed sets whether the panel is open: feed.expand decides, and auto
// opens it when the list leaves room. A toggle holds until the view changes.
// It runs when the view loads or the terminal resizes, never on a background
// refresh, so the list does not jump.
func (m *Model) decideFeed() {
	key := gidOf(m.viewProject) + "\x00" + m.savedView
	if m.feed.manual && key == m.feed.viewKey {
		return
	}
	m.feed.manual = false
	switch m.feed.expand {
	case config.ExpandCollapsed:
		m.feed.open = false
	case config.ExpandExpanded:
		m.feed.open = true
	default:
		listW, _, _ := m.paneWidths()
		m.feed.open = m.listLines(listW)+m.feed.height <= m.paneHeight()
	}
}

// listLines is how many rows the whole list needs.
func (m *Model) listLines(width int) int {
	lines := strings.Split(m.listView(width, len(m.visible)*4+8), "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

// toggleFeed opens or collapses the panel until the view changes.
func (m *Model) toggleFeed() {
	m.feed.open, m.feed.manual = !m.feed.open, true
	m.feed.viewKey = gidOf(m.viewProject) + "\x00" + m.savedView
	if !m.feed.open && m.focusFeed {
		m.focusList()
	}
}

// focusFeedPanel opens the panel and gives it focus.
func (m *Model) focusFeedPanel() {
	if !m.feedShown() {
		return
	}
	if !m.feed.open {
		m.toggleFeed()
	}
	m.focusList()
	m.focusFeed = true
}

// updateFeed runs a [keys.views] binding while the panel has focus.
func (m *Model) updateFeed(name string) tea.Cmd {
	items := m.feedRows()
	if len(items) == 0 {
		return nil
	}
	c := &m.feed.cursor
	*c = min(*c, len(items)-1)
	switch name {
	case "down":
		*c = min(*c+1, len(items)-1)
	case "up":
		*c = max(*c-1, 0)
	case "top":
		*c = 0
	case "bottom":
		*c = len(items) - 1
	case "open":
		it := items[*c]
		switch {
		case it.task != nil:
			m.focusFeed = false
			return m.showTicket(*it.task)
		case it.URL != "":
			return openURL(it.URL)
		}
	}
	return nil
}

// feedView draws the panel width columns wide and rows tall.
func (m *Model) feedView(width, rows int) string {
	right := m.feedSummary()
	toggle := "expand"
	if m.feed.open {
		toggle = "collapse"
	}
	if k := keyLabel("main", "toggle_feed"); k != "" {
		right = strings.TrimPrefix(right+" · "+k+" "+toggle, " · ")
	}
	var highlight = m.focusColor(m.focusFeed)
	if rows <= 2 {
		return m.panel("[3] Feed", right, "", width, rows, highlight)
	}
	return m.panel("[3] Feed", right, m.feedLines(width-panelFrame, rows-panelFrame), width, rows, highlight)
}

// feedLines lists the items in height rows, keeping the cursor in view.
func (m *Model) feedLines(width, height int) string {
	items := m.feedRows()
	if len(items) == 0 {
		return dimStyle.Render(" nothing in the feed yet")
	}
	now := m.now()
	upcoming := 0
	for _, it := range items {
		if it.Time.After(now) {
			upcoming++
		}
	}
	type row struct {
		line string
		item int // -1 for a header
	}
	var rows []row
	for i, it := range items {
		if upcoming > 0 && (i == 0 || i == upcoming) {
			label, n := "Next", upcoming
			if i > 0 {
				label, n = "Earlier", len(items)-upcoming
			}
			rows = append(rows, row{m.groupHeader(m.headerStyle, label, n, width), -1})
		}
		rows = append(rows, row{m.feedLine(it, i == m.feed.cursor && m.focusFeed, width), i})
	}
	cursorRow := slices.IndexFunc(rows, func(r row) bool { return r.item == min(m.feed.cursor, len(items)-1) })
	start := max(cursorRow-height+1, 0)
	if !m.focusFeed {
		start = 0
	}
	var lines []string
	for _, r := range rows[start:min(start+height, len(rows))] {
		lines = append(lines, r.line)
	}
	return strings.Join(lines, "\n")
}

// feedLine is one item: its time, its text in its tone, the linked
// ticket's title, and its source at the right edge.
func (m *Model) feedLine(it feedRow, selected bool, width int) string {
	style := map[string]lipgloss.Style{feed.ToneOK: okStyle, feed.ToneWarn: warnStyle, feed.ToneError: errorStyle, feed.ToneMuted: dimStyle, toneAccent: m.accentStyle.UnsetBold()}[it.Tone]
	when := feedTime(it.Time, m.now())
	source := " " + it.source
	text := style.Render(it.Text)
	if it.task != nil {
		text += dimStyle.Render(" · ") + tagTitle(ticket.Clean(it.task.Name), lipgloss.NewStyle())
	}
	room := max(width-8-ansi.StringWidth(source), 1)
	text = ansi.Truncate(text, room, "…")
	line := " " + dimStyle.Render(fmt.Sprintf("%-7s", when)) + text +
		strings.Repeat(" ", max(room-ansi.StringWidth(text), 0)) + dimStyle.Render(source)
	if selected {
		return selectedStyle.Render(ansi.Strip(line))
	}
	return line
}

// feedTime shows when an item happens: "in 25m" or "in 2h" ahead, the clock
// time earlier today, the date before that, or nothing when unknown.
func feedTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	if d := t.Sub(now); d > 0 {
		if d < time.Hour {
			return fmt.Sprintf("in %dm", max(int(d.Minutes()), 1))
		}
		return fmt.Sprintf("in %dh", int(d.Hours()))
	}
	if sameDay(t.In(now.Location()), now) {
		return t.In(now.Location()).Format("15:04")
	}
	return shortDate(t, now)
}
