package tui

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/ticket"
)

type commentOp int

const commentReact commentOp = iota

// quickReactions are the emoji the react picker offers on every comment.
var quickReactions = []struct{ emoji, name string }{
	{"👍", "thumbs up"}, {"❤️", "heart"}, {"🎉", "party"}, {"😂", "laugh"},
	{"👀", "eyes"}, {"🙏", "thanks"}, {"✅", "done"},
}

// openCommentMenu lists what can be done to comment c of the ticket with gid.
func (m *Model) openCommentMenu(gid string, c asana.Story) {
	p := newPicker(pickValue(func(op commentOp) tea.Cmd {
		m.modal = nil
		switch op {
		case commentReact:
			m.openReactPicker(gid, c)
		}
		return nil
	}), "Comment by "+ticket.Author(c), []pickItem{
		{Label: "React", Key: "r", Value: commentReact},
	})
	p.keySelect = true
	m.modal = p
}

// openReactPicker offers the quick reactions, then any other emoji already on
// comment c. Picking one the user reacted with removes it.
func (m *Model) openReactPicker(gid string, c asana.Story) {
	mine := make(map[string]bool)
	for _, r := range c.Reactions {
		mine[r.Emoji] = mine[r.Emoji] || r.Reacted
	}
	var items []pickItem
	add := func(emoji, name string) {
		label := ticket.OneLine(emoji)
		if name != "" {
			label += "  " + name
		}
		item := pickItem{Label: label, Value: emoji}
		if len(items) < 9 {
			item.Key = strconv.Itoa(len(items) + 1)
		}
		if mine[emoji] {
			item.Hint = "remove"
		}
		items = append(items, item)
	}
	quick := make(map[string]bool, len(quickReactions))
	for _, q := range quickReactions {
		add(q.emoji, q.name)
		quick[q.emoji] = true
	}
	for _, r := range c.Reactions {
		if r.Count > 0 && !quick[r.Emoji] {
			add(r.Emoji, "")
		}
	}
	p := newPicker(pickValue(func(emoji string) tea.Cmd {
		m.modal = nil
		svc, story, reacted := m.writer(), c.GID, !mine[emoji]
		what := "react " + ticket.OneLine(emoji)
		if !reacted {
			what = "remove " + ticket.OneLine(emoji)
		}
		m.status = what + "…"
		return saveEdit(gid, what, func(ctx context.Context) error {
			return svc.React(ctx, story, emoji, reacted)
		})
	}), "React to "+ticket.Author(c)+"'s comment", items)
	p.keySelect = true
	m.modal = p
}
