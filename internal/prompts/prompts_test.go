package prompts

import (
	"regexp"
	"strings"
	"testing"
)

func TestTexts(t *testing.T) {
	t.Parallel()
	ids := IDs()
	if len(ids) == 0 {
		t.Fatal("no fragments")
	}
	name := regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`)
	for _, id := range ids {
		x, ok := Get(id)
		if !ok || x.ID != id || x.Body == "" || x.Hash != Hash(x.Body) || len(x.Hash) != 12 {
			t.Errorf("%s: %+v", id, x)
		}
		if !name.MatchString(id) {
			t.Errorf("fragment ID %q", id)
		}
		// Nothing that looks like a slot and is not one.
		if rest := slot.ReplaceAllString(x.Body, ""); strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
			t.Errorf("%s: braces that are no slot", id)
		}
	}
	if _, ok := Get("no.such"); ok {
		t.Error("a fragment that is no file")
	}
}

// The body is the file verbatim: a system prompt's line ends with its newline, the tag before
// a message with its space and no newline.
func TestVerbatim(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{
		"message.tag": "[{{tag}}] ",
		"lang":        "- User's language: {{language}}\n",
	} {
		if x, _ := Get(id); x.Body != want {
			t.Errorf("%s: %q", id, x.Body)
		}
	}
	if x, _ := Get("title"); !strings.HasPrefix(x.Body, "\n## Title\n") || !strings.HasSuffix(x.Body, "set.\n") {
		t.Errorf("title: %q", x.Body)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	x := Text{ID: "x", Body: "a {{one}} b {{two-2}} c {{one}}"}
	if got := x.Slots(); len(got) != 2 || got[0] != "one" || got[1] != "two-2" {
		t.Errorf("slots %v", got)
	}
	// One pass: a value is not read for slots.
	s, err := x.Render(map[string]string{"one": "{{two-2}}", "two-2": "2"})
	if err != nil || s != "a {{two-2}} b 2 c {{two-2}}" {
		t.Errorf("%q, %v", s, err)
	}
	if _, err := x.Render(map[string]string{"one": "1"}); err == nil || !strings.Contains(err.Error(), "{{two-2}}") {
		t.Errorf("a slot without a value: %v", err)
	}
	if _, err := x.Render(map[string]string{"one": "1", "two-2": "2", "three": "3"}); err == nil || !strings.Contains(err.Error(), "{{three}}") {
		t.Errorf("a value without a slot: %v", err)
	}
	if s, err := (Text{ID: "y", Body: "plain\n"}).Render(nil); err != nil || s != "plain\n" {
		t.Errorf("%q, %v", s, err)
	}
}
