package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/activity"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/ticket"
)

var activityModes = []string{"Both", "Mine", "Related"}

type activityView struct {
	report    activity.Report
	visible   []activity.Entry
	mode      string
	days      int
	cursor    int
	query     textinput.Model
	filtering bool
	automated bool
	details   bool
	reader    viewport.Model
	loading   bool
	seq       uint64
	cancel    context.CancelFunc
	updates   chan activityMsg
	err       string
}

// activityMsg carries a scan snapshot. Partial snapshots stream in while the
// scan runs; the last one is final and may carry an error.
type activityMsg struct {
	seq     uint64
	report  activity.Report
	err     error
	partial bool
}

func (m *Model) openActivity() tea.Cmd {
	if m.activityOpen {
		m.closeActivity()
		m.layout()
		return nil
	}
	if m.activity == nil {
		query := textinput.New()
		query.Prompt = "/"
		m.activity = &activityView{mode: "Both", days: 7, query: query, reader: viewport.New()}
	}
	m.activityOpen, m.activityTicket = true, false
	m.focusList()
	m.layout()
	return m.reloadActivity()
}

func (m *Model) closeActivity() {
	m.activityOpen, m.activityTicket = false, false
	if a := m.activity; a != nil {
		if a.cancel != nil {
			a.cancel()
		}
		a.seq++
		a.loading, a.filtering, a.details = false, false, false
		a.query.Blur()
	}
	m.focusList()
}

func (m *Model) reloadActivity() tea.Cmd {
	a := m.activity
	if a.cancel != nil {
		a.cancel()
	}
	a.seq++
	seq := a.seq
	a.loading, a.err = true, ""
	until := m.now()
	since := until.Add(-time.Duration(a.days) * 24 * time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 3*requestTimeout)
	a.cancel = cancel
	c, workspace := m.deps.Client, m.deps.Config.Workspace
	if c == nil {
		cancel()
		return func() tea.Msg { return activityMsg{seq: seq, err: fmt.Errorf("Asana client is unavailable")} }
	}
	// The buffer holds only the newest snapshot, so a slow UI skips stale
	// ones and the scan never blocks. Publishes are serialized.
	updates := make(chan activityMsg, 1)
	a.updates = updates
	publish := func(msg activityMsg) {
		select {
		case <-updates:
		default:
		}
		updates <- msg
	}
	go func() {
		defer cancel()
		report, err := activity.Fetch(ctx, c, workspace, since, until, func(r activity.Report) {
			publish(activityMsg{seq: seq, report: r, partial: true})
		})
		publish(activityMsg{seq: seq, report: report, err: err})
	}()
	return tea.Batch(waitActivity(updates), m.startSpinner())
}

func waitActivity(updates <-chan activityMsg) tea.Cmd {
	return func() tea.Msg { return <-updates }
}

func (m *Model) gotActivity(msg activityMsg) tea.Cmd {
	a := m.activity
	if a == nil || a.seq != msg.seq {
		return nil
	}
	var next tea.Cmd
	if msg.partial {
		next = waitActivity(a.updates)
	} else {
		a.loading = false
	}
	if msg.err != nil {
		a.err = ticket.OneLine(msg.err.Error())
		return nil
	}
	a.report = msg.report
	m.filterActivity()
	return next
}

func (a *activityView) selected() (activity.Entry, bool) {
	if a.cursor >= 0 && a.cursor < len(a.visible) {
		return a.visible[a.cursor], true
	}
	return activity.Entry{}, false
}

func (m *Model) filterActivity() {
	a := m.activity
	previous, _ := a.selected()
	a.visible = nil
	words := strings.Fields(strings.ToLower(a.query.Value()))
	for _, e := range a.report.Entries {
		if e.Automatic && !a.automated || a.mode == "Mine" && !e.Mine || a.mode == "Related" && e.Mine {
			continue
		}
		text := strings.ToLower(e.Task.Name + " " + e.Story.Text + " " + ticket.Author(e.Story) + " " + e.Story.ResourceSubtype + " " + strings.Join(e.Reasons, " "))
		if !slices.ContainsFunc(words, func(word string) bool { return !strings.Contains(text, word) }) {
			a.visible = append(a.visible, e)
		}
	}
	i := slices.IndexFunc(a.visible, func(e activity.Entry) bool { return e.Story.GID == previous.Story.GID })
	a.cursor = max(i, 0)
	// Streamed snapshots must not reset the reader under the user.
	m.renderActivityDetail(i >= 0)
}

