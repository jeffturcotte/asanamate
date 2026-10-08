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
	err       string
}

type activityMsg struct {
	seq    uint64
	report activity.Report
	err    error
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
	return tea.Batch(func() tea.Msg {
		defer cancel()
		if c == nil {
			return activityMsg{seq: seq, err: fmt.Errorf("Asana client is unavailable")}
		}
		report, err := activity.Fetch(ctx, c, workspace, since, until)
		return activityMsg{seq: seq, report: report, err: err}
	}, m.startSpinner())
}

func (m *Model) gotActivity(msg activityMsg) tea.Cmd {
	a := m.activity
	if a == nil || a.seq != msg.seq {
		return nil
	}
	a.loading = false
	if msg.err != nil {
		a.err = ticket.OneLine(msg.err.Error())
		return nil
	}
	a.report = msg.report
	m.filterActivity()
	return nil
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
	if i := slices.IndexFunc(a.visible, func(e activity.Entry) bool { return e.Story.GID == previous.Story.GID }); i >= 0 {
		a.cursor = i
	} else {
		a.cursor = 0
	}
	m.renderActivityDetail(false)
}

func (m *Model) layoutActivity() {
	a := m.activity
	_, width, split := m.paneWidths()
	if !split {
		width = m.width
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
	lines = append(lines, "", dimStyle.Render(ansi.Wrap("Event: "+ticket.OneLine(e.Story.ResourceSubtype), width, "")), "", dimStyle.Render("Enter: open ticket · Esc: activity list"))
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
		if k == "enter" || k == "esc" {
			a.filtering = false
			a.query.Blur()
			return nil
		}
		return m.updateActivitySearch(msg)
	}
	if m.focusNav && slices.Contains(navKeys, k) {
		return m.updateNav(k)
	}
	move := 0
	switch k {
	case "q":
		m.closeActivity()
		return tea.Quit
	case "I":
		m.closeActivity()
		m.layout()
		return nil
	case "esc":
		if m.focusNav {
			m.focusNav = false
		} else if a.details {
			a.details = false
		} else {
			m.closeActivity()
			m.layout()
		}
	case "[", "]":
		i := slices.Index(activityModes, a.mode)
		if k == "[" {
			i += 2
		} else {
			i++
		}
		a.mode = activityModes[i%len(activityModes)]
		m.filterActivity()
	case "z":
		a.automated = !a.automated
		m.filterActivity()
	case "r":
		return m.reloadActivity()
	case "D":
		m.openActivityPeriod()
	case "/":
		a.filtering = true
		return a.query.Focus()
	case "0":
		if m.showNav() {
			m.focusNav = true
			m.focusNavPanel()
		}
	case "1":
		m.focusNav, a.details = false, false
	case "2", "tab", "shift+tab":
		m.focusNav = false
		a.details = !a.details
	case "enter":
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
	case "o":
		if e, ok := a.selected(); ok {
			return openURL(e.Task.PermalinkURL)
		}
	case "c":
		if e, ok := a.selected(); ok {
			m.status = "link sent to clipboard"
			return tea.SetClipboard(e.Task.PermalinkURL)
		}
	case "p":
		return m.requestProjects(func() tea.Cmd { m.openProjectPicker(); return nil })
	case "?":
		m.help = true
	case "j", "down":
		move = 1
	case "k", "up":
		move = -1
	case "g", "home":
		if a.details {
			a.reader.GotoTop()
			return nil
		}
		move = -len(a.visible)
	case "G", "end":
		if a.details {
			a.reader.GotoBottom()
			return nil
		}
		move = len(a.visible)
	case "ctrl+d":
		move = max(a.reader.Height()/6, 1)
	case "ctrl+u":
		move = -max(a.reader.Height()/6, 1)
	case "ctrl+f", "pgdown":
		move = max(a.reader.Height()/3, 1)
	case "ctrl+b", "pgup":
		move = -max(a.reader.Height()/3, 1)
	}
	if move != 0 {
		if a.details {
			var cmd tea.Cmd
			a.reader, cmd = a.reader.Update(msg)
			return cmd
		}
		a.cursor = max(0, min(a.cursor+move, len(a.visible)-1))
		m.renderActivityDetail(false)
	}
	return nil
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
	m.modal.browseFirst = true
	m.modal.input.Blur()
	m.modal.cursor = slices.Index([]int{1, 7, 30}, m.activity.days)
}

