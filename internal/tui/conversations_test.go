package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/conversations"
	"github.com/sadmachine/asanamate/internal/ticket"
)

const conversationsReply = `{"version":1,"conversations":[
{"provider":"codex","session_id":"old","task_gid":"1","title":"Same [asana:1]","updated_at":"2026-10-01T10:00:00Z","activity":"inactive"},
{"provider":"claude","session_id":"new","task_gid":"1","title":"Same [asana:1]","updated_at":"2026-10-15T10:00:00Z","archived":true,"activity":"unknown"},
{"provider":"claude","session_id":"undated","task_gid":"1","title":"\u001b[31mNo time","cwd":"/gone","branch":"fix","activity":"active"}]}`

// replyCommand is a source fixture that prints reply after reading the request.
func replyCommand(reply string) string {
	return "cat >/dev/null; printf '%s' '" + strings.ReplaceAll(reply, "\n", "") + "'"
}

func conversationModel(t *testing.T, reply string, actions ...config.Action) *Model {
	t.Helper()
	m, _ := testModel(t, config.Config{Actions: actions, Conversations: config.Conversations{Command: replyCommand(reply), Open: "true"}})
	tasks := []asana.Task{{GID: "1", Name: "Fix"}, {GID: "2", Name: "Other"}}
	m.tasks, m.visible = tasks, tasks
	m.details["1"] = ticket.Ticket{Task: tasks[0]}
	m.width, m.height = 100, 30
	return m
}

// openConversations picks the action menu's Conversations entry and returns
// the lookup command without running it.
func openConversations(t *testing.T, m *Model) {
	t.Helper()
	m.openActionMenu()
	if m.modal == nil {
		t.Fatal("action menu did not open")
	}
	m.modal.cursor = len(m.modal.items) - 1
	if it := m.modal.current(); it == nil || it.Label != "Conversations" {
		t.Fatalf("last menu item = %+v", it)
	}
	_, cmd := m.Update(key("enter"))
	if m.waiting == "" {
		t.Fatal("lookup shows no loading modal")
	}
	drain(m, cmd)
}

func labels(p *picker) []string {
	var out []string
	for _, it := range p.items {
		out = append(out, it.Label)
	}
	return out
}

func TestConversationsEntryWithoutActions(t *testing.T) {
	m := conversationModel(t, conversationsReply)
	openConversations(t, m)
	if m.modal == nil || !strings.HasPrefix(m.modal.title, "Resume in terminal") {
		t.Fatalf("modal = %+v", m.modal)
	}
	if got := strings.Join(labels(m.modal), "|"); got != "Same|Same|No time|Refresh|Cancel" {
		t.Fatalf("rows = %s", got)
	}
	first, _ := m.modal.items[0].Value.(conversations.Record)
	undated, _ := m.modal.items[2].Value.(conversations.Record)
	if first.SessionID != "new" || !strings.Contains(m.modal.items[0].Hint, "archived") || undated.Title != "\x1b[31mNo time" ||
		!strings.Contains(m.modal.items[2].Hint, "unknown time") || !strings.Contains(m.modal.items[2].Help, "/gone") {
		t.Fatalf("items = %+v", m.modal.items)
	}
	if m.lastAction != -1 {
		t.Fatal("Conversations replaced the repeatable action")
	}
	m.width = 24
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Refresh") {
		t.Fatalf("narrow view lost rows:\n%s", view)
	}
}

func TestConversationsOffAddsNoEntry(t *testing.T) {
	m := conversationModel(t, conversationsReply, config.Action{Name: "Go", Key: "g", Command: "true"})
	m.deps.Config.Conversations = config.Conversations{}
	m.openActionMenu()
	if got := labels(m.modal); len(got) != 1 || got[0] != "Go" {
		t.Fatalf("menu = %v", got)
	}
}

