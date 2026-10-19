package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/action"
	"github.com/sadmachine/asanamate/internal/conversations"
	"github.com/sadmachine/asanamate/internal/ticket"
)

// conversationLookup is one lookup in flight for a ticket. Starting another,
// cancelling, or closing its picker replaces or clears m.conversations, so a
// late reply is dropped instead of attaching to another ticket.
type conversationLookup struct {
	ticket ticket.Ticket
	cancel context.CancelFunc
	stale  *picker // last result shown while refreshing; nil shows a loading modal
}

type conversationsMsg struct {
	lookup *conversationLookup
	result conversations.Result
	err    error
}

// conversationRow is a picker row that is not a record.
type conversationRow int

const (
	conversationsRefresh conversationRow = iota
	conversationsCancel
)

// conversationsEntry is the action menu's built-in Conversations entry.
type conversationsEntry struct{}

// lookupConversations starts a fresh lookup for t. A previous successful
// result for t stays visible, marked stale, until the reply arrives.
// highlighted is the record or row Value to keep highlighted.
func (m *Model) lookupConversations(t ticket.Ticket, highlighted any) tea.Cmd {
	m.stopConversations()
	ctx, cancel := context.WithCancel(context.Background())
	l := &conversationLookup{ticket: t, cancel: cancel}
	m.conversations = l
	source := conversations.Source{Command: m.deps.Config.Conversations.Command}
	cmd := func() tea.Msg {
		result, err := source.List(ctx, t.GID)
		return conversationsMsg{lookup: l, result: result, err: err}
	}
	if last, ok := m.conversationResults[t.GID]; ok {
		l.stale = m.conversationPicker(t, last, highlighted, "Refreshing… showing the last result.")
		m.modal = l.stale
		return cmd
	}
	return m.wait("Finding conversations…", cmd)
}

// stopConversations cancels the lookup in flight, if any.
func (m *Model) stopConversations() {
	if m.conversations != nil {
		m.conversations.cancel()
		m.conversations = nil
	}
}

func (m *Model) gotConversations(msg conversationsMsg) {
	l := msg.lookup
	if l != m.conversations {
		return
	}
	m.stopConversations()
	if l.stale == nil {
		m.waiting = ""
	} else if m.modal != l.stale {
		return
	}
	if t, ok := m.selected(); !ok || t.GID != l.ticket.GID {
		m.modal = nil
		m.setStatus(statusInfo, "selection changed; conversations closed")
		return
	}
	// Refresh keeps the row highlighted in the stale picker when still present.
	var highlighted any
	if it := l.stale.current(); it != nil {
		highlighted = it.Value
	}
	gid := l.ticket.GID
	if msg.err == nil {
		conversations.Sort(msg.result.Conversations)
		m.conversationResults[gid] = msg.result
		m.modal = m.conversationPicker(l.ticket, msg.result, highlighted, "")
		return
	}
	notice := failureNotice(conversations.Failure(msg.err))
	last, ok := m.conversationResults[gid]
	if ok {
		notice += " Showing the last successful result."
	}
	m.modal = m.conversationPicker(l.ticket, last, highlighted, notice)
}

// conversationPicker lists result's records with Refresh and Cancel rows.
// notice explains a stale or failed lookup; otherwise the picker tells an
// empty complete lookup from a partial one.
func (m *Model) conversationPicker(t ticket.Ticket, result conversations.Result, highlighted any, notice string) *picker {
	items := make([]pickItem, 0, len(result.Conversations)+2)
	for _, c := range result.Conversations {
		items = append(items, conversationItem(c))
	}
	items = append(items,
		pickItem{Label: "Refresh", Value: conversationsRefresh},
		pickItem{Label: "Cancel", Value: conversationsCancel})
	cursor := 0
	for i, it := range items {
		if sameConversationRow(it.Value, highlighted) {
			cursor = i
		}
	}
	var notes []string
	if notice != "" {
		notes = append(notes, notice)
	} else if len(result.Conversations) == 0 && result.Complete() {
		notes = append(notes, "No linked conversations.")
	}
	if !result.Complete() {
		notes = append(notes, "Incomplete lookup; some conversations may be missing.")
	}
	for _, d := range result.Diagnostics {
		notes = append(notes, diagnosticNotice(d))
	}
	p := newPicker(func(res pickResult) tea.Cmd { return m.pickedConversation(t, res.item.Value) },
		"Resume in terminal: "+ticket.Clean(t.Name), items)
	p.helpIcon, p.notice = m.sym.info, strings.Join(notes, "\n")
	p.cursor = cursor
	return p
}