func (m *Model) activityList(width, height int) string {
	a := m.activity
	if len(a.visible) == 0 {
		text := "No matching activity."
		if a.loading {
			text = "Loading workspace activity…"
		} else if a.err != "" {
			text = a.err
		}
		return lipgloss.NewStyle().Width(width).Height(height).Render(ansi.Wrap(ticket.OneLine(text), max(width, 1), ""))
	}
	count := max(height/3, 1)
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
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m *Model) activityPanes(h int) string {
	a := m.activity
	var tabs []string
	for _, mode := range activityModes {
		count := 0
		for _, e := range a.report.Entries {
			if e.Automatic && !a.automated {
				continue
			}
			if mode == "Both" || mode == "Mine" && e.Mine || mode == "Related" && !e.Mine {
				count++
			}
		}
		label := fmt.Sprintf("%s %d", mode, count)
		if mode == a.mode {
			label = selectedStyle.Render("[" + label + "]")
		} else {
			label = dimStyle.Render(label)
		}
		tabs = append(tabs, label)
	}
	top := strings.Join(tabs, "  ") + dimStyle.Render(fmt.Sprintf("  ·  last %d days", a.days))
	if a.filtering {
		top = a.query.View()
	} else if q := a.query.Value(); q != "" {
		top += dimStyle.Render("  /" + ticket.OneLine(q))
	}
	coverage := "API search · accessible tasks · consolidated history"
	if !a.report.Until.IsZero() {
		coverage = fmt.Sprintf("%d/%d tickets scanned · %s · consolidated history", a.report.Scanned, a.report.Total, a.report.Until.Local().Format("15:04 MST"))
	}
	if a.loading {
		coverage = "Loading workspace activity… · r restarts · I returns"
		if len(m.sym.spinner) > 0 {
			coverage = m.sym.spinner[m.frame%len(m.sym.spinner)] + " " + coverage
		}
	}
	if a.err != "" {
		coverage = "Error: " + a.err
	} else if a.report.Warning != "" {
		coverage = "Partial: " + ticket.OneLine(a.report.Warning)
	}
	if a.err == "" && a.report.Warning == "" && !a.loading {
		coverage += " · r refresh"
	}
	if a.automated {
		coverage += " · automated included"
	}
	header := ansi.Truncate(top, m.width, "…") + "\n" + ansi.Truncate(warnStyle.Render(coverage), m.width, "…") + "\n"
	height := max(h-2, 1)
	listW, readerW, split := m.paneWidths()
	if !split {
		a.reader.SetHeight(height)
		body := m.activityList(listW, height)
		if a.details {
			body = a.reader.View()
		}
		return header + lipgloss.NewStyle().Width(m.width).Height(height).Render(body)
	}
	inner := max(height-panelFrame, 1)
	a.reader.SetHeight(inner)
	count := fmt.Sprintf("%d/%d", a.cursor+1, len(a.visible))
	if len(a.visible) == 0 {
		count = "0/0"
	}
	list := m.panel("[1] Activity", count, m.activityList(listW, inner), listW+panelFrame, height, m.focusColor(!a.details && !m.focusNav))
	reader := m.panel("[2] Event", "enter ticket", a.reader.View(), readerW+panelFrame, height, m.focusColor(a.details))
	if m.showNav() {
		nav := m.panel("[0] Views", "", m.navView(navW-panelFrame, inner), navW, height, m.focusColor(m.focusNav))
		return header + lipgloss.JoinHorizontal(lipgloss.Top, nav, list, reader)
	}
	return header + lipgloss.JoinHorizontal(lipgloss.Top, list, reader)
}

func (m *Model) activityStatusline() string {
	a := m.activity
	left := m.accentStyle.Reverse(true).Render(" ACTIVITY ") + fmt.Sprintf(" %s · %d events", a.mode, len(a.visible))
	right := "j/k move  [/] filter  D period  / search  r refresh  ? keys"
	if a.details {
		right = "j/k scroll  enter ticket  esc list"
	}
	if a.filtering {
		right = "enter/esc done · words match ticket, actor, and activity"
	}
	if m.focusNav {
		right = "j/k move  enter open  esc activity"
	}
	if m.status != "" {
		right = ticket.OneLine(m.status)
	}
	gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		return ansi.Truncate(left+"  "+dimStyle.Render(right), m.width, "…")
	}
	return left + strings.Repeat(" ", gap) + dimStyle.Render(right)
}

func (m *Model) activityHelp() string {
	return m.accentStyle.Render("Activity") + "\n\n" +
		"I / Esc     return to tickets\n" +
		"[ / ]       Both / Mine / Related\n" +
		"D           last 24 hours / 7 days / 30 days\n" +
		"/           search ticket, actor, and activity text\n" +
		"z           include / hide automated reminders\n" +
		"j / k       move; scroll when event reader has focus\n" +
		"Tab / 2     focus event reader (also on narrow screens)\n" +
		"Enter       open ticket; narrow list first shows event\n" +
		"Esc         event list; from opened ticket, Activity\n" +
		"r           refresh from API\n" +
		"o / c       open ticket in browser / copy link\n\n" +
		dimStyle.Render("Workspace search requires paid Asana access.\nCurrent ownership/mentions are labelled as current context.\nAsana consolidates stories; this is not an exhaustive audit log.\nActivity stays in memory; no local activity history.\n\nAny key closes")
}
