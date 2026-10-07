package ccmux

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readBackup returns the one timestamped backup of path.
func readBackup(t *testing.T, path string) string {
	t.Helper()
	backups, _ := filepath.Glob(path + ".*.bak")
	if len(backups) != 1 {
		t.Fatalf("backups of %s = %v, want one", path, backups)
	}
	data, _ := os.ReadFile(backups[0])
	return string(data)
}

// offerer runs OfferCodexHook for executable with the given answers.
func offerer(t *testing.T, home, executable string) func(input string) string {
	return func(input string) string {
		t.Helper()
		out := &strings.Builder{}
		if err := OfferCodexHook(bufio.NewReader(strings.NewReader(input)), out, home, executable); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
}

func TestOfferCodexHook(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hooks.json")
	theirs := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"other && x"}]}]},"extra":1}`
	os.WriteFile(path, []byte(theirs), 0o600)
	offer := offerer(t, home, "/bin/it's asanamate-ccmux")

	if out := offer("n\n"); !strings.Contains(out, "inaccurate or wrong") || !strings.Contains(out, "asanamate-ccmux setup") {
		t.Fatalf("decline output = %q", out)
	}
	if data, _ := os.ReadFile(path); string(data) != theirs {
		t.Fatalf("declined but changed: %s", data)
	}

	offer("y\n")
	if data := readBackup(t, path); data != theirs {
		t.Fatalf("backup = %s", data)
	}
	var got struct {
		Hooks map[string][]struct {
			Hooks []struct{ Type, Command string }
		}
		Extra int
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := `'/bin/it'\''s asanamate-ccmux' hook codex`
	stop := got.Hooks["Stop"]
	if got.Extra != 1 || len(stop) != 2 || stop[0].Hooks[0].Command != "other && x" || stop[1].Hooks[0].Command != want {
		t.Fatalf("hooks.json = %s", data)
	}
	if len(got.Hooks["UserPromptSubmit"]) != 1 || len(got.Hooks["SessionEnd"]) != 1 {
		t.Fatalf("missing events: %s", data)
	}

	if out := offer(""); !strings.Contains(out, "up to date") {
		t.Fatalf("installed hook offered again: %q", out)
	}
}

// A hook left at an old path or command, such as `asanamate hook codex`, or
// missing from some events, is replaced in place: other hooks, even in the
// same group, stay, and nothing is doubled.
func TestOfferCodexHookRepairsStaleHook(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hooks.json")
	stale := `{"hooks":{"Stop":[{"matcher":"x","hooks":[` +
		`{"type":"command","command":"other"},{"type":"command","command":"'/old/asanamate' hook codex"}]}],` +
		`"SessionEnd":[{"hooks":[{"type":"command","command":"'/new/asanamate-ccmux' hook codex"}]}]}}`
	os.WriteFile(path, []byte(stale), 0o600)
	offer := offerer(t, home, "/new/asanamate-ccmux")
	if state, err := CodexHook(home, "/new/asanamate-ccmux"); err != nil || state != HookStale {
		t.Fatalf("state = %v, err = %v", state, err)
	}
	if legacy, err := LegacyCodexHook(home); err != nil || !legacy {
		t.Fatalf("legacy = %v, err = %v", legacy, err)
	}
	if out := offer("n\n"); !strings.Contains(out, "out of date") {
		t.Fatalf("decline output = %q", out)
	}
	if data, _ := os.ReadFile(path); string(data) != stale {
		t.Fatalf("declined but changed: %s", data)
	}

	if out := offer("y\n"); !strings.Contains(out, "Replaced the hook") {
		t.Fatalf("repair output = %q", out)
	}
	if data := readBackup(t, path); data != stale {
		t.Fatalf("backup = %s", data)
	}
	var got struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct{ Type, Command string }
		}
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	const want = `'/new/asanamate-ccmux' hook codex`
	stop := got.Hooks["Stop"]
	if len(stop) != 2 || stop[0].Matcher != "x" || len(stop[0].Hooks) != 1 || stop[0].Hooks[0].Command != "other" || stop[1].Hooks[0].Command != want {
		t.Fatalf("hooks.json = %s", data)
	}
	for _, event := range HookEvents() {
		ours := 0
		for _, g := range got.Hooks[event] {
			for _, h := range g.Hooks {
				if h.Command == want {
					ours++
				}
			}
		}
		if ours != 1 {
			t.Fatalf("%s has %d asanamate hooks: %s", event, ours, data)
		}
	}
	if state, _ := CodexHook(home, "/new/asanamate-ccmux"); state != HookCurrent {
		t.Fatalf("state after repair = %v", state)
	}
	if legacy, err := LegacyCodexHook(home); err != nil || legacy {
		t.Fatalf("legacy after repair = %v, err = %v", legacy, err)
	}
	if out := offer(""); !strings.Contains(out, "up to date") {
		t.Fatalf("repaired hook offered again: %q", out)
	}
}

func TestOfferCodexHookSkipsWithoutCodex(t *testing.T) {
	home := filepath.Join(t.TempDir(), "missing")
	if out := offerer(t, home, "asanamate-ccmux")(""); !strings.Contains(out, "no hook needed") {
		t.Fatalf("out = %q", out)
	}
	if state, err := CodexHook(home, "asanamate-ccmux"); err != nil || state != CodexAbsent {
		t.Fatalf("state = %v, err = %v", state, err)
	}
	if legacy, err := LegacyCodexHook(home); err != nil || legacy {
		t.Fatalf("legacy = %v, err = %v", legacy, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("created %s: %v", home, err)
	}
}
