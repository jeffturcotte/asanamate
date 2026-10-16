package action

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/conversations"
)

// ConversationCommand prepares a private ticket/selection snapshot and a command
// for the existing foreground handoff. The caller supplies terminal streams, as
// with Command. The helper runs in asanamate's cwd, without ticket repo/branch
// steps; native revalidation, cwd recovery, and resumption belong to the opener.
func ConversationCommand(stateDir, command string, c Context, selected conversations.Record) (*exec.Cmd, error) {
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("conversations.open is not configured")
	}
	if err := selected.Validate(c.Ticket.GID); err != nil {
		return nil, err
	}
	stateDir, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, err
	}
	files, err := WriteFiles(stateDir, c.Ticket)
	if err != nil {
		return nil, err
	}
	request := conversations.OpenRequest{
		Version: conversations.ProtocolVersion, Operation: "open",
		Asana: conversations.Asana{TaskGID: c.Ticket.GID}, Conversation: selected,
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err == nil {
		files.Conversation = filepath.Join(filepath.Dir(files.JSON), "conversation.json")
		err = os.WriteFile(files.Conversation, data, 0o600)
	}
	if err != nil {
		os.RemoveAll(filepath.Dir(files.JSON))
		return nil, err
	}
	c.Files, c.Conversation = files, &selected
	c.Repo, c.Worktree = "", ""
	return Command(config.Action{Command: command, Mode: config.ModeForeground}, c), nil
}
