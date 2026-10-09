package activity

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sadmachine/asanamate/internal/asana"
)

func entry(gid string, at time.Time, mine bool) Entry {
	return Entry{Task: asana.Task{GID: "t" + gid, HTMLNotes: "<body>notes</body>"}, Story: asana.Story{GID: gid, Text: gid}, At: at, Mine: mine}
}

func gids(entries []Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Story.GID)
	}
	return out
}

func TestMergeDedupesKeepsFirstFoundAndPrunes(t *testing.T) {
	c := &Cache{}
	first, second := until.Add(time.Minute), until.Add(time.Hour)
	c.Merge(Report{User: "me", Until: until, Complete: true, Entries: []Entry{entry("a", until.Add(-time.Hour), false), entry("old", until.Add(-Window-time.Hour), false)}}, first)
	if got := gids(c.Entries); len(got) != 1 || got[0] != "a" || c.Entries[0].Task.HTMLNotes != "" || !c.Until.Equal(until) {
		t.Fatalf("first merge = %v, until %v", got, c.Until)
	}
	edited := entry("a", until.Add(-time.Hour), false)
	edited.Story.Text = "edited"
	c.Merge(Report{User: "me", Until: until.Add(time.Hour), Entries: []Entry{edited, entry("b", until, false)}}, second)
	if got := gids(c.Entries); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("second merge = %v", got)
	}
	if a := c.Entries[1]; a.Story.Text != "edited" || !a.FoundAt.Equal(first) || !c.Entries[0].FoundAt.Equal(second) {
		t.Fatalf("FoundAt = %v / %v", a.FoundAt, c.Entries[0].FoundAt)
	}
	if !c.Until.Equal(until) {
		t.Fatal("incomplete report must not extend Until")
	}
}

func TestReplaceDropsStaleEntriesInsideItsWindow(t *testing.T) {
	c := &Cache{}
	c.Merge(Report{User: "me", Entries: []Entry{entry("gone", until.Add(-time.Hour), false), entry("kept", since.Add(-time.Hour), false), entry("same", until.Add(-2*time.Hour), false)}}, since)
	c.Replace(Report{User: "me", Since: since, Until: until, Entries: []Entry{entry("same", until.Add(-2*time.Hour), false)}}, until)
	if got := gids(c.Entries); len(got) != 3 {
		t.Fatalf("incomplete Replace must merge, got %v", got)
	}
	c.Replace(Report{User: "me", Since: since, Until: until, Complete: true, Entries: []Entry{entry("same", until.Add(-2*time.Hour), false)}}, until)
	if got := gids(c.Entries); len(got) != 2 || got[0] != "same" || got[1] != "kept" || !c.Entries[0].FoundAt.Equal(since) {
		t.Fatalf("Replace = %v", got)
	}
}

func TestNewSkipsOwnAutomatedAndSeen(t *testing.T) {
	related := entry("r", until, false)
	related.FoundAt = until.Add(time.Minute)
	mine, auto, seen, seeded := related, related, related, related
	mine.Mine, auto.Automatic, seen.FoundAt, seeded.FoundAt = true, true, until, time.Time{}
	if !related.NewSince(until) || mine.NewSince(until) || auto.NewSince(until) || seen.NewSince(until) || seeded.NewSince(until) {
		t.Fatal("NewSince must count only related entries found after seen")
	}
	if !(&Cache{Until: until}).Stale(until.Add(Window+time.Second)) || (&Cache{Until: until}).Stale(until.Add(time.Hour)) || !(&Cache{}).Stale(until) {
		t.Fatal("Stale must follow the window")
	}
}

func TestCacheRoundTripAndMismatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", CacheFile)
	c := LoadCache(path, "w")
	c.Merge(Report{User: "me", Until: until, Complete: true, Entries: []Entry{entry("a", until, false)}}, until)
	c.SeenAt = since
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("stat = %v, %v", info, err)
	}
	got := LoadCache(path, "w")
	if got.User != "me" || !got.Until.Equal(until) || !got.SeenAt.Equal(since) || len(got.Entries) != 1 || !got.Entries[0].FoundAt.Equal(until) {
		t.Fatalf("round trip = %+v", got)
	}
	if other := LoadCache(path, "other"); len(other.Entries) != 0 || other.Workspace != "other" {
		t.Fatal("another workspace must get an empty cache")
	}
	got.Merge(Report{User: "someone", Entries: []Entry{entry("b", until, false)}}, until)
	if g := gids(got.Entries); len(g) != 1 || g[0] != "b" || !got.Until.IsZero() || !got.SeenAt.IsZero() {
		t.Fatalf("another user = %+v", got)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if bad := LoadCache(path, "w"); len(bad.Entries) != 0 {
		t.Fatal("corrupt file must give an empty cache")
	}
	if err := LoadCache("", "w").Save(); err != nil {
		t.Fatal("empty path must keep the cache in memory")
	}
}
