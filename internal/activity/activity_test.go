package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sadmachine/asanamate/internal/asana"
)

var since = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
var until = since.AddDate(0, 0, 7)

func story(gid, actor, kind string) asana.Story {
	s := asana.Story{GID: gid, CreatedAt: since.Add(time.Hour).Format(time.RFC3339), ResourceSubtype: kind}
	if actor != "" {
		s.CreatedBy = &asana.Ref{GID: actor, Name: actor}
	}
	return s
}

func TestClassifyOwnAndDirectActivity(t *testing.T) {
	task := asana.Task{GID: "1", Assignee: &asana.Ref{GID: "other"}}
	own := story("own", "me", "assigned")
	own.Assignee = &asana.Ref{GID: "other"}
	assigned := story("assigned", "other", "assigned")
	assigned.Assignee = &asana.Ref{GID: "me"}
	mention := story("mention", "other", "comment_added")
	mention.HTMLText = `<body><a data-asana-type="user" data-asana-gid="me">Austin</a></body>`
	unrelated := story("unrelated", "other", "comment_added")
	unrelated.HTMLText = `<body>Austin <a data-asana-type="task" data-asana-gid="me">link</a></body>`
	old := story("old", "me", "comment_added")
	old.CreatedAt = since.Add(-time.Second).Format(time.RFC3339)
	future := story("future", "me", "comment_added")
	future.CreatedAt = until.Add(time.Second).Format(time.RFC3339)
	bad := story("bad", "me", "comment_added")
	bad.CreatedAt = "bad timestamp"
	entries := Classify(task, []asana.Story{own, assigned, mention, unrelated, old, future, bad, own}, "me", since, until)
	if len(entries) != 3 || !entries[0].Mine || entries[1].Mine || entries[2].Mine {
		t.Fatalf("classification = %+v", entries)
	}
	if !slices.Contains(entries[1].Reasons, "assigned to you") || !slices.Contains(entries[2].Reasons, "comment mentions you") {
		t.Fatalf("reasons missing: %+v", entries)
	}
}

func TestAutomatedRemindersAreNotOwnActions(t *testing.T) {
	task := asana.Task{GID: "1", Assignee: &asana.Ref{GID: "me"}}
	entries := Classify(task, []asana.Story{story("due", "me", "due_today"), story("reminder", "me", "mentioned_comment_reminder_story"), story("system", "", "text_custom_field_changed")}, "me", since, until)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		if e.Mine || !e.Automatic || !slices.Contains(e.Reasons, "currently assigned to you") {
			t.Fatalf("entry = %+v", e)
		}
	}
}

func TestDescriptionMentionsAreCurrentContext(t *testing.T) {
	task := asana.Task{HTMLNotes: `<body><a data-asana-gid='me' data-asana-type='user'>Austin</a></body>`}
	entries := Classify(task, []asana.Story{story("1", "other", "due_date_changed")}, "me", since, until)
	if len(entries) != 1 || !slices.Contains(entries[0].Reasons, "current description mentions you") {
		t.Fatalf("entries = %+v", entries)
	}
	if Mentions(`<body><a href="https://app.asana.com/me">Austin</a></body>`, "me") || Mentions(`<a data-asana-type="task" data-asana-gid="me">Austin</a>`, "me") {
		t.Fatal("names and task links must not count as user mentions")
	}
}

func TestFetchKeepsPartialResultsAndSorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.URL.Path {
		case "/users/me":
			data = asana.User{GID: "me"}
		case "/workspaces/w/tasks/search":
			data = []asana.Task{{GID: "1", Name: "reassigned"}, {GID: "2", Name: "assigned", Assignee: &asana.Ref{GID: "me"}}, {GID: "3"}}
		case "/tasks/1/stories":
			data = []asana.Story{story("own", "me", "comment_added")}
		case "/tasks/2/stories":
			s := story("related", "other", "due_date_changed")
			s.CreatedAt = since.Add(2 * time.Hour).Format(time.RFC3339)
			data = []asana.Story{s}
		case "/tasks/3/stories":
			w.WriteHeader(http.StatusForbidden)
			return
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer srv.Close()
	c := asana.New("token")
	c.BaseURL = srv.URL
	var snaps []Report
	r, err := Fetch(context.Background(), c, "w", since, until, func(r Report) { snaps = append(snaps, r) })
	if err != nil || r.Total != 3 || r.Scanned != 2 || len(r.Entries) != 2 || r.Entries[0].Story.GID != "related" || !strings.Contains(r.Warning, "1 ticket feeds failed") {
		t.Fatalf("report = %+v, err = %v", r, err)
	}
	// One snapshot for the search page, then one per ticket feed.
	if len(snaps) != 4 || snaps[0].Total != 3 || snaps[0].Scanned != 0 || len(snaps[3].Entries) != 2 || snaps[3].Entries[0].Story.GID != "related" {
		t.Fatalf("snapshots = %+v", snaps)
	}
}

func TestPaidSearchErrorIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me" {
			_, _ = w.Write([]byte(`{"data":{"gid":"me"}}`))
			return
		}
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()
	c := asana.New("token")
	c.BaseURL = srv.URL
	_, err := Fetch(context.Background(), c, "w", since, until, nil)
	if err == nil || !strings.Contains(err.Error(), "paid workspace search API") {
		t.Fatalf("err = %v", err)
	}
}