func (m *Model) layoutActivity() {
	a := m.activity
	_, width, split := m.paneWidths()
	if !split {
		width = m.width - m.navWidth()
	}
	a.reader.SetWidth(max(width, 1))
	a.reader.SetHeight(max(m.bodyHeight()-3, 1))
	a.query.SetWidth(max(m.width-4, 1))
	m.renderActivityDetail(true)
}

func (m *Model) renderActivityDetail(keepScroll bool) {
	a := m.activity
	e, ok := a.selected()
	if !ok {
		a.reader.SetContent(dimStyle.Render("Select an activity entry to see its details."))
		return
	}
	width := max(a.reader.Width()-2, 1)
	actor := ticket.Author(e.Story)
	if e.Mine {
		actor = "You"
	}
	if e.Automatic {
		actor = "Asana / automated"
	}
	reason := strings.Join(e.Reasons, "; ")
	if e.Mine {
		reason = "Your action"
	}
	lines := []string{
		m.accentStyle.Render(ansi.Wrap(ticket.OneLine(e.Task.Name), width, "")), "",
		ansi.Wrap(actor+" · "+e.At.Local().Format("Mon Jan 2, 15:04 MST"), width, ""),
		dimStyle.Render(ansi.Wrap("Why here: "+reason, width, "")), "",
		strings.Repeat("─", width), "",
	}
	if e.Story.HTMLText != "" {
		lines = append(lines, m.glamour(ticket.HTMLToMarkdown(e.Story.HTMLText), width, true))
	} else {
		lines = append(lines, ansi.Wrap(ticket.Clean(e.Story.Text), width, ""))
	}
	if e.Story.NewDates != nil {
		lines = append(lines, "", "Previous due: "+activityDue(e.Story.OldDates), "New due: "+activityDue(e.Story.NewDates))
	}
	if e.Story.Assignee != nil {
		lines = append(lines, "", "Assigned to: "+ticket.OneLine(e.Story.Assignee.Name))
	}
	lines = append(lines, "", dimStyle.Render(ansi.Wrap("Event: "+ticket.OneLine(e.Story.ResourceSubtype), width, "")), "", dimStyle.Render(keyLabel("main", "open")+": open ticket · "+keyLabel("main", "focus_list")+": activity list"))
	before := a.reader.YOffset()
	a.reader.SetContent(strings.Join(lines, "\n"))
	if keepScroll {
		a.reader.SetYOffset(before)
	} else {
		a.reader.GotoTop()
	}
}

func activityDue(d *asana.StoryDates) string {
	if d == nil {
		return "none"
	}
	if d.DueAt != nil {
		return ticket.OneLine(*d.DueAt)
	}
	if d.DueOn != nil {
		return ticket.OneLine(*d.DueOn)
	}
	return "none"
}

