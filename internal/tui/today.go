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

// todayPanel is the Today panel under the ticket list: what feed commands,
// Asana activity, agents, and the git check report for today.
type todayPanel struct {
	expand   string        // today.expand
	height   int           // rows when expanded, borders included
	interval time.Duration // feed and git refresh interval; 0 is off
	feeds    []feedResult  // by index into today.feeds
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

// todayItem is one row of the panel. task is the ticket it links to, if any.
type todayItem struct {
	feed.Item
	source string
	task   *asana.Task
}

// toneAccent colors built-in Asana items; feeds have only the protocol tones.
const toneAccent = "accent"

type (
	todayTickMsg struct{}
	feedMsg      struct {
		index int
		reply feed.Reply
		err   error
	}
	gitMsg struct {
		seq   uint64
		items []feed.Item
	}
)

func newTodayPanel(cfg config.Today) todayPanel {
	def := config.Default().Today
	interval, _ := config.Today{RefreshInterval: cmp.Or(cfg.RefreshInterval, def.RefreshInterval)}.Interval()
	return todayPanel{
		expand:   cmp.Or(cfg.Expand, def.Expand),
		height:   cmp.Or(cfg.Height, def.Height),
		interval: interval,
		feeds:    make([]feedResult, len(cfg.Feeds)),
		since:    map[string]agentMark{},
	}
}

// startToday runs the feeds and starts their timer. The git check waits for
// the tickets to load.
func (m *Model) startToday() tea.Cmd {
	return tea.Batch(m.runFeeds(), m.scheduleToday())
}

func (m *Model) scheduleToday() tea.Cmd {
	if m.today.interval == 0 {
		return nil
	}
	return tea.Tick(m.today.interval, func(time.Time) tea.Msg { return todayTickMsg{} })
}

// refreshToday reruns the feeds and the git check.
func (m *Model) refreshToday() tea.Cmd { return tea.Batch(m.runFeeds(), m.checkGit()) }

func (m *Model) runFeeds() tea.Cmd {
	var cmds []tea.Cmd
	day := m.now()
	for i, command := range m.deps.Config.Today.Feeds {
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
	if msg.index >= len(m.today.feeds) {
		return
	}
	r := &m.today.feeds[msg.index]
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
	m.today.gitSeq++
	seq := m.today.gitSeq
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
	if msg.seq == m.today.gitSeq {
		m.today.git = msg.items
	}
}

// markAgents records when each agent entered its state. Agents already
// running at the first poll have no known time.
func (m *Model) markAgents(list []agents.Agent) {
	now, live := m.now(), map[string]bool{}
	if !m.today.polled {
		now, m.today.polled = time.Time{}, true
	}
	for _, a := range list {
		live[a.Path] = true
		if mark, ok := m.today.since[a.Path]; !ok || mark.state != a.State {
			m.today.since[a.Path] = agentMark{state: a.State, at: now}
		}
	}
	for path := range m.today.since {
		if !live[path] {
			delete(m.today.since, path)
		}
	}
}

// todayItems collects the panel's items: upcoming ones first, soonest first,
// then the rest newest first.
func (m *Model) todayItems() []todayItem {
	byGID := map[string]*asana.Task{}
	for i := range m.tasks {
		byGID[m.tasks[i].GID] = &m.tasks[i]
	}
	var items []todayItem
	add := func(source string, it feed.Item) {
		items = append(items, todayItem{Item: it, source: source, task: m.itemTicket(it, byGID)})
	}
	for _, r := range m.today.feeds {
		for _, it := range r.reply.Items {
			add(r.reply.Source, it)
		}
	}
	for _, it := range m.today.git {
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
		items = append(items, todayItem{Item: feed.Item{Time: e.At, Text: activitySummary(e), Tone: toneAccent}, source: "asana", task: &task})
	}
	items = append(items, m.agentItems(byGID)...)
	slices.SortStableFunc(items, func(a, b todayItem) int {
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
func (m *Model) agentItems(byGID map[string]*asana.Task) []todayItem {
	linked := map[string]*asana.Task{}
	for _, t := range m.tasks {
		for _, a := range m.viewAgents(t) {
			if _, ok := linked[a.Path]; !ok {
				linked[a.Path] = byGID[t.GID]
			}
		}
	}
	var items []todayItem
	for _, a := range m.agents {
		it := feed.Item{Time: m.today.since[a.Path].at, Text: m.sym.robot + " " + filepath.Base(a.Path) + " · " + string(a.State)}
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
		items = append(items, todayItem{Item: it, source: "agents", task: task})
	}
	return items
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// todaySummary is the collapsed panel's line: each source's summary, or
// "<source> failed".
func (m *Model) todaySummary() string {
	var parts []string
	for i, r := range m.today.feeds {
		switch {
		case r.err != "":
			parts = append(parts, feedName(r.reply.Source, m.deps.Config.Today.Feeds[i])+" failed")
		case r.reply.Summary != "":
			parts = append(parts, r.reply.Summary)
		}
	}
	if n := m.activity.newActivity(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d new", n))
	}
	branches := map[string]bool{}
	for _, it := range m.today.git {
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

// todayShown reports whether the panel has a place: under the list panel.
func (m *Model) todayShown() bool {
	_, _, split := m.paneWidths()
	return !m.activityOpen && (split || m.showNav())
}

// todayRows is the panel's height, borders included: 0 when it has no place,
// 2 collapsed, today.height expanded, leaving the list at least 3 rows.
func (m *Model) todayRows() int {
	if !m.todayShown() {
		return 0
	}
	if !m.today.open {
		return 2
	}
	return max(min(m.today.height, m.paneHeight()-3), 2)
}

// listHeight is the list's content height above the panel.
func (m *Model) listHeight() int { return max(m.paneHeight()-m.todayRows(), 1) }

// decideToday sets whether the panel is open: today.expand decides, and auto
// opens it when the list leaves room. A toggle holds until the view changes.
// It runs when the view loads or the terminal resizes, never on a background
// refresh, so the list does not jump.
func (m *Model) decideToday() {
	key := gidOf(m.viewProject) + "\x00" + m.savedView
	if m.today.manual && key == m.today.viewKey {
		return
	}
	m.today.manual = false
	switch m.today.expand {
	case config.ExpandCollapsed:
		m.today.open = false
	case config.ExpandExpanded:
		m.today.open = true
	default:
		listW, _, _ := m.paneWidths()
		m.today.open = m.listLines(listW)+m.today.height <= m.paneHeight()
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

// toggleToday opens or collapses the panel until the view changes.
func (m *Model) toggleToday() {
	m.today.open, m.today.manual = !m.today.open, true
	m.today.viewKey = gidOf(m.viewProject) + "\x00" + m.savedView
	if !m.today.open && m.focusToday {
		m.focusList()
	}
}

// focusTodayPanel opens the panel and gives it focus.
func (m *Model) focusTodayPanel() {
	if !m.todayShown() {
		return
	}
	if !m.today.open {
		m.toggleToday()
	}
	m.focusList()
	m.focusToday = true
}

// updateToday runs a [keys.views] binding while the panel has focus.
func (m *Model) updateToday(name string) tea.Cmd {
	items := m.todayItems()
	if len(items) == 0 {
		return nil
	}
	c := &m.today.cursor
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
			m.focusToday = false
			return m.showTicket(*it.task)
		case it.URL != "":
			return openURL(it.URL)
		}
	}
	return nil
}

// todayView draws the panel width columns wide and rows tall.
func (m *Model) todayView(width, rows int) string {
	right := m.todaySummary()
	toggle := "expand"
	if m.today.open {
		toggle = "collapse"
	}
	if k := keyLabel("main", "toggle_today"); k != "" {
		right = strings.TrimPrefix(right+" · "+k+" "+toggle, " · ")
	}
	var highlight = m.focusColor(m.focusToday)
	if rows <= 2 {
		return m.panel("[3] Today", right, "", width, rows, highlight)
	}
	return m.panel("[3] Today", right, m.todayLines(width-panelFrame, rows-panelFrame), width, rows, highlight)
}

// todayLines lists the items in height rows, keeping the cursor in view.
func (m *Model) todayLines(width, height int) string {
	items := m.todayItems()
	if len(items) == 0 {
		return dimStyle.Render(" nothing yet today")
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
		rows = append(rows, row{m.todayLine(it, i == m.today.cursor && m.focusToday, width), i})
	}
	cursorRow := slices.IndexFunc(rows, func(r row) bool { return r.item == min(m.today.cursor, len(items)-1) })
	start := max(cursorRow-height+1, 0)
	if !m.focusToday {
		start = 0
	}
	var lines []string
	for _, r := range rows[start:min(start+height, len(rows))] {
		lines = append(lines, r.line)
	}
	return strings.Join(lines, "\n")
}

// todayLine is one item: its time, its text in its tone, the linked
// ticket's title, and its source at the right edge.
func (m *Model) todayLine(it todayItem, selected bool, width int) string {
	style := map[string]lipgloss.Style{feed.ToneOK: okStyle, feed.ToneWarn: warnStyle, feed.ToneError: errorStyle, feed.ToneMuted: dimStyle, toneAccent: m.accentStyle.UnsetBold()}[it.Tone]
	when := todayTime(it.Time, m.now())
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

// todayTime shows when an item happens: "in 25m" or "in 2h" ahead, the clock
// time earlier today, the date before that, or nothing when unknown.
func todayTime(t, now time.Time) string {
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
