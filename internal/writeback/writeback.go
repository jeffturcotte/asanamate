// Package writeback performs confirmed updates to Asana tasks.
package writeback

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/prompt"
	"github.com/sadmachine/asanamate/internal/ticket"
)

var (
	// ErrNoTTY means confirmation was required but no terminal was available.
	ErrNoTTY = errors.New("confirmation required but no terminal is available; pass --yes or set confirm_writes = false")
	// ErrDeclined means the user answered no.
	ErrDeclined = errors.New("cancelled")
)

// Confirmer asks the user to approve a write.
type Confirmer func(prompt string) (bool, error)

// NeedsConfirm resolves whether to confirm: --yes wins, then
// ASANAMATE_CONFIRM_WRITES ("1"/"0"), then the config default.
func NeedsConfirm(yes bool, envValue string, configDefault bool) bool {
	if yes {
		return false
	}
	switch envValue {
	case "0":
		return false
	case "1":
		return true
	}
	return configDefault
}

// TTYConfirm asks on /dev/tty, so it works even when stdin is a pipe.
func TTYConfirm(question string) (bool, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, ErrNoTTY
	}
	defer tty.Close()
	return prompt.Confirm(bufio.NewReader(tty), tty, question)
}

// Service writes to Asana, asking Confirm first when it is set.
type Service struct {
	Client  *asana.Client
	Confirm Confirmer
	// Workspace is where Comment looks up @mentioned users when Users is nil.
	Workspace string
	// Users are the people a comment can @mention; nil loads them on demand.
	Users []asana.Ref
}

// users returns s.Users, or loads the workspace's users when it is nil.
func (s Service) users(ctx context.Context) ([]asana.Ref, error) {
	if s.Users != nil || s.Workspace == "" {
		return s.Users, nil
	}
	return s.Client.WorkspaceUsers(ctx, s.Workspace)
}

func (s Service) confirm(question string) error {
	if s.Confirm == nil {
		return nil
	}
	ok, err := s.Confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeclined
	}
	return nil
}

// Comment posts Markdown as a rich-text comment on the task. Each @Name matching a
// workspace user becomes a mention that notifies them. Each local image,
// ![alt](path), is attached to the task and shown inline.
func (s Service) Comment(ctx context.Context, gid, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("comment text is empty")
	}
	users := s.Users
	if strings.Contains(text, "@") {
		var err error
		if users, err = s.users(ctx); err != nil {
			return fmt.Errorf("loading people to mention: %w", err)
		}
	}
	paths := LocalImages(text)
	files := make([][]byte, len(paths))
	for i, p := range paths {
		data, err := readImage(p)
		if err != nil {
			return fmt.Errorf("comment image %s: %w", p, err)
		}
		files[i] = data
	}
	html, mentioned := CommentHTML(text, users, nil)
	if s.Confirm != nil {
		t, err := s.Client.Task(ctx, gid)
		if err != nil {
			return err
		}
		question := fmt.Sprintf("Comment on %q?\n%s\n", t.Name, text)
		if len(mentioned) > 0 {
			names := make([]string, len(mentioned))
			for i, u := range mentioned {
				names[i] = ticket.Clean(u.Name)
			}
			question += "Mentions: " + strings.Join(names, ", ") + "\n"
		}
		if len(paths) > 0 {
			question += "Uploads: " + strings.Join(paths, ", ") + "\n"
		}
		if err := s.confirm(question); err != nil {
			return err
		}
	}
	if len(paths) > 0 {
		images := make(map[string]string, len(paths))
		for i, p := range paths {
			a, err := s.Client.UploadAttachment(ctx, gid, filepath.Base(p), files[i])
			if err != nil {
				return fmt.Errorf("uploading %s: %w", p, err)
			}
			images[p] = a.GID
		}
		html, _ = CommentHTML(text, users, images)
	}
	if html == "<body>"+escapeHTML.Replace(text)+"</body>" {
		return s.Client.AddComment(ctx, gid, text)
	}
	return s.Client.AddCommentHTML(ctx, gid, html)
}

// maxImageSize is Asana's attachment size limit.
const maxImageSize = 100 << 20

// readImage reads a comment image path; a leading ~/ is the home directory.
// Only regular files whose content is an image qualify, so a comment cannot
// upload other local files.
func readImage(path string) ([]byte, error) {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, rest)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxImageSize {
		return nil, errors.New("not a regular file of at most 100 MB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(http.DetectContentType(data), "image/") {
		return nil, errors.New("not an image")
	}
	return data, nil
}

// Move puts the task in the named section of one of its projects.
func (s Service) Move(ctx context.Context, gid, sectionName, projectGID string) error {
	t, err := s.Client.Task(ctx, gid)
	if err != nil {
		return err
	}
	project, err := pickProject(t, projectGID)
	if err != nil {
		return err
	}
	sections, err := s.Client.Sections(ctx, project.GID)
	if err != nil {
		return err
	}
	var names []string
	for _, sec := range sections {
		if strings.EqualFold(strings.TrimSpace(sec.Name), strings.TrimSpace(sectionName)) {
			if err := s.confirm(fmt.Sprintf("Move %q to %s / %s?", t.Name, ticket.Clean(project.Name), ticket.Clean(sec.Name))); err != nil {
				return err
			}
			return s.Client.AddToSection(ctx, sec.GID, gid)
		}
		names = append(names, ticket.Clean(sec.Name))
	}
	return fmt.Errorf("no section %q in %s; sections: %s", sectionName, ticket.Clean(project.Name), strings.Join(names, ", "))
}