func (m *Model) handleActivityKey(msg tea.KeyPressMsg) tea.Cmd {
	a, k := m.activity, msg.String()
	if a.filtering {
		if name := keys.Name("filter", k); name == "done" || name == "cancel" {
			a.filtering = false
			a.query.Blur()
			return nil
		}
		return m.updateActivitySearch(msg)
	}
	if m.focusNav {
		if name := keys.Name("views", k); name != "" {
			return m.updateNav(name)
		}
	}
	name := keys.Name("activity", k)
	if name == "" {
		name = keys.Name("main", k)
	}
	move := 0
	switch name {
	case "quit":
		m.closeActivity()
		return tea.Quit
	case "activity":
		m.closeActivity()
		m.layout()
		return nil
	case "focus_list":
		if m.focusNav {
			m.focusNav = false
		} else if a.details {
			a.details = false
		} else {
			m.closeActivity()
			m.layout()
		}
	case "prev_mode", "next_mode":
		i := slices.Index(activityModes, a.mode)
		if name == "prev_mode" {
			i += 2
		} else {
			i++
		}
		a.mode = activityModes[i%len(activityModes)]
		m.filterActivity()
	case "toggle_reader":
		if !m.readerHidden {
			a.details = false
		}
		return m.toggleReader()
	case "toggle_sidebar":
		return m.toggleSidebar()
	case "automated":
		a.automated = !a.automated
		m.filterActivity()
	case "reload":
		return m.reloadActivity()
	case "period":
		m.openActivityPeriod()
	case "filter":
		a.filtering = true
		return a.query.Focus()
	case "focus_views":
		if m.showNav() {
			m.focusNav = true
			m.focusNavPanel()
		}
	case "focus_reader":
		m.focusNav = false
		a.details = !a.details
	case "next_pane", "prev_pane":
		m.cycleActivityPane(name == "prev_pane")
	case "open":
		if _, _, split := m.paneWidths(); !split && !a.details {
			a.details = true
			return nil
		}
		if e, ok := a.selected(); ok {
			m.activityOpen, m.activityTicket = false, true
			m.layout()
			cmd := m.showTicket(e.Task)
			// The normal reader may have been cached before this workspace
			// scan. Refresh it before the user acts on older ticket details.
			if _, cached := m.details[e.Task.GID]; cached {
				return tea.Batch(cmd, m.loadDetail(e.Task.GID))
			}
			return cmd
		}
	case "open_browser":
		if e, ok := a.selected(); ok {
			return openURL(e.Task.PermalinkURL)
		}
	case "copy_link":
		if e, ok := a.selected(); ok {
			m.status = "link sent to clipboard"
			return tea.SetClipboard(e.Task.PermalinkURL)
		}
	case "projects":
		return m.requestProjects(func() tea.Cmd { m.openProjectPicker(); return nil })
	case "help":
		m.help = true
	case "down", "scroll_down":
		move = 1
	case "up", "scroll_up":
		move = -1
	case "top":
		if a.details {
			a.reader.GotoTop()
			return nil
		}
		move = -len(a.visible)
	case "bottom":
		if a.details {
			a.reader.GotoBottom()
			return nil
		}
		move = len(a.visible)
	case "half_page_down":
		move = max(a.reader.Height()/6, 1)
	case "half_page_up":
		move = -max(a.reader.Height()/6, 1)
	case "page_down":
		move = max(a.reader.Height()/3, 1)
	case "page_up":
		move = -max(a.reader.Height()/3, 1)
	}
	if move != 0 {
		if a.details {
			if move > 0 {
				a.reader.ScrollDown(move)
			} else {
				a.reader.ScrollUp(-move)
			}
			return nil
		}
		a.cursor = max(0, min(a.cursor+move, len(a.visible)-1))
		m.renderActivityDetail(false)
	}
	return nil
}

// cycleActivityPane moves focus to the next or, with back, previous of the
// views panel, the list and the event reader.
func (m *Model) cycleActivityPane(back bool) {
	a := m.activity
	first, i := 1, 1
	if m.showNav() {
		first = 0
	}
	switch {
	case m.focusNav:
		i = 0
	case a.details:
		i = 2
	}
	dir := 1
	if back {
		dir = -1
	}
	n := 3 - first
	i = first + (i-first+dir+n)%n
	m.focusNav, a.details = false, i == 2
	if i == 0 {
		m.focusNavPanel()
	}
}

func (m *Model) updateActivitySearch(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.activity.query, cmd = m.activity.query.Update(msg)
	m.filterActivity()
	return cmd
}

func (m *Model) openActivityPeriod() {
	items := []pickItem{{Label: "Last 24 hours", Value: 1}, {Label: "Last 7 days", Value: 7}, {Label: "Last 30 days", Value: 30}}
	m.modal = newPicker(pickValue(func(days int) tea.Cmd {
		m.modal = nil
		m.activity.days, m.activity.cursor = days, 0
		// Old results belong to a different period; do not relabel them.
		m.activity.report, m.activity.visible = activity.Report{}, nil
		m.renderActivityDetail(false)
		return m.reloadActivity()
	}), "Activity period", items)
	m.modal.searching = false
	m.modal.input.Blur()
	m.modal.cursor = slices.Index([]int{1, 7, 30}, m.activity.days)
}

