package writeback

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/asana"
)

func TestCommentMarkdown(t *testing.T) {
	cases := []struct {
		in, want string
		mentions int
	}{
		{"**bold** *italic* ~~gone~~", "<body><strong>bold</strong> <em>italic</em> <s>gone</s></body>", 0},
		{"```go\n<x> & @Sam Lee\n```", "<body><pre>&lt;x&gt; &amp; @Sam Lee\n</pre></body>", 0},
		{"`@Sam Lee` **@Sam Lee**", "<body><code>@Sam Lee</code> <strong><a data-asana-gid=\"1\"/></strong></body>", 1},
		{"- one\n- two", "<body><ul><li>one</li><li>two</li></ul></body>", 0},
		{"1. one\n2. two", "<body><ol><li>one</li><li>two</li></ol></body>", 0},
		{"[go](https://example.com/?a=1&b=2)", "<body><a href=\"https://example.com/?a=1&amp;b=2\">go</a></body>", 0},
		{"[bad](javascript:alert(1))", "<body>bad</body>", 0},
		{"[bad](https:relative)", "<body>bad</body>", 0},
		{"<https://example.com>", "<body><a href=\"https://example.com\">https://example.com</a></body>", 0},
		{"<sam@example.com>", "<body><a href=\"mailto:sam@example.com\">sam@example.com</a></body>", 0},
		{"# Header", "<body><strong>Header</strong></body>", 0},
		{"<script>bad()</script>", "<body>&lt;script&gt;bad()&lt;/script&gt;</body>", 0},
		{"one\n\ntwo", "<body>one\n\ntwo</body>", 0},
		{"\\*literal\\* &amp;", "<body>*literal* &amp;</body>", 0},
		{"    a < b\n", "<body><pre>a &lt; b\n</pre></body>", 0},
		{"> quoted", "<body><blockquote>quoted</blockquote></body>", 0},
		{"> - one\n> - two", "<body><blockquote>- one\n- two</blockquote></body>", 0},
		{"- one\n\n  ```\n  x\n  ```", "<body><ul><li>one\n\n<code>x\n</code></li></ul></body>", 0},
		{"- > quoted", "<body><ul><li>quoted</li></ul></body>", 0},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, mentioned := CommentHTML(c.in, []asana.Ref{{GID: "1", Name: "Sam Lee"}}, nil)
			if got != c.want || len(mentioned) != c.mentions {
				t.Fatalf("CommentHTML(%q) = %q, %v; want %q, %d mentions", c.in, got, mentioned, c.want, c.mentions)
			}
			decoder := xml.NewDecoder(strings.NewReader(got))
			for {
				_, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("invalid Asana XML %q: %v", got, err)
				}
			}
		})
	}
}

func TestCommentMarkdownPostsHTML(t *testing.T) {
	client, writes := fake(t)
	const markdown = "**Values**\n\n```json\n{\"enabled\": true}\n```"
	const want = "<body><strong>Values</strong>\n\n<pre>{\"enabled\": true}\n</pre></body>"
	var asked string
	service := Service{Client: client, Confirm: func(prompt string) (bool, error) { asked = prompt; return true, nil }}
	if err := service.Comment(ctx, "1", markdown); err != nil {
		t.Fatal(err)
	}
	if len(*writes) != 1 || (*writes)[0].method != "POST" || (*writes)[0].path != "/tasks/1/stories" || (*writes)[0].body["html_text"] != want || (*writes)[0].body["text"] != nil {
		t.Fatalf("writes = %+v", *writes)
	}
	if !strings.Contains(asked, markdown) {
		t.Fatalf("confirmation omits Markdown: %q", asked)
	}
}

func TestCommentMarkdownCodeMentions(t *testing.T) {
	client, writes := fake(t)
	const markdown = "`@Sam Lee` [@Sam Lee](https://example.com)\n\n```\n@Sam Lee\n```"
	var asked string
	service := Service{Client: client, Users: []asana.Ref{{GID: "1", Name: "Sam Lee"}}, Confirm: func(prompt string) (bool, error) { asked = prompt; return true, nil }}
	if err := service.Comment(ctx, "1", markdown); err != nil {
		t.Fatal(err)
	}
	if len(*writes) != 1 {
		t.Fatalf("writes = %+v", *writes)
	}
	body, ok := (*writes)[0].body["html_text"].(string)
	if !ok || strings.Contains(body, "data-asana-gid") || strings.Contains(asked, "Mentions:") {
		t.Fatalf("code/link text creates mentions: body = %q, prompt = %q", body, asked)
	}
}

func TestLocalImages(t *testing.T) {
	got := LocalImages("![a](shot.png) ![b](<~/My Shots/x.png>) ![c](https://example.com/c.png) ![d](shot.png) `![e](code.png)`")
	want := []string{"shot.png", "~/My Shots/x.png"}
	if !slices.Equal(got, want) {
		t.Fatalf("LocalImages = %q; want %q", got, want)
	}
}

func TestCommentHTMLInlinesUploadedImages(t *testing.T) {
	got, _ := CommentHTML("see ![shot](a.png) and ![web](https://example.com/b.png)", nil, map[string]string{"a.png": "7"})
	want := `<body>see <img data-asana-gid="7"/> and web</body>`
	if got != want {
		t.Fatalf("CommentHTML = %q; want %q", got, want)
	}
}

func TestCommentUploadsLocalImages(t *testing.T) {
	client, writes := fake(t)
	dir := t.TempDir()
	png := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(png, []byte("\x89PNG\r\n\x1a\nrest"), 0o600); err != nil {
		t.Fatal(err)
	}
	var asked string
	service := Service{Client: client, Confirm: func(prompt string) (bool, error) { asked = prompt; return true, nil }}
	if err := service.Comment(ctx, "1", "Context:\n\n![shot]("+png+")"); err != nil {
		t.Fatal(err)
	}
	if len(*writes) != 2 || (*writes)[0].path != "/attachments" || (*writes)[0].body["parent"] != "1" || (*writes)[0].body["name"] != "shot.png" {
		t.Fatalf("writes = %+v", *writes)
	}
	if want := "<body>Context:\n\n<img data-asana-gid=\"a1\"/></body>"; (*writes)[1].body["html_text"] != want {
		t.Fatalf("html_text = %q; want %q", (*writes)[1].body["html_text"], want)
	}
	if !strings.Contains(asked, "Uploads: "+png) {
		t.Fatalf("confirmation omits uploads: %q", asked)
	}
}

func TestCommentRejectsNonImageFiles(t *testing.T) {
	client, writes := fake(t)
	secret := filepath.Join(t.TempDir(), "id_rsa.png")
	if err := os.WriteFile(secret, []byte("-----BEGIN OPENSSH PRIVATE KEY-----"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{secret, filepath.Dir(secret), filepath.Join(filepath.Dir(secret), "missing.png")} {
		if err := (Service{Client: client}).Comment(ctx, "1", "![x]("+path+")"); err == nil {
			t.Errorf("Comment with %s posted", path)
		}
	}
	if len(*writes) != 0 {
		t.Fatalf("writes = %+v", *writes)
	}
}
