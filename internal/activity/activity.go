// Package activity assembles API-backed task activity without persisting it.
package activity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"

	"github.com/sadmachine/asanamate/internal/asana"
)

// Entry is one story with the task and relationship needed to explain it.
type Entry struct {
	Task      asana.Task
	Story     asana.Story
	At        time.Time
	Mine      bool
	Automatic bool
	Reasons   []string
}

// Report includes scan coverage. Entries can be useful even when Warning
// reports failed searches, inaccessible tasks, or a request deadline.
type Report struct {
	Entries []Entry
	Since   time.Time
	Until   time.Time
	Scanned int
	Total   int
	Warning string
}

// Fetch scans recently changed tasks across the workspace, including tasks
// no longer assigned to the user. Ticket feeds load while search pages are
// still arriving, and progress, when non-nil, receives a sorted snapshot
// after each search page and ticket feed. Calls are serialized; progress must
// not block. Bounded concurrency limits API pressure.
func Fetch(ctx context.Context, c *asana.Client, workspace string, since, until time.Time, progress func(Report)) (Report, error) {
	r := Report{Since: since, Until: until}
	me, err := c.Me(ctx)
	if err != nil {
		return r, err
	}
	var mu sync.Mutex
	var failed int
	var firstErr error
	// emit runs with mu held.
	emit := func() {
		if progress != nil {
			sortEntries(r.Entries)
			snap := r
			snap.Entries = slices.Clone(r.Entries)
			progress(snap)
		}
	}
	g := new(errgroup.Group)
	g.SetLimit(6)
	searchErr := c.RecentTasks(ctx, workspace, since, func(tasks []asana.Task) {
		mu.Lock()
		r.Total += len(tasks)
		emit()
		mu.Unlock()
		for _, t := range tasks {
			if ctx.Err() != nil {
				return
			}
			g.Go(func() error {
				stories, err := c.Stories(ctx, t.GID, since)
				var entries []Entry
				if err == nil {
					entries = Classify(t, stories, me.GID, since, until)
				}
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failed++
					if firstErr == nil {
						firstErr = err
					}
				} else {
					r.Scanned++
					r.Entries = append(r.Entries, entries...)
				}
				emit()
				return nil
			})
		}
	})
	_ = g.Wait()
	if r.Total == 0 && searchErr != nil {
		var apiErr *asana.APIError
		if errors.As(searchErr, &apiErr) && apiErr.Status == 402 {
			return r, fmt.Errorf("Activity requires Asana's paid workspace search API: %w", searchErr)
		}
		return r, searchErr
	}
	var warnings []string
	if searchErr != nil {
		warnings = append(warnings, "task search incomplete: "+searchErr.Error())
	}
	if failed > 0 {
		warnings = append(warnings, fmt.Sprintf("%d ticket feeds failed: %v", failed, firstErr))
	}
	if err := ctx.Err(); err != nil {
		warnings = append(warnings, err.Error())
	}
	r.Warning = strings.Join(warnings, "; ")
	sortEntries(r.Entries)
	return r, nil
}

// sortEntries orders entries newest first.
func sortEntries(entries []Entry) {
	slices.SortFunc(entries, func(a, b Entry) int {
		if n := b.At.Compare(a.At); n != 0 {
			return n
		}
		return strings.Compare(a.Story.GID, b.Story.GID)
	})
}

// Classify keeps own actions and directly related activity. Current task
// ownership and description mentions are deliberately labelled as current
// context: stories do not provide an exhaustive historical ownership record.
func Classify(t asana.Task, stories []asana.Story, user string, since, until time.Time) []Entry {
	assigned := t.Assignee != nil && t.Assignee.GID == user
	description := Mentions(t.HTMLNotes, user)
	seen := map[string]bool{}
	var entries []Entry
	for _, s := range stories {
		at, err := time.Parse(time.RFC3339Nano, s.CreatedAt)
		if err != nil || at.Before(since) || at.After(until) || seen[s.GID] {
			continue
		}
		seen[s.GID] = true
		e := Entry{Task: t, Story: s, At: at, Automatic: automatic(s)}
		e.Mine = !e.Automatic && s.CreatedBy != nil && s.CreatedBy.GID == user
		if s.Assignee != nil && s.Assignee.GID == user {
			e.Reasons = append(e.Reasons, "assigned to you")
		}
		if Mentions(s.HTMLText, user) {
			e.Reasons = append(e.Reasons, "comment mentions you")
		}
		if assigned {
			e.Reasons = append(e.Reasons, "currently assigned to you")
		}
		if description {
			e.Reasons = append(e.Reasons, "current description mentions you")
		}
		if e.Mine || len(e.Reasons) > 0 {
			entries = append(entries, e)
		}
	}
	return entries
}

func automatic(s asana.Story) bool {
	if s.CreatedBy == nil {
		return true
	}
	switch s.ResourceSubtype {
	case "due_today", "due_soon", "overdue", "mentioned_comment_reminder_story", "personal_comment_reminder_story":
		return true
	}
	return strings.HasSuffix(s.ResourceSubtype, "_reminder_story")
}

// Mentions checks typed Asana user links, not names or arbitrary URLs.
func Mentions(fragment, user string) bool {
	z := html.NewTokenizer(strings.NewReader(fragment))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data != "a" {
				continue
			}
			var kind, gid string
			for _, a := range t.Attr {
				switch a.Key {
				case "data-asana-type":
					kind = a.Val
				case "data-asana-gid":
					gid = a.Val
				}
			}
			if kind == "user" && gid == user && user != "" {
				return true
			}
		}
	}
}