func (m *Model) activityList(width, height int) string {
	a := m.activity
	// An error or partial scan leads the list, above any older results.
	var banner string
	rows := height
	if note := a.note(); note != "" {
		banner = warnStyle.Render(ansi.Wrap(note, max(width, 1), "")) + "\n"
		rows -= lipgloss.Height(banner) - 1
	}
	box := lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height)
	if len(a.visible) == 0 {
		text := "No matching activity."
		if a.loading {
			text = "Loading workspace activity…"
		} else if a.err != "" {
			text = ""
		}
		return box.Render(banner + ansi.Wrap(text, max(width, 1), ""))
	}
	count := max(rows/3, 1)
	start := max(0, a.cursor-count+1)
	var lines []string
	for i := start; i < min(start+count, len(a.visible)); i++ {
		e := a.visible[i]
		lead := "  "
		if i == a.cursor {
			lead = m.sym.cursor + " "
		}
		icon := m.sym.icon(iconView)
		switch e.Story.ResourceSubtype {
		case "due_date_changed", "due_today":
			icon = m.sym.icon(iconDue)
		case "section_changed", "added_to_project":
			icon = m.sym.icon(iconFolder)
		}
		kind := "RELATED"
		if e.Mine {
			kind = "MINE"
		}
		if e.Automatic {
			kind = "AUTO"
		}
		summary := ticket.OneLine(e.Story.Text)
		if e.Story.ResourceSubtype == "comment_added" {
			summary = ticket.Author(e.Story) + " commented"
		}
		if e.Mine && e.Story.CreatedBy != nil {
			summary = strings.Replace(summary, ticket.OneLine(e.Story.CreatedBy.Name), "You", 1)
		}
		rows := []string{lead + icon + ticket.OneLine(e.Task.Name), "  " + summary, "  " + e.At.Local().Format("01/02 15:04") + "  " + kind}
		for j, row := range rows {
			row = ansi.Truncate(row, max(width, 1), "…")
			if i == a.cursor && !a.details && !m.focusNav {
				row = selectedStyle.Render(row + strings.Repeat(" ", max(width-ansi.StringWidth(row), 0)))
			} else if j > 0 {
				row = dimStyle.Render(row)
			}
			lines = append(lines, row)
		}
	}
	return box.Render(banner + strings.Join(lines, "\n"))
}

// count is how many loaded events mode shows, before the search.
func (a *activityView) count(mode string) int {
	n := 0
	for _, e := range a.report.Entries {
		if e.Automatic && !a.automated {
			continue
		}
		if mode == "Both" || mode == "Mine" && e.Mine || mode == "Related" && !e.Mine {
			n++
		}
	}
	return n
}

// note is the load error or partial-scan warning, if any.
func (a *activityView) note() string {
	if a.err != "" {
		return "Error: " + a.err
	}
	if a.report.Warning != "" {
		return "Partial: " + ticket.OneLine(a.report.Warning)
	}
	return ""
}

func (m *Model) activityPanes(h int) string {
	a := m.activity
	listW, readerW, split := m.paneWidths()
	if !split {
		a.reader.SetHeight(h)
		body := m.activityList(listW, h)
		if a.details {
			body = a.reader.View()
		}
		if m.showNav() {
			nav := m.panel("[0] Views", "", m.navView(navW-panelFrame, max(h-panelFrame, 1)), navW, h, m.focusColor(m.focusNav))
			return lipgloss.JoinHorizontal(lipgloss.Top, nav, lipgloss.NewStyle().Width(listW).Height(h).Render(body))
		}
		return lipgloss.NewStyle().Width(m.width).Height(h).Render(body)
	}
	inner := max(h-panelFrame, 1)
	a.reader.SetHeight(inner)
	count := fmt.Sprintf("%d/%d", a.cursor+1, len(a.visible))
	if len(a.visible) == 0 {
		count = "0/0"
	}
	list := m.panel("[1] Activity · "+a.mode, count, m.activityList(listW, inner), listW+panelFrame, h, m.focusColor(!a.details && !m.focusNav))
	reader := m.panel("[2] Event", "enter ticket", a.reader.View(), readerW+panelFrame, h, m.focusColor(a.details))
	if m.showNav() {
		nav := m.panel("[0] Views", "", m.navView(navW-panelFrame, inner), navW, h, m.focusColor(m.focusNav))
		return lipgloss.JoinHorizontal(lipgloss.Top, nav, list, reader)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, reader)
}

