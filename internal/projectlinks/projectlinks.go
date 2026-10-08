// Package projectlinks calls an optional command provider for the links an
// Asana project has in another system, such as its git repositories.
package projectlinks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sadmachine/asanamate/internal/jsoncmd"
)

// Link types asanamate reads itself. Providers may return other types; they
// pass through to other providers, such as time tracking, unread.
const (
	// Git is a git remote URL.
	Git = "git"
	// HarvestProject is a numeric Harvest project ID.
	HarvestProject = "harvest_project"
)

// Link is one value of an Asana project in another system.
type Link struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Asana identifies the project whose links are requested.
type Asana struct {
	ProjectGID string `json:"project_gid"`
}

// ProtocolVersion is the project links protocol version every request
// carries. It changes only when existing providers would break; new optional
// fields keep it, so providers must ignore request fields they do not know.
const ProtocolVersion = 1

// Request is sent as JSON on provider stdin.
type Request struct {
	Version   int    `json:"version"`
	Operation string `json:"operation"`
	Asana     Asana  `json:"asana"`
}

// Provider executes the configured command. An empty Command is off and
// knows no links.
type Provider struct{ Command string }

// Links returns the project's links, best match first. An empty list means
// the provider knows none.
func (p Provider) Links(ctx context.Context, projectGID string) ([]Link, error) {
	if p.Command == "" {
		return nil, nil
	}
	if projectGID == "" {
		return nil, errors.New("project links need an Asana project")
	}
	out, err := jsoncmd.Run(ctx, "project links", p.Command, Request{Version: ProtocolVersion, Operation: "links", Asana: Asana{ProjectGID: projectGID}})
	if err != nil {
		return nil, err
	}
	var response struct {
		Links []Link `json:"links"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, fmt.Errorf("project links: %w", err)
	}
	for _, link := range response.Links {
		if link.Type == "" || link.Value == "" {
			return nil, errors.New("project links need a type and value on every link")
		}
	}
	return response.Links, nil
}

// Values returns the values of links of type typ, in order.
func Values(links []Link, typ string) []string {
	var values []string
	for _, link := range links {
		if link.Type == typ {
			values = append(values, link.Value)
		}
	}
	return values
}
