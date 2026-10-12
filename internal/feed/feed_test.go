package feed

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var day = time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local)

// TestFeedFixture runs testdata/feed.sh, the minimal feed docs/extensions.md
// describes, and checks the request it received, so a protocol change fails
// here.
func TestFeedFixture(t *testing.T) {
	script, err := filepath.Abs("testdata/feed.sh")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "requests.jsonl")
	reply, err := Fetch(context.Background(), "/bin/sh '"+script+"' '"+log+"'", day)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Source != "ci" || reply.Summary != "1 failed" || len(reply.Items) != 2 {
		t.Fatalf("reply = %+v", reply)
	}
	first := reply.Items[0]
	if first.Text != "build failed" || first.Branch != "feature/x" || first.Repo != "/code/web" || first.Tone != ToneError ||
		!first.Time.Equal(time.Date(2026, 10, 12, 18, 5, 0, 0, time.UTC)) {
		t.Fatalf("first item = %+v", first)
	}
	if reply.Items[1].URL != "https://example.com/deploys" {
		t.Fatalf("second item = %+v", reply.Items[1])
	}
	data, _ := os.ReadFile(log)
	if got := strings.TrimSpace(string(data)); got != `{"version":1,"operation":"feed","date":"2026-10-12"}` {
		t.Fatalf("request = %s", got)
	}
}

func TestFetchCleansAndRejects(t *testing.T) {
	reply, err := Fetch(context.Background(), `printf '%s' '{"source":"x","summary":"`+strings.Repeat("s", 30)+`","items":[{"time":"2026-10-12T09:00:00Z","text":"a\tb\n  c"}]}'`, day)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Items[0].Text != "a b c" || len([]rune(reply.Summary)) != MaxSummary || !strings.HasSuffix(reply.Summary, "…") {
		t.Fatalf("reply = %+v", reply)
	}
	for name, body := range map[string]string{
		"no source":   `{"items":[]}`,
		"long source": `{"source":"123456789","items":[]}`,
		"no items":    `{"source":"x"}`,
		"no time":     `{"source":"x","items":[{"text":"a"}]}`,
		"no text":     `{"source":"x","items":[{"time":"2026-10-12T09:00:00Z","text":" "}]}`,
		"bad tone":    `{"source":"x","items":[{"time":"2026-10-12T09:00:00Z","text":"a","tone":"red"}]}`,
		"not json":    `nope`,
	} {
		if _, err := Fetch(context.Background(), "printf '%s' '"+body+"'", day); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := Fetch(context.Background(), "echo broken >&2; exit 3", day); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("failing command error = %v", err)
	}
	if reply, err := Fetch(context.Background(), `printf '%s' '{"source":"x","items":[]}'`, day); err != nil || len(reply.Items) != 0 {
		t.Fatalf("empty feed = %+v, %v", reply, err)
	}
}