// activityMode is mode for the Activity view.
func (m *Model) activityMode() (name string, pill lipgloss.Style, hints [][2]string) {
	switch a := m.activity; {
	case a.filtering:
		return "FILTER", filterPill, [][2]string{{typedLabel("filter", "done"), "apply"}, {typedLabel("filter", "cancel"), "cancel"}}
	case m.focusNav:
		return "VIEWS", normalPill, [][2]string{{keyLabel("views", "down", "up"), "move"}, {keyLabel("views", "open"), "open"}, {keyLabel("main", "focus_list"), "activity"}}
	case a.details:
		return "ACTIVITY", readPill, [][2]string{{keyLabel("main", "down", "up"), "scroll"}, {keyLabel("main", "open"), "ticket"}, {keyLabel("main", "focus_list"), "list"}}
	}
	return "ACTIVITY", normalPill, [][2]string{{keyLabel("main", "down", "up"), "move"}, {keyLabel("activity", "prev_mode", "next_mode"), "filter"},
		{keyLabel("activity", "period"), "period"}, {keyLabel("main", "filter"), "search"}, {keyLabel("main", "reload"), "refresh"},
		{keyLabel("main", "activity"), "back"}, {keyLabel("main", "help"), "keys"}}
}

// activitySegs are the statusline segments for the Activity view: what it
// shows and how complete the last scan was.
func (m *Model) activitySegs() []string {
	a := m.activity
	period := fmt.Sprintf("last %d days", a.days)
	if a.days == 1 {
		period = "last 24 hours"
	}
	segs := []string{m.sym.icon(iconView) + "Activity · " + a.mode, period}
	if a.filtering {
		segs = append(segs, a.query.View())
	} else if q := a.query.Value(); q != "" {
		segs = append(segs, m.sym.icon(iconFilter)+ticket.OneLine(q))
	}
	if a.automated {
		segs = append(segs, "automated shown")
	}
	segs = append(segs, fmt.Sprintf("%d events", len(a.visible)))
	switch {
	case a.loading:
		loading := "scanning"
		if a.report.Total > 0 {
			loading = fmt.Sprintf("scanning %d/%d", a.report.Scanned, a.report.Total)
		}
		if len(m.sym.spinner) > 0 {
			loading = m.sym.spinner[m.frame%len(m.sym.spinner)] + " " + loading
		}
		segs = append(segs, loading)
	case !a.report.Until.IsZero():
		segs = append(segs, fmt.Sprintf("%d/%d scanned %s", a.report.Scanned, a.report.Total, a.report.Until.Local().Format("15:04")))
	}
	return segs
}

func (m *Model) activityHelp() string {
	rows := [][3]string{
		{"main", "activity", "return to tickets"},
		{"activity", "prev_mode", "previous filter: Both / Mine / Related"},
		{"activity", "next_mode", "next filter: Both / Mine / Related"},
		{"activity", "period", "last 24 hours / 7 days / 30 days"},
		{"main", "filter", "search ticket, actor, and activity text"},
		{"activity", "automated", "include / hide automated reminders"},
		{"main", "down", "move down; scroll event reader"},
		{"main", "up", "move up; scroll event reader"},
		{"main", "next_pane", "next pane: views, list, event reader"},
		{"main", "open", "open ticket; narrow list first shows event"},
		{"main", "focus_list", "event list; from opened ticket, Activity"},
		{"main", "toggle_reader", "show / hide preview"},
		{"main", "toggle_sidebar", "show / hide views panel"},
		{"main", "reload", "refresh from API"},
		{"main", "open_browser", "open ticket in browser"},
		{"main", "copy_link", "copy ticket link"},
	}
	lines := []string{m.accentStyle.Render("Activity"), ""}
	for _, row := range rows {
		if label := keyLabel(row[0], row[1]); label != "" {
			lines = append(lines, m.accentStyle.Width(12).Render(label)+row[2])
		}
	}
	return strings.Join(lines, "\n") + "\n\n" + dimStyle.Render("Workspace search requires paid Asana access.\nCurrent ownership/mentions are labelled as current context.\nAsana consolidates stories; this is not an exhaustive audit log.\nActivity stays in memory; no local activity history.\n\nAny key closes")
}