// sameConversationRow compares records by exact identity, other rows by value.
func sameConversationRow(a, b any) bool {
	ra, okA := a.(conversations.Record)
	rb, okB := b.(conversations.Record)
	if okA || okB {
		return okA && okB && ra.Provider == rb.Provider && ra.SessionID == rb.SessionID
	}
	return a != nil && a == b
}

// conversationItem shows a record's display-safe title, without the ticket
// marker the ticket already implies; the exact identity stays in Value.
func conversationItem(c conversations.Record) pickItem {
	title := conversations.DisplayText(c.Title)
	title = strings.TrimSpace(strings.TrimSuffix(title, "[asana:"+c.TaskGID+"]"))
	if title == "" {
		title = "(untitled)"
	}
	provider := conversations.DisplayText(c.Provider)
	help := []string{provider + " " + conversations.DisplayText(c.SessionID)}
	if c.CWD != "" {
		help = append(help, conversations.DisplayText(c.CWD))
	}
	if c.Branch != "" {
		help = append(help, "branch "+conversations.DisplayText(c.Branch))
	}
	return pickItem{
		Label:  title,
		Hint:   provider + " · " + c.UpdatedLabel() + " · " + c.StateLabel(),
		Help:   strings.Join(help, " · "),
		Search: strings.Join(help, " "),
		Value:  c,
	}
}

func failureNotice(d conversations.Diagnostic) string {
	label := map[string]string{
		"unsupported_version": "Unsupported conversation source version",
		"incomplete":          "Conversation lookup did not finish",
		"provider_error":      "Conversation source sent an invalid response",
	}[d.Code]
	if label == "" {
		label = "Conversation lookup unavailable"
	}
	return label + ": " + d.Message + " (" + conversations.Guidance(d.Code) + ")."
}

func diagnosticNotice(d conversations.Diagnostic) string {
	line := conversations.DisplayText(d.Provider) + ": " + d.Message
	if d.Severity != "info" {
		line += " (" + conversations.Guidance(d.Code) + ")"
	}
	return line
}

// pickedConversation refreshes, closes, or opens a record through the
// foreground action handoff, which restores the TUI when the opener exits.
func (m *Model) pickedConversation(t ticket.Ticket, value any) tea.Cmd {
	m.modal = nil
	switch v := value.(type) {
	case conversationRow:
		if v == conversationsRefresh {
			return m.lookupConversations(t, v)
		}
		m.stopConversations()
		return nil
	case conversations.Record:
		m.stopConversations()
		return m.openConversation(t, v)
	}
	return nil
}

func (m *Model) openConversation(t ticket.Ticket, c conversations.Record) tea.Cmd {
	const name = "Resume in terminal"
	files, err := action.WriteFiles(m.deps.StateDir, t)
	if err != nil {
		m.setStatus(statusError, "writing ticket files: "+err.Error())
		return nil
	}
	cmd, err := action.ConversationCommand(m.deps.StateDir, m.deps.Config.Conversations.Open,
		action.Context{Ticket: t, Files: files, BranchField: m.deps.Config.BranchField}, c)
	if err != nil {
		m.setStatus(statusError, name+" failed: "+err.Error())
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return actionDoneMsg{name: name, err: err} })
}
