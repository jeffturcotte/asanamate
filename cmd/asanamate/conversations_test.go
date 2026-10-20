package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/conversations"
)

// conversationCLI configures a fixture source that prints reply, with no
// Asana token, and returns the reply file so tests can change it.
func conversationCLI(t *testing.T, reply string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "reply.json")
	if err := os.WriteFile(file, []byte(reply), 0o600); err != nil {
		t.Fatal(err)
	}
	cliConfig(t, fmt.Sprintf("workspace = \"1\"\n[conversations]\ncommand = %q\nopen = \"true\"\n", "cat >/dev/null; cat '"+file+"'"))
	t.Setenv(config.TokenEnv, "")
	return file
}

const cliReply = `{"version":1,"conversations":[
{"provider":"codex","session_id":"older id","task_gid":"42","title":"Old [asana:42]","updated_at":"2026-10-01T10:00:00Z","activity":"inactive"},
{"provider":"claude","session_id":"newer","task_gid":"42","title":"New \u001b[31m[asana:42]","updated_at":"2026-10-15T10:00:00Z","archived":true,"activity":"unknown"}],
"diagnostics":[{"provider":"codex","code":"unavailable","severity":"info","message":"not installed"}]}`

func TestConversationsCLI(t *testing.T) {
	conversationCLI(t, cliReply)
	var out strings.Builder
	if err := runConversations([]string{"https://app.asana.com/1/9/project/7/task/42?focus=true"}, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 4 || !strings.HasPrefix(lines[1], "claude") || !strings.Contains(lines[1], "unknown, archived") ||
		!strings.Contains(lines[2], `"older id"`) || strings.Contains(out.String(), "\x1b") || !strings.Contains(lines[3], "info: codex: unavailable: not installed") {
		t.Fatalf("human output:\n%s", out.String())
	}
	out.Reset()
	if err := runConversations([]string{"42", "--json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result conversations.Result
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil || result.Version != 1 || len(result.Conversations) != 2 ||
		result.Conversations[0].SessionID != "newer" || len(result.Diagnostics) != 1 {
		t.Fatalf("json = %s, err = %v", out.String(), err)
	}
	if err := runConversations([]string{"https://app.asana.com/0/42"}, &out); err == nil {
		t.Fatal("project URL accepted as a task")
	}
}

func TestConversationsCLIFailures(t *testing.T) {
	file := conversationCLI(t, `{"version":1,"conversations":[]}`)
	var out strings.Builder
	if err := runConversations([]string{"42"}, &out); err != nil || out.String() != "No linked conversations.\n" {
		t.Fatalf("empty = %q, err = %v", out.String(), err)
	}
	partial := `{"version":1,"conversations":[{"provider":"codex","session_id":"s","task_gid":"42","title":"t","activity":"unknown"}],"diagnostics":[{"provider":"claude","code":"permission_denied","severity":"warning","message":"denied"}]}`
	os.WriteFile(file, []byte(partial), 0o600)
	out.Reset()
	err := runConversations([]string{"--json", "42"}, &out)
	var result conversations.Result
	if !errors.Is(err, errIncompleteLookup) || json.Unmarshal([]byte(out.String()), &result) != nil || len(result.Conversations) != 1 {
		t.Fatalf("partial = %s, err = %v", out.String(), err)
	}
	os.WriteFile(file, []byte(`{"version":2,"conversations":[]}`), 0o600)
	out.Reset()
	err = runConversations([]string{"--json", "42"}, &out)
	if err == nil || json.Unmarshal([]byte(out.String()), &result) != nil || len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != "unsupported_version" || result.Diagnostics[0].Provider != conversations.SourceProvider {
		t.Fatalf("unsupported = %s, err = %v", out.String(), err)
	}
	out.Reset()
	if err := runConversations([]string{"42"}, &out); err == nil || strings.Contains(out.String(), "No linked") {
		t.Fatalf("failed lookup printed %q", out.String())
	}

	cliConfig(t, "workspace = \"1\"\n")
	if err := runConversations([]string{"42"}, &out); err == nil || !strings.Contains(err.Error(), "conversations are off") {
		t.Fatalf("off = %v", err)
	}
}

func TestConversationOpenRequiresExactLinkedIdentity(t *testing.T) {
	conversationCLI(t, cliReply)
	for _, args := range [][]string{
		{"open", "42", "--provider", "codex"},
		{"open", "--session", "newer", "42"},
		{"42", "--provider", "claude", "--session", "newer"},
	} {
		if err := runConversation(args); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	if err := runConversation([]string{"open", "42", "--provider", "codex", "--session", "newer"}); err == nil || !strings.Contains(err.Error(), "is linked to task") {
		t.Fatalf("other provider's ID opened: %v", err)
	}
	// The fixture links task 42 only, so a lookup for 43 fails validation.
	if err := runConversation([]string{"open", "43", "--provider", "claude", "--session", "newer"}); err == nil {
		t.Fatal("conversation opened for another task")
	}
	if snapshots, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_STATE_HOME"), "asanamate", "tickets", "*", "*", "conversation.json")); len(snapshots) != 0 {
		t.Fatalf("rejected opens wrote snapshots: %v", snapshots)
	}
}

func TestDoctorReportsConversations(t *testing.T) {
	conversationCLI(t, cliReply)
	out, err := captureStdout(t, func() error { return runDoctor([]string{"42"}) })
	if !errors.Is(err, errDoctorBlocking) || !strings.Contains(out, "ok: 2 linked to 42") || !strings.Contains(out, "info: codex: unavailable") {
		t.Fatalf("doctor = %q, err = %v", out, err)
	}
	cliConfig(t, "workspace = \"1\"\n[conversations]\ncommand = \"definitely-missing-source list\"\nopen = \"true\"\n")
	out, _ = captureStdout(t, func() error { return runDoctor(nil) })
	if !strings.Contains(out, "warning: conversations.command: definitely-missing-source not found") {
		t.Fatalf("doctor = %q", out)
	}
	cliConfig(t, "workspace = \"1\"\n")
	if out, _ = captureStdout(t, func() error { return runDoctor(nil) }); strings.Contains(out, "conversations") {
		t.Fatalf("off source reported: %q", out)
	}
}
