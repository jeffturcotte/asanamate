package writeback

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/asana"
)

const taskJSON = `{"data":{"gid":"1","name":"Fix login",
 "memberships":[{"project":{"gid":"p1","name":"Web"}},{"project":{"gid":"p2","name":"API"}}],
 "custom_fields":[
  {"gid":"f1","name":"Branch Name ","resource_subtype":"text"},
  {"gid":"f2","name":"Points","resource_subtype":"number"},
  {"gid":"f3","name":"Stage","resource_subtype":"enum","enum_options":[{"gid":"o1","name":"Review"}]},
  {"gid":"f4","name":"When","resource_subtype":"date"},
  {"gid":"f5","name":"Areas","resource_subtype":"multi_enum","enum_options":[{"gid":"o2","name":"Frontend"},{"gid":"o3","name":"Backend"}]},
  {"gid":"f6","name":"Reviewers","resource_subtype":"people"}]}}`

type write struct {
	method, path string
	body         map[string]any
}

func fake(t *testing.T) (*asana.Client, *[]write) {
	t.Helper()
	var writes []write
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/1":
			io.WriteString(w, taskJSON)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p1/sections":
			io.WriteString(w, `{"data":[{"gid":"s1","name":"To Do"},{"gid":"s2","name":"In Review"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p3":
			io.WriteString(w, `{"data":{"gid":"p3","name":"Ops"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/attachments":
			_, h, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			writes = append(writes, write{r.Method, r.URL.Path, map[string]any{"parent": r.FormValue("parent"), "name": h.Filename}})
			io.WriteString(w, `{"data":{"gid":"a`+strconv.Itoa(len(writes))+`"}}`)
		case r.Method != http.MethodGet:
			var body struct {
				Data map[string]any `json:"data"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			writes = append(writes, write{r.Method, r.URL.Path, body.Data})
			io.WriteString(w, `{"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	c := asana.New("tok")
	c.BaseURL = srv.URL
	return c, &writes
}

var ctx = context.Background()

func TestNeedsConfirm(t *testing.T) {
	cases := []struct {
		yes      bool
		env      string
		def, out bool
	}{
		{true, "1", true, false},
		{false, "0", true, false},
		{false, "1", false, true},
		{false, "", true, true},
		{false, "", false, false},
	}
	for _, c := range cases {
		if got := NeedsConfirm(c.yes, c.env, c.def); got != c.out {
			t.Errorf("NeedsConfirm(%v, %q, %v) = %v", c.yes, c.env, c.def, got)
		}
	}
}

func TestCommentPosts(t *testing.T) {
	c, writes := fake(t)
	if err := (Service{Client: c}).Comment(ctx, "1", "  PR: https://x  \n"); err != nil {
		t.Fatal(err)
	}
	if len(*writes) != 1 || (*writes)[0].path != "/tasks/1/stories" || (*writes)[0].body["text"] != "PR: https://x" {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestDeclineBlocksWrite(t *testing.T) {
	c, writes := fake(t)
	var asked string
	svc := Service{Client: c, Confirm: func(p string) (bool, error) { asked = p; return false, nil }}
	if err := svc.Comment(ctx, "1", "hi"); !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v", err)
	}
	if len(*writes) != 0 || !strings.Contains(asked, "Fix login") {
		t.Fatalf("writes = %+v, prompt = %q", *writes, asked)
	}
}

func TestConfirmErrorBlocksWrite(t *testing.T) {
	c, writes := fake(t)
	svc := Service{Client: c, Confirm: func(string) (bool, error) { return false, ErrNoTTY }}
	if err := svc.SetField(ctx, "1", "points", "3", ""); !errors.Is(err, ErrNoTTY) {
		t.Fatalf("err = %v", err)
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestMoveNeedsProjectWhenAmbiguous(t *testing.T) {
	c, _ := fake(t)
	err := (Service{Client: c}).Move(ctx, "1", "In Review", "")
	if err == nil || !strings.Contains(err.Error(), "--project") || !strings.Contains(err.Error(), "Web (p1)") {
		t.Fatalf("err = %v", err)
	}
}

func TestMoveMatchesSectionCaseInsensitively(t *testing.T) {
	c, writes := fake(t)
	if err := (Service{Client: c}).Move(ctx, "1", "in review", "p1"); err != nil {
		t.Fatal(err)
	}
	if len(*writes) != 1 || (*writes)[0].path != "/sections/s2/addTask" || (*writes)[0].body["task"] != "1" {
		t.Fatalf("writes = %+v", *writes)
	}
	if err := (Service{Client: c}).Move(ctx, "1", "Shipped", "p1"); err == nil || !strings.Contains(err.Error(), "To Do") {
		t.Fatalf("err = %v, want list of sections", err)
	}
}

func TestAssign(t *testing.T) {
	users := []asana.Ref{{GID: "9", Name: "Ann Lee"}, {GID: "8", Name: "Bo"}}
	for user, want := range map[string]any{"Bo": "8", " ann lee ": "9", "9": "9", "me": "me", "": nil} {
		c, writes := fake(t)
		if err := (Service{Client: c, Users: users}).Assign(ctx, "1", user); err != nil {
			t.Fatalf("Assign(%q): %v", user, err)
		}
		if len(*writes) != 1 || (*writes)[0].path != "/tasks/1" || (*writes)[0].body["assignee"] != want {
			t.Errorf("Assign(%q): writes = %+v, want assignee %v", user, *writes, want)
		}
	}
}

func TestAssignRejectsUnknownOrAmbiguousUsers(t *testing.T) {
	c, writes := fake(t)
	svc := Service{Client: c, Users: []asana.Ref{{GID: "9", Name: "Ann"}, {GID: "7", Name: "ann"}, {GID: "8", Name: "Bo"}}}
	for user, want := range map[string]string{"Ann": "several", "Cy": "no user"} {
		if err := svc.Assign(ctx, "1", user); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Assign(%q) = %v, want error containing %q", user, err, want)
		}
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestSetDue(t *testing.T) {
	for date, want := range map[string]any{"2026-10-01": "2026-10-01", " 2026-10-01\n": "2026-10-01", "": nil} {
		c, writes := fake(t)
		if err := (Service{Client: c}).SetDue(ctx, "1", date); err != nil {
			t.Fatalf("SetDue(%q): %v", date, err)
		}
		if len(*writes) != 1 || (*writes)[0].path != "/tasks/1" || (*writes)[0].body["due_on"] != want {
			t.Errorf("SetDue(%q): writes = %+v, want due_on %v", date, *writes, want)
		}
	}
}

func TestSetDueRejectsBadDates(t *testing.T) {
	c, writes := fake(t)
	for _, date := range []string{"tomorrow", "2026-02-30", "10/01/2026", "2026-1-1", "2026-10-01T10:00:00Z"} {
		if err := (Service{Client: c}).SetDue(ctx, "1", date); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
			t.Errorf("SetDue(%q) = %v, want YYYY-MM-DD error", date, err)
		}
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestProjectMembership(t *testing.T) {
	c, writes := fake(t)
	svc := Service{Client: c}
	if err := svc.AddProject(ctx, "1", "p3"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveProject(ctx, "1", "p1"); err != nil {
		t.Fatal(err)
	}
	want := []write{
		{"POST", "/tasks/1/addProject", map[string]any{"project": "p3"}},
		{"POST", "/tasks/1/removeProject", map[string]any{"project": "p1"}},
	}
	if !reflect.DeepEqual(*writes, want) {
		t.Fatalf("writes = %+v", *writes)
	}
	if svc.AddProject(ctx, "1", "") == nil || svc.RemoveProject(ctx, "1", "") == nil {
		t.Fatal("an empty project gid must fail")
	}
}

func TestNewWritesConfirmWithNames(t *testing.T) {
	c, writes := fake(t)
	var asked []string
	svc := Service{Client: c, Users: []asana.Ref{{GID: "8", Name: "Bo"}}, Confirm: func(p string) (bool, error) {
		asked = append(asked, p)
		return false, nil
	}}
	for _, err := range []error{
		svc.Assign(ctx, "1", "Bo"),
		svc.Assign(ctx, "1", ""),
		svc.SetDue(ctx, "1", "2026-10-01"),
		svc.AddProject(ctx, "1", "p3"),
		svc.RemoveProject(ctx, "1", "p2"),
	} {
		if !errors.Is(err, ErrDeclined) {
			t.Fatalf("err = %v", err)
		}
	}
	want := []string{
		`Assign "Fix login" to Bo?`,
		`Unassign "Fix login"?`,
		`Set the due date of "Fix login" to 2026-10-01?`,
		`Add "Fix login" to project Ops?`,
		`Remove "Fix login" from project API?`,
	}
	if !reflect.DeepEqual(asked, want) || len(*writes) != 0 {
		t.Fatalf("asked = %q, writes = %+v", asked, *writes)
	}
	if err := svc.RemoveProject(ctx, "1", "p3"); err == nil || !strings.Contains(err.Error(), "not in project") {
		t.Fatalf("remove from a project the task is not in: err = %v", err)
	}
}

func TestSetFieldValues(t *testing.T) {
	cases := []struct {
		field, value string
		want         any
	}{
		{"branch name", "feat/x", "feat/x"},
		{"Points", "3.5", 3.5},
		{"stage", "review", "o1"},
		{"Points", "", nil},
		{"When", "2026-01-01", map[string]any{"date": "2026-01-01"}},
	}
	for _, tc := range cases {
		c, writes := fake(t)
		if err := (Service{Client: c}).SetField(ctx, "1", tc.field, tc.value, ""); err != nil {
			t.Fatalf("%s=%q: %v", tc.field, tc.value, err)
		}
		fields := (*writes)[0].body["custom_fields"].(map[string]any)
		for _, got := range fields {
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s=%q: sent %v, want %v", tc.field, tc.value, got, tc.want)
			}
		}
	}
}

func TestSetFieldListValues(t *testing.T) {
	users := []asana.Ref{{GID: "9", Name: "Ann Lee"}, {GID: "8", Name: "Bo"}}
	cases := []struct {
		field, value string
		want         any
	}{
		{"areas", "backend, frontend", []any{"o3", "o2"}},
		{"Areas", "Frontend,frontend", []any{"o2"}},
		{"reviewers", "ann lee, Bo", []any{"9", "8"}},
		{"Reviewers", "", nil},
	}
	for _, tc := range cases {
		c, writes := fake(t)
		if err := (Service{Client: c, Users: users}).SetField(ctx, "1", tc.field, tc.value, ""); err != nil {
			t.Fatalf("%s=%q: %v", tc.field, tc.value, err)
		}
		fields := (*writes)[0].body["custom_fields"].(map[string]any)
		if got := fields[map[string]string{"areas": "f5", "Areas": "f5", "reviewers": "f6", "Reviewers": "f6"}[tc.field]]; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s=%q: sent %v, want %v", tc.field, tc.value, got, tc.want)
		}
	}
}

func TestSetFieldRejectsUnknownOrAmbiguousPeople(t *testing.T) {
	c, writes := fake(t)
	svc := Service{Client: c, Users: []asana.Ref{{GID: "9", Name: "Ann"}, {GID: "7", Name: "Ann"}, {GID: "8", Name: "Bo"}}}
	for value, want := range map[string]string{"Ann": "several", "Cy": "no user", "Bo,": "empty"} {
		if err := svc.SetField(ctx, "1", "Reviewers", value, ""); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("SetField(Reviewers, %q) = %v, want error containing %q", value, err, want)
		}
	}
	if err := svc.SetField(ctx, "1", "Areas", "Frontend, Ops", ""); err == nil || !strings.Contains(err.Error(), "Backend") {
		t.Errorf("unknown option err = %v, want list of options", err)
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestSetFieldRejectsBadInput(t *testing.T) {
	c, writes := fake(t)
	svc := Service{Client: c}
	for _, args := range [][2]string{{"Points", "many"}, {"Stage", "Shipped"}, {"When", "tomorrow"}, {"Nope", "x"}} {
		if err := svc.SetField(ctx, "1", args[0], args[1], ""); err == nil {
			t.Errorf("SetField(%q, %q) succeeded", args[0], args[1])
		}
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}

func TestMessagesStripControlCharacters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/1":
			io.WriteString(w, `{"data":{"gid":"1","name":"T","memberships":[{"project":{"gid":"p1","name":"Web\u001b]0;x\u0007"}},{"project":{"gid":"p2","name":"API"}}]}}`)
		case "/projects/p1/sections":
			io.WriteString(w, `{"data":[{"gid":"s1","name":"To Do\u001b[2J"}]}`)
		}
	}))
	defer srv.Close()
	c := asana.New("tok")
	c.BaseURL = srv.URL
	svc := Service{Client: c}
	for _, err := range []error{svc.Move(ctx, "1", "Nope", "p1"), svc.Move(ctx, "1", "Nope", "")} {
		if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
			t.Errorf("err = %q", err)
		}
	}
}

func TestSetFieldSameNameNeedsProject(t *testing.T) {
	var puts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tasks/1" && r.Method == http.MethodGet:
			io.WriteString(w, `{"data":{"gid":"1","name":"T","custom_fields":[
				{"gid":"f1","name":"Branch Name ","resource_subtype":"text"},
				{"gid":"f5","name":"Branch name","resource_subtype":"text"}]}}`)
		case r.URL.Path == "/projects/p2/custom_field_settings":
			io.WriteString(w, `{"data":[{"custom_field":{"gid":"f5"}}]}`)
		case r.URL.Path == "/projects/p3/custom_field_settings":
			io.WriteString(w, `{"data":[{"custom_field":{"gid":"zz"}}]}`)
		case r.Method == http.MethodPut:
			var body struct {
				Data struct {
					CustomFields map[string]any `json:"custom_fields"`
				} `json:"data"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			for gid := range body.Data.CustomFields {
				puts = append(puts, gid)
			}
			io.WriteString(w, `{"data":{}}`)
		}
	}))
	defer srv.Close()
	c := asana.New("tok")
	c.BaseURL = srv.URL
	svc := Service{Client: c}
	if err := svc.SetField(ctx, "1", "branch name", "x", ""); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("no project: err = %v", err)
	}
	if err := svc.SetField(ctx, "1", "branch name", "x", "p3"); err == nil {
		t.Fatal("project without either field must fail")
	}
	if err := svc.SetField(ctx, "1", "branch name", "x", "p2"); err != nil || len(puts) != 1 || puts[0] != "f5" {
		t.Fatalf("err = %v, puts = %v", err, puts)
	}
	if err := svc.SetField(ctx, "1", "f1", "x", ""); err != nil || len(puts) != 2 || puts[1] != "f1" {
		t.Fatalf("by gid: err = %v, puts = %v", err, puts)
	}
}

func TestSetFieldTakesGIDs(t *testing.T) {
	users := []asana.Ref{{GID: "9", Name: "Ann"}, {GID: "7", Name: "Ann"}}
	cases := []struct {
		field, value string
		want         any
	}{
		{"f3", "o1", "o1"},
		{"Areas", "o3,o2", []any{"o3", "o2"}},
		{"Reviewers", "7", []any{"7"}},
	}
	for _, tc := range cases {
		c, writes := fake(t)
		if err := (Service{Client: c, Users: users}).SetField(ctx, "1", tc.field, tc.value, ""); err != nil {
			t.Fatalf("%s=%q: %v", tc.field, tc.value, err)
		}
		for _, got := range (*writes)[0].body["custom_fields"].(map[string]any) {
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s=%q: sent %v, want %v", tc.field, tc.value, got, tc.want)
			}
		}
	}
}

func TestCommentHTML(t *testing.T) {
	users := []asana.Ref{
		{GID: "10", Name: "Victoria"},
		{GID: "11", Name: "Victoria Andersen"},
		{GID: "12", Name: "Sam Lee"},
		{GID: "13", Name: "Sam Lee"},
		{GID: "x<", Name: "Bad Gid"},
	}
	cases := []struct {
		text, html string
		gids       []string
	}{
		{"hi @victoria andersen, see <this> & that", `<body>hi <a data-asana-gid="11"/>, see &lt;this&gt; &amp; that</body>`, []string{"11"}},
		{"@Victoria said\n@VICTORIA ANDERSEN", "<body><a data-asana-gid=\"10\"/> said\n<a data-asana-gid=\"11\"/></body>", []string{"10", "11"}},
		{"@Victoria Andersenx", `<body><a data-asana-gid="10"/> Andersenx</body>`, []string{"10"}},
		{"mail me@victoria or @nobody", "<body>mail me@victoria or @nobody</body>", nil},
		{"ask @Sam Lee or @Bad Gid", "<body>ask @Sam Lee or @Bad Gid</body>", nil},
	}
	for _, c := range cases {
		html, mentioned := CommentHTML(c.text, users, nil)
		var gids []string
		for _, u := range mentioned {
			gids = append(gids, u.GID)
		}
		if html != c.html || !reflect.DeepEqual(gids, c.gids) {
			t.Errorf("CommentHTML(%q) = %q, %v; want %q, %v", c.text, html, gids, c.html, c.gids)
		}
	}
}

func TestCommentMentionsPostHTML(t *testing.T) {
	c, writes := fake(t)
	var asked string
	svc := Service{Client: c, Users: []asana.Ref{{GID: "11", Name: "Victoria Andersen"}},
		Confirm: func(p string) (bool, error) { asked = p; return true, nil }}
	if err := svc.Comment(ctx, "1", "thanks @victoria andersen"); err != nil {
		t.Fatal(err)
	}
	want := `<body>thanks <a data-asana-gid="11"/></body>`
	if len(*writes) != 1 || (*writes)[0].body["html_text"] != want || (*writes)[0].body["text"] != nil {
		t.Fatalf("writes = %+v", *writes)
	}
	if !strings.Contains(asked, "Mentions: Victoria Andersen") {
		t.Fatalf("prompt = %q", asked)
	}
}
