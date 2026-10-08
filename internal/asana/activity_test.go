package asana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRecentTasksUsesSearchCursorAndDeduplicatesBoundary(t *testing.T) {
	var first []Task
	for i := range 100 {
		first = append(first, Task{GID: fmt.Sprint(i + 1), CreatedAt: time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339)})
	}
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/workspaces/77/tasks/search" || q.Get("sort_by") != "created_at" || q.Get("sort_ascending") != "true" || q.Get("offset") != "" || q.Get("modified_at.after") == "" {
			t.Errorf("unexpected search: %s", r.URL)
		}
		page := first
		if calls == 2 {
			cursor, err := time.Parse(time.RFC3339Nano, q.Get("created_at.after"))
			boundary, _ := time.Parse(time.RFC3339, first[99].CreatedAt)
			if err != nil || !cursor.Before(boundary) {
				t.Errorf("boundary overlap missing: %s", r.URL)
			}
			page = []Task{first[99], {GID: "101"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": page})
	})
	var tasks []Task
	pages := 0
	err := c.RecentTasks(ctx, "77", time.Now().Add(-24*time.Hour), func(page []Task) {
		pages++
		tasks = append(tasks, page...)
	})
	if err != nil || len(tasks) != 101 || calls != 2 || pages != 2 {
		t.Fatalf("len = %d, calls = %d, pages = %d, err = %v", len(tasks), calls, pages, err)
	}
}

func TestRecentTasksReportsStalledSearchInsteadOfLosingTies(t *testing.T) {
	var page []Task
	for i := range 100 {
		page = append(page, Task{GID: fmt.Sprint(i), CreatedAt: "2026-01-01T00:00:00Z"})
	}
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"data": page})
	})
	var tasks []Task
	err := c.RecentTasks(ctx, "77", time.Now(), func(page []Task) { tasks = append(tasks, page...) })
	if len(tasks) != 100 || calls != 2 || err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("len = %d, calls = %d, err = %v", len(tasks), calls, err)
	}
}

func TestStoriesLoadsSystemActivityAndFollowsPagination(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("created_after") == "" || !strings.Contains(q.Get("opt_fields"), "resource_subtype") || q.Get("resource_subtype") != "" {
			t.Errorf("unexpected query %s", r.URL)
		}
		if q.Get("offset") == "" {
			_, _ = w.Write([]byte(`{"data":[{"gid":"1","type":"system","resource_subtype":"assigned","assignee":{"gid":"me"}}],"next_page":{"offset":"next"}}`))
		} else {
			_, _ = w.Write([]byte(`{"data":[{"gid":"2","type":"comment","resource_subtype":"comment_added"}]}`))
		}
	})
	stories, err := c.Stories(ctx, "ticket", time.Now())
	if err != nil || len(stories) != 2 || stories[0].Assignee.GID != "me" || stories[1].Type != "comment" {
		t.Fatalf("stories = %+v, err = %v", stories, err)
	}
}
