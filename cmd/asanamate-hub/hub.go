package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/sadmachine/asanamate/internal/projectlinks"
)

// Hub credential environment variables: a Hub service account's OAuth2
// client credentials.
const (
	HubClientIDEnv     = "HUB_CLIENT_ID"
	HubClientSecretEnv = "HUB_CLIENT_SECRET"
)

// hubURL is imarc's Hub.
const hubURL = "https://hub.imarc.net"

// Hub link type names read from projects.
const (
	hubAsanaProject   = "Asana Project"
	hubGitRepository  = "Git Repository"
	hubHarvestProject = "Harvest Project"
)

type hubLink struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type hubProject struct {
	Links         []hubLink `json:"links"`
	ContractLinks []hubLink `json:"contract_links"`
}

// checkHub reports whether Hub credentials are set, without calling Hub.
func checkHub() error {
	var missing []string
	for _, key := range []string{HubClientIDEnv, HubClientSecretEnv} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("export %s from a Hub service account", strings.Join(missing, " and "))
	}
	return nil
}

// runHub serves one request of the project links protocol from Hub,
// read-only.
func runHub(ctx context.Context, in io.Reader, out io.Writer, baseURL, clientID, clientSecret string, now time.Time) error {
	var req projectlinks.Request
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return fmt.Errorf("project links request: %w", err)
	}
	if req.Operation != "links" {
		return fmt.Errorf("unknown project links operation %q", req.Operation)
	}
	gid := req.Asana.ProjectGID
	if gid == "" || strings.Trim(gid, "0123456789") != "" {
		return errors.New("Hub lookup needs a numeric Asana project GID")
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return checkHub()
	}
	base := strings.TrimRight(baseURL, "/")
	token, err := hubToken(ctx, base, clientID, clientSecret)
	if err != nil {
		return err
	}
	query := url.Values{"type": {hubAsanaProject}, "term": {gid}, "limit": {"50"}}
	body, err := hubCall(ctx, http.MethodGet, base+"/api/projects/find?"+query.Encode(), "Bearer "+token, nil)
	if err != nil {
		return err
	}
	var found struct {
		Data []struct {
			Project hubProject `json:"project"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return fmt.Errorf("Hub projects: %w", err)
	}
	today := now.Format(time.DateOnly)
	links := []projectlinks.Link{}
	for _, result := range found.Data {
		project := result.Project
		// Find also returns partial and fuzzy matches; keep exact Asana links.
		if !slices.ContainsFunc(project.Links, func(l hubLink) bool { return l.Type == hubAsanaProject && l.Value == gid && l.activeOn(today) }) {
			continue
		}
		// Contract links first: the current contract's Harvest project is the
		// one to log time to.
		for _, l := range append(slices.Clone(project.ContractLinks), project.Links...) {
			if !l.activeOn(today) {
				continue
			}
			link := projectlinks.Link{Value: strings.TrimSpace(l.Value)}
			switch l.Type {
			case hubGitRepository:
				link.Type = projectlinks.Git
			case hubHarvestProject:
				link.Type = projectlinks.HarvestProject
			default:
				continue
			}
			if link.Value != "" && !slices.Contains(links, link) {
				links = append(links, link)
			}
		}
	}
	return json.NewEncoder(out).Encode(map[string][]projectlinks.Link{"links": links})
}

// activeOn reports whether today, a YYYY-MM-DD date, is within the link's
// optional dates.
func (l hubLink) activeOn(today string) bool {
	return (l.StartDate == "" || l.StartDate <= today) && (l.EndDate == "" || l.EndDate >= today)
}

// hubToken exchanges client credentials for a bearer token.
func hubToken(ctx context.Context, base, clientID, clientSecret string) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {clientSecret}}
	body, err := hubCall(ctx, http.MethodPost, base+"/oauth/token", "", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil || token.AccessToken == "" {
		return "", errors.New("Hub token: response has no access_token")
	}
	return token.AccessToken, nil
}

func hubCall(ctx context.Context, method, endpoint, authorization string, form io.Reader) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, form)
	if err != nil {
		return nil, err
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "asanamate")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Hub: %w", err)
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("Hub response: %w", err)
	}
	if len(result) > 1<<20 {
		return nil, errors.New("Hub response exceeds 1 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Hub: %s: %s", response.Status, strings.TrimSpace(string(result)))
	}
	return result, nil
}
