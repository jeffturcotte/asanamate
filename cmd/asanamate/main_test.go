package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/writeback"
)

// fakeWriter is a writeback service backed by a fake Asana that creates task
// 9 and records each request body's data.
func fakeWriter(t *testing.T, bodies *[]map[string]any) func(bool) (writeback.Service, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Data map[string]any `json:"data"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		*bodies = append(*bodies, body.Data)
		io.WriteString(w, `{"data":{"gid":"9","name":"New"}}`)
	}))
	t.Cleanup(srv.Close)
	c := asana.New("tok")
	c.BaseURL = srv.URL
	return func(bool) (writeback.Service, error) {
		return writeback.Service{Client: c, Workspace: "w"}, nil
	}
}

func TestCreatePrintsOnlyTheGID(t *testing.T) {
	var bodies []map[string]any
	var out strings.Builder
	err := runCreate([]string{"--yes", "--notes", "-", "New"}, strings.NewReader("hi"), &out, fakeWriter(t, &bodies))
	if err != nil || out.String() != "9\n" {
		t.Fatalf("out = %q, err = %v", out.String(), err)
	}
	if len(bodies) != 1 || bodies[0]["assignee"] != "me" || bodies[0]["html_notes"] != "<body>hi</body>" {
		t.Fatalf("bodies = %v", bodies)
	}
}

func TestCreateRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"a", "b"},
		{"--project", "web", "New"},
		{"--parent", "1", "--project", "2", "New"},
	} {
		var bodies []map[string]any
		var out strings.Builder
		if err := runCreate(args, strings.NewReader(""), &out, fakeWriter(t, &bodies)); err == nil || out.Len() > 0 || len(bodies) > 0 {
			t.Errorf("%q: err = %v, out = %q, bodies = %v", args, err, out.String(), bodies)
		}
	}
}

func TestCheckFeedsRunsEachFeed(t *testing.T) {
	var out bytes.Buffer
	checkFeeds(context.Background(), &out, []string{
		`printf '%s' '{"source":"ci","summary":"1 failed","items":[{"time":"2026-10-12T09:00:00Z","text":"build"}]}'`,
		"exit 3",
	})
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "ok: ") || !strings.HasSuffix(lines[0], "ci, 1 item today, 1 failed") || !strings.Contains(lines[1], "warning: ") {
		t.Fatalf("doctor feeds:\n%s", out.String())
	}
	out.Reset()
	checkFeeds(context.Background(), &out, nil)
	if !strings.Contains(out.String(), "none: set today.feeds") {
		t.Fatalf("no feeds = %q", out.String())
	}
}
