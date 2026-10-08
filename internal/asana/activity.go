package asana

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// RecentTasks searches all accessible workspace tasks changed since since.
// Search uses creation-time cursors, not offset pagination. Partial results
// accompany errors so callers can disclose incomplete coverage.
func (c *Client) RecentTasks(ctx context.Context, workspace string, since time.Time) ([]Task, error) {
	q := fields("name,assignee.name,html_notes,created_at,modified_at,permalink_url,memberships.project.name")
	q.Set("modified_at.after", since.UTC().Add(-time.Second).Format(time.RFC3339Nano))
	q.Set("sort_by", "created_at")
	q.Set("sort_ascending", "true")
	q.Set("limit", "100")
	var all []Task
	seen := map[string]bool{}
	var cursor time.Time
	for {
		var page struct {
			Data []Task `json:"data"`
		}
		if err := c.do(ctx, http.MethodGet, "/workspaces/"+workspace+"/tasks/search", q, nil, &page); err != nil {
			return all, err
		}
		for _, t := range page.Data {
			if !seen[t.GID] {
				seen[t.GID] = true
				all = append(all, t)
			}
		}
		if len(page.Data) < 100 {
			return all, nil
		}
		last, err := time.Parse(time.RFC3339Nano, page.Data[len(page.Data)-1].CreatedAt)
		if err != nil || !last.After(cursor) {
			return all, fmt.Errorf("task search cursor did not advance; results are incomplete")
		}
		cursor = last
		// Keep boundary tasks in the next page and de-duplicate by gid. This
		// avoids silently losing tasks sharing the boundary creation time.
		q.Set("created_at.after", last.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano))
	}
}

// Stories returns task activity since since, including comments and system
// changes. The caller uses resource_subtype rather than parsing localized text.
func (c *Client) Stories(ctx context.Context, gid string, since time.Time) ([]Story, error) {
	q := fields("type,resource_subtype,created_at,created_by.name,text,html_text,assignee.name,old_dates,new_dates")
	q.Set("created_after", since.UTC().Format(time.RFC3339Nano))
	return getAll[Story](ctx, c, "/tasks/"+gid+"/stories", q)
}
