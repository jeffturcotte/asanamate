package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/drafts"
	"github.com/sadmachine/asanamate/internal/ticket"
	"github.com/sadmachine/asanamate/internal/writeback"
)

// commentComposer owns the editor until a draft save, discard, or successful post.
// Keeping the editor while a write runs prevents failures from losing input.
type commentComposer struct {
	gid       string
	storyGID  string // empty when composing a new comment
	original  string // initial Markdown, used to close an unchanged edit quietly
	box       *inputBox
	store     *drafts.CommentStore
	images    map[string][]byte
	uncertain bool
	busy      bool
	confirm   *picker
	quitting  bool
}

type commentOperation int

const (
	commentSave commentOperation = iota
	commentDiscard
	commentPost
)

type commentDoneMsg struct {
	composer *commentComposer
	op       commentOperation
	draft    drafts.Comment
	saved    bool
	err      error
	cleanup  error // a successful post whose draft could not be removed
}

func (m *Model) openCommentComposer(t asana.Task) {
	m.openCommentInput(t.GID, "", "Comment on "+ticket.Clean(t.Name), "")
}

func (m *Model) openCommentEditor(gid string, story asana.Story) {
	m.openCommentInput(gid, story.GID, "Edit comment by "+ticket.Author(story), ticket.CommentMarkdown(story))
}

func (m *Model) openCommentInput(gid, storyGID, title, initial string) {
	b := newInputBox(title, "comment text")
	b.area.DynamicHeight = true
	b.area.MinHeight = 10
	b.area.MaxContentHeight = math.MaxInt
	b.images = true
	b.area.SetValue(initial)
	c := &commentComposer{gid: gid, storyGID: storyGID, original: initial, box: b}
	identity := gid
	if storyGID != "" {
		identity = "story:" + storyGID
	}
	store, draft, err := drafts.OpenComment(m.deps.StateDir, m.deps.Config.Workspace, identity)
	if err != nil {
		b.err = "loading draft: " + err.Error()
	} else {
		c.store, c.images, c.uncertain = store, draft.Images, draft.Uncertain
		if draft.Text != "" {
			b.area.SetValue(draft.Text)
			b.title += " (draft)"
		}
		c.deliveryHint()
	}
	b.onSubmit = func(string) tea.Cmd { return m.postComment() }
	b.onSaveDraft = func() tea.Cmd {
		if strings.TrimSpace(b.area.Value()) == "" {
			return m.leaveComment(false)
		}
		return m.runComment(commentSave)
	}
	m.composer, m.input, m.edit = c, b, nil
}

func (c *commentComposer) deliveryHint() {
	c.box.help = ""
	if c.storyGID != "" {
		c.box.help = "Existing mentions and images use asana:<gid> links. Keep these links to preserve them."
	}
	if c.uncertain {
		c.box.help = "Delivery uncertain. Check the ticket's comments before submitting again."
	}
}

func (m *Model) updateComment(msg tea.Msg) tea.Cmd {
	c := m.composer
	if c.busy {
		return nil
	}
	if c.confirm != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			res, cmd := c.confirm.update(key)
			if res.cancelled {
				c.confirm, c.quitting = nil, false
			} else if res.done {
				return tea.Batch(cmd, c.confirm.onPick(res))
			}
			return cmd
		}
		return nil
	}
	res, cmd := c.box.update(msg)
	if res.cancelled {
		return m.leaveComment(false)
	}
	if res.done {
		return m.postComment()
	}
	return cmd
}

func (m *Model) leaveComment(quit bool) tea.Cmd {
	c := m.composer
	if c.busy {
		return nil
	}
	c.quitting = quit
	if strings.TrimSpace(c.box.area.Value()) == "" || (c.storyGID != "" && c.box.area.Value() == c.original) {
		return m.runComment(commentDiscard)
	}
	c.confirm = newPicker(pickValue(func(choice string) tea.Cmd {
		c.confirm = nil
		switch choice {
		case "keep":
			c.quitting = false
			return nil
		case "save":
			return m.runComment(commentSave)
		default:
			return m.runComment(commentDiscard)
		}
	}), "Save comment as draft?", []pickItem{
		{Label: "Save draft", Value: "save"},
		{Label: "Discard", Value: "discard"},
		{Label: "Keep editing", Value: "keep"},
	})
	// Confirmations never begin in search mode, even with picker.type_first.
	c.confirm.stopSearch()
	return nil
}