func TestConversationsLateRepliesDropped(t *testing.T) {
	m := conversationModel(t, conversationsReply)
	m.openActionMenu()
	m.modal.cursor = len(m.modal.items) - 1
	_, cmd := m.Update(key("enter"))
	m.Update(key("esc"))
	drain(m, cmd)
	if m.modal != nil || m.waiting != "" {
		t.Fatalf("cancelled lookup opened: %+v", m.modal)
	}

	tk := m.details["1"]
	first := m.lookupConversations(tk, nil)
	second := m.lookupConversations(tk, nil)
	drain(m, first)
	if m.modal != nil {
		t.Fatal("superseded lookup attached")
	}
	m.cursor = 1 // the user moved to another ticket
	drain(m, second)
	if m.modal != nil || len(m.conversationResults) != 0 {
		t.Fatalf("reply attached to another ticket: %+v", m.modal)
	}
}

func TestConversationsRefreshShowsStaleAndKeepsSelection(t *testing.T) {
	m := conversationModel(t, conversationsReply)
	openConversations(t, m)
	tk := m.details["1"]
	cmd := m.pickedConversation(tk, conversationsRefresh)
	if m.modal == nil || !strings.Contains(m.modal.notice, "Refreshing") || len(m.modal.items) != 5 {
		t.Fatalf("stale picker = %+v", m.modal)
	}
	m.modal.cursor = 2 // highlight "undated" while refreshing
	drain(m, cmd)
	if it := m.modal.current(); it == nil || it.Value.(conversations.Record).SessionID != "undated" || m.modal.notice != "" {
		t.Fatalf("refresh lost the selection: %+v", it)
	}

	m.deps.Config.Conversations.Command = "echo history unreadable >&2; exit 3"
	drain(m, m.pickedConversation(tk, conversationsRefresh))
	if len(m.modal.items) != 5 || !strings.Contains(m.modal.notice, "unavailable") || !strings.Contains(m.modal.notice, "last successful result") {
		t.Fatalf("failure replaced the last result: %+v", m.modal)
	}
}

func TestConversationsStates(t *testing.T) {
	for reply, want := range map[string]string{
		`{"version":1,"conversations":[]}`: "No linked conversations.",
		`{"version":1,"conversations":[],"diagnostics":[{"provider":"codex","code":"permission_denied","severity":"warning","message":"denied"}]}`: "Incomplete lookup",
		`{"version":2,"conversations":[]}`: "Unsupported conversation source version",
		`{"version":1,"conversations":[],"diagnostics":[{"provider":"claude","code":"metadata_conflict","severity":"warning","message":"tag and title disagree"}]}`: "repair its ticket metadata",
	} {
		m := conversationModel(t, reply)
		openConversations(t, m)
		if !strings.Contains(m.modal.notice, want) || (want != "No linked conversations." && strings.Contains(m.modal.notice, "No linked")) {
			t.Fatalf("%s: notice = %q", reply, m.modal.notice)
		}
	}
	m := conversationModel(t, "")
	m.deps.Config.Conversations.Command = "exit 1"
	openConversations(t, m)
	if strings.Contains(m.modal.notice, "No linked") || !strings.Contains(m.modal.notice, "unavailable") {
		t.Fatalf("failed lookup notice = %q", m.modal.notice)
	}
}

func TestConversationOpenWritesSnapshot(t *testing.T) {
	m := conversationModel(t, conversationsReply)
	openConversations(t, m)
	record := m.modal.items[0].Value.(conversations.Record)
	if cmd := m.pickedConversation(m.details["1"], record); cmd == nil || m.modal != nil {
		t.Fatalf("open did not hand off: %+v", m.status)
	}
	snapshots, _ := filepath.Glob(filepath.Join(m.deps.StateDir, "tickets", "1", "run-*", "conversation.json"))
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %v", snapshots)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(snapshots[0]), "ticket.json")); err != nil {
		t.Fatal("ticket exports missing beside the snapshot")
	}
}
