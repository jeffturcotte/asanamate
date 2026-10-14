// Package timetracking calls an optional command provider for a time-entry
// form, completed entries, and ticket totals.
package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sadmachine/asanamate/internal/form"
	"github.com/sadmachine/asanamate/internal/jsoncmd"
	"github.com/sadmachine/asanamate/internal/projectlinks"
)

// Asana identifies the selected ticket and its chosen Asana project.
type Asana struct {
	TaskGID    string `json:"task_gid"`
	ProjectGID string `json:"project_gid"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

// ProtocolVersion is the time provider protocol version every request
// carries. It changes only when existing providers would break; new optional
// fields keep it, so providers must ignore request fields they do not know.
const ProtocolVersion = 1

// Request is sent as JSON on provider stdin. Links, on form requests, are the
// chosen Asana project's links from the project links provider.
type Request struct {
	Version   int                 `json:"version"`
	Operation string              `json:"operation"`
	Values    map[string]string   `json:"values,omitempty"`
	Asana     *Asana              `json:"asana,omitempty"`
	Links     []projectlinks.Link `json:"links,omitempty"`
}

// Provider executes the configured command. Data goes through stdin, never
// into user-authored shell text.
type Provider struct{ Command string }

// Summary contains all-time, unrounded time from stopped entries visible to
// the authenticated account, across projects for one exact ticket.
type Summary struct {
	TotalSeconds int64 `json:"total_seconds"`
	// TodaySeconds is the subset logged on today's local calendar date.
	// Nil means the provider does not supply daily totals.
	TodaySeconds *int64 `json:"today_seconds,omitempty"`
}

// ErrUnsupported means the provider does not offer ticket summaries.
var ErrUnsupported = errors.New("time summary unsupported")

// Summary fetches a ticket total. Missing totals must not become zero.
func (p Provider) Summary(ctx context.Context, asana Asana) (Summary, error) {
	if asana.TaskGID == "" {
		return Summary{}, errors.New("time summary needs an Asana task")
	}
	out, err := p.call(ctx, Request{Operation: "summary", Asana: &asana})
	if err != nil {
		return Summary{}, err
	}
	var response struct {
		TotalSeconds *int64 `json:"total_seconds"`
		TodaySeconds *int64 `json:"today_seconds"`
		Unsupported  bool   `json:"unsupported"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return Summary{}, fmt.Errorf("time summary: %w", err)
	}
	if response.Unsupported {
		return Summary{}, ErrUnsupported
	}
	if response.TotalSeconds == nil || *response.TotalSeconds < 0 {
		return Summary{}, errors.New("time summary needs nonnegative integer total_seconds")
	}
	if response.TodaySeconds != nil && (*response.TodaySeconds < 0 || *response.TodaySeconds > *response.TotalSeconds) {
		return Summary{}, errors.New("time summary needs nonnegative integer today_seconds no greater than total_seconds")
	}
	return Summary{TotalSeconds: *response.TotalSeconds, TodaySeconds: response.TodaySeconds}, nil
}

func (p Provider) call(ctx context.Context, req Request) ([]byte, error) {
	req.Version = ProtocolVersion
	return jsoncmd.Run(ctx, "time provider", p.Command, req)
}

// Form fetches fresh controls and choices for an entry in asana's project;
// links may suggest select defaults.
func (p Provider) Form(ctx context.Context, asana Asana, links []projectlinks.Link) (form.Spec, error) {
	out, err := p.call(ctx, Request{Operation: "form", Asana: &asana, Links: links})
	if err != nil {
		return form.Spec{}, err
	}
	var spec form.Spec
	if err := json.Unmarshal(out, &spec); err != nil {
		return form.Spec{}, fmt.Errorf("time form: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return form.Spec{}, err
	}
	hours := 0
	for _, field := range spec.Fields {
		if field.Type == form.Hours {
			hours++
		}
	}
	if hours != 1 {
		return form.Spec{}, errors.New("time form needs exactly one hours field")
	}
	return spec, nil
}

// Log sends one completed entry. Never retry a failed call: provider may have
// created an entry before reporting its error.
func (p Provider) Log(ctx context.Context, spec form.Spec, values map[string]string, asana Asana) error {
	if err := spec.ValidateValues(values); err != nil {
		return err
	}
	_, err := p.call(ctx, Request{Operation: "log", Values: values, Asana: &asana})
	return err
}