func (m *Model) postComment() tea.Cmd {
	c := m.composer
	if strings.TrimSpace(c.box.area.Value()) == "" {
		if c.storyGID != "" {
			c.box.err = "comment text is empty"
			return nil
		}
		return m.leaveComment(false)
	}
	if c.uncertain {
		retry := "Post again"
		if c.storyGID != "" {
			retry = "Save again"
		}
		c.confirm = newPicker(pickValue(func(post bool) tea.Cmd {
			c.confirm = nil
			if post {
				return m.runComment(commentPost)
			}
			return nil
		}), "Delivery uncertain: check comments before retrying", []pickItem{
			{Label: "Keep editing", Value: false},
			{Label: retry, Value: true},
		})
		c.confirm.stopSearch()
		return nil
	}
	return m.runComment(commentPost)
}

// runComment does disk and network work off the UI loop. Saving before posting
// also protects against a crash while Asana is receiving the comment.
func (m *Model) runComment(op commentOperation) tea.Cmd {
	c := m.composer
	if c.store == nil {
		if op == commentDiscard {
			return m.commentDone(commentDoneMsg{composer: c, op: op})
		}
		c.box.err = "draft storage is unavailable; keep this text and fix the draft file before saving or posting"
		return nil
	}
	c.busy = true
	c.box.err, c.box.help = "", "Saving draft…"
	if op == commentDiscard {
		c.box.help = "Discarding draft…"
	} else if op == commentPost {
		c.box.help = "Posting comment…"
		if c.storyGID != "" {
			c.box.help = "Updating comment…"
		}
	}
	draft := drafts.Comment{Text: c.box.area.Value(), Images: maps.Clone(c.images), Uncertain: c.uncertain}
	svc, gid := m.writer(), c.gid
	return request(func(ctx context.Context) tea.Msg {
		msg := commentDoneMsg{composer: c, op: op}
		if op == commentDiscard {
			msg.err = c.store.Remove()
			return msg
		}
		images := map[string][]byte{}
		for _, path := range writeback.LocalImages(draft.Text) {
			data, saved := draft.Images[path]
			if !saved {
				var err error
				data, err = writeback.ReadImage(path)
				if err != nil {
					msg.err = fmt.Errorf("comment image %s: %w", path, err)
					return msg
				}
			}
			images[path] = data
		}
		draft.Images = images
		if op == commentPost {
			draft.Uncertain = true
		}
		if msg.err = c.store.Save(draft); msg.err != nil {
			return msg
		}
		msg.saved, msg.draft = true, draft
		if op == commentSave {
			return msg
		}
		svc.Images = images
		if c.storyGID != "" {
			msg.err = svc.EditComment(ctx, c.storyGID, draft.Text)
		} else {
			msg.err = svc.Comment(ctx, gid, draft.Text)
		}
		if msg.err != nil {
			draft.Uncertain = commentDeliveryUncertain(msg.err)
			msg.draft = draft
			if err := c.store.Save(draft); err != nil {
				msg.err = fmt.Errorf("%w; saving recovery draft: %v", msg.err, err)
			}
		} else {
			msg.cleanup = c.store.Remove()
		}
		return msg
	})
}

func commentDeliveryUncertain(err error) bool {
	var networkError net.Error
	var apiError *asana.APIError
	return errors.As(err, &networkError) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		(errors.As(err, &apiError) && apiError.Status >= 500)
}

func (m *Model) commentDone(msg commentDoneMsg) tea.Cmd {
	c := m.composer
	if c == nil || c != msg.composer {
		return nil
	}
	c.busy = false
	if msg.saved {
		c.images, c.uncertain = msg.draft.Images, msg.draft.Uncertain
	}
	c.deliveryHint()
	if msg.err != nil {
		// Empty input has nothing to protect. Close even if an old draft
		// cannot be removed, preserving that file and reporting the failure.
		if msg.op == commentDiscard && strings.TrimSpace(c.box.area.Value()) == "" {
			m.composer, m.input = nil, nil
			m.status = "comment closed; draft retained: " + msg.err.Error()
			if c.quitting {
				m.closeActivity()
				return tea.Quit
			}
			return nil
		}
		c.quitting = false
		c.box.err = msg.err.Error()
		return nil
	}
	m.composer, m.input = nil, nil
	switch msg.op {
	case commentSave:
		m.status = "comment draft saved"
	case commentDiscard:
		m.status = "comment draft discarded"
	case commentPost:
		what := "comment"
		if c.storyGID != "" {
			what = "edit comment"
		}
		cmd := m.editDone(editDoneMsg{gid: c.gid, what: what})
		if msg.cleanup != nil {
			m.status = "comment posted; removing draft failed: " + msg.cleanup.Error()
			if c.storyGID != "" {
				m.status = "comment updated; removing draft failed: " + msg.cleanup.Error()
			}
		}
		return cmd
	}
	if c.quitting {
		m.closeActivity()
		return tea.Quit
	}
	return nil
}