func pickProject(t asana.Task, projectGID string) (asana.Ref, error) {
	var choices []string
	for _, m := range t.Memberships {
		if m.Project.GID == projectGID {
			return m.Project, nil
		}
		choices = append(choices, fmt.Sprintf("%s (%s)", ticket.Clean(m.Project.Name), m.Project.GID))
	}
	switch {
	case projectGID != "":
		return asana.Ref{}, fmt.Errorf("task is not in project %s", projectGID)
	case len(t.Memberships) == 0:
		return asana.Ref{}, errors.New("task is not in any project")
	case len(t.Memberships) == 1:
		return t.Memberships[0].Project, nil
	}
	return asana.Ref{}, fmt.Errorf("task is in several projects; pass --project with one of: %s", strings.Join(choices, ", "))
}

// SetField sets a custom field from text, as FieldValue reads it; a people
// field takes comma-separated full names of workspace users. An empty value
// clears it. When the task has several fields with that name, projectGID picks
// the one attached to that project.
func (s Service) SetField(ctx context.Context, gid, fieldName, value, projectGID string) error {
	t, err := s.Client.Task(ctx, gid)
	if err != nil {
		return err
	}
	f, err := s.resolveField(ctx, t, fieldName, projectGID)
	if err != nil {
		return err
	}
	v, err := s.fieldValue(ctx, f, value)
	if err != nil {
		return err
	}
	if err := s.confirm(fmt.Sprintf("Set %q on %q to %q?", ticket.FieldName(f), t.Name, value)); err != nil {
		return err
	}
	return s.Client.SetCustomField(ctx, gid, f.GID, v)
}

// resolveField finds the one field to write. Writes never guess between
// same-named fields: they need the project that owns the intended one.
func (s Service) resolveField(ctx context.Context, t asana.Task, name, projectGID string) (asana.CustomField, error) {
	matches := t.FieldsNamed(name)
	switch {
	case len(matches) == 0:
		return asana.CustomField{}, fmt.Errorf("task has no custom field %q", name)
	case len(matches) == 1:
		return matches[0], nil
	case projectGID == "":
		return asana.CustomField{}, fmt.Errorf("task has %d custom fields named %q; pass --project with the gid of the project whose field to set", len(matches), name)
	}
	onProject, err := s.Client.ProjectFieldGIDs(ctx, projectGID)
	if err != nil {
		return asana.CustomField{}, err
	}
	for _, f := range matches {
		if onProject[f.GID] {
			return f, nil
		}
	}
	return asana.CustomField{}, fmt.Errorf("none of the fields named %q is on project %s", name, projectGID)
}

// fieldValue is FieldValue, plus people fields, whose names it resolves to
// workspace users.
func (s Service) fieldValue(ctx context.Context, f asana.CustomField, value string) (any, error) {
	if f.ResourceSubtype != asana.FieldPeople || value == "" {
		return FieldValue(f, value)
	}
	users, err := s.users(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading people: %w", err)
	}
	names, err := splitList(f, value)
	if err != nil {
		return nil, err
	}
	gids := make([]string, 0, len(names))
	for _, name := range names {
		var match []asana.Ref
		for _, u := range users {
			if strings.EqualFold(strings.TrimSpace(u.Name), name) {
				match = append(match, u)
			}
		}
		switch len(match) {
		case 0:
			return nil, fmt.Errorf("%s: no user named %q", ticket.FieldName(f), name)
		case 1:
			gids = appendNew(gids, match[0].GID)
		default:
			return nil, fmt.Errorf("%s: several users are named %q", ticket.FieldName(f), name)
		}
	}
	return gids, nil
}

// splitList splits comma-separated names, refusing empty ones.
func splitList(f asana.CustomField, value string) ([]string, error) {
	names := strings.Split(value, ",")
	for i, n := range names {
		if names[i] = strings.TrimSpace(n); names[i] == "" {
			return nil, fmt.Errorf("%s: empty name in %q", ticket.FieldName(f), value)
		}
	}
	return names, nil
}

func appendNew(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// optionGID finds the enum option named name, ignoring case.
func optionGID(f asana.CustomField, name string) (string, error) {
	var options []string
	for _, o := range f.EnumOptions {
		if strings.EqualFold(o.Name, name) {
			return o.GID, nil
		}
		options = append(options, ticket.Clean(o.Name))
	}
	return "", fmt.Errorf("%s has no option %q; options: %s", ticket.FieldName(f), name, strings.Join(options, ", "))
}

// FieldValue converts typed text to the API value for a text, number, enum,
// multi_enum (comma-separated option names), or date (YYYY-MM-DD) field.
// Empty text is nil, which clears the field.
func FieldValue(f asana.CustomField, value string) (any, error) {
	name := ticket.FieldName(f)
	if value == "" {
		return nil, nil
	}
	switch f.ResourceSubtype {
	case asana.FieldText:
		return value, nil
	case asana.FieldNumber:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("%s needs a number, got %q", name, value)
		}
		return n, nil
	case asana.FieldEnum:
		return optionGID(f, value)
	case asana.FieldMultiEnum:
		names, err := splitList(f, value)
		if err != nil {
			return nil, err
		}
		gids := make([]string, 0, len(names))
		for _, n := range names {
			gid, err := optionGID(f, n)
			if err != nil {
				return nil, err
			}
			gids = appendNew(gids, gid)
		}
		return gids, nil
	case asana.FieldDate:
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return nil, fmt.Errorf("%s needs a date as YYYY-MM-DD, got %q", name, value)
		}
		return map[string]any{"date": value}, nil
	}
	return nil, fmt.Errorf("%s: custom field type %q is not supported", name, f.ResourceSubtype)
}
