package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every state the page always carries (web/src/stream.ts ALWAYS) is one the stream gives: a key
// left out of stateKey was ignored, its changes never reaching the page (cards, scripts till
// 2026-11-26).
func TestStateKeysOfThePage(t *testing.T) {
	b, err := os.ReadFile("../../web/src/stream.ts")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const ALWAYS = \[([^\]]*)\]`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no ALWAYS in stream.ts")
	}
	var keys []string
	for _, k := range strings.Split(string(m[1]), ",") {
		keys = append(keys, strings.Trim(strings.TrimSpace(k), `"`))
	}
	if len(keys) < 10 {
		t.Fatalf("ALWAYS read as %v", keys)
	}
	carried, ignored := (&Server{}).stateKeys(caller{}, keys)
	for _, e := range ignored {
		if e.Ignored == "unknown" {
			t.Errorf("the page carries %q; the stream does not know it", e.Key)
		}
	}
	if len(carried) == 0 {
		t.Error("none carried")
	}
}
