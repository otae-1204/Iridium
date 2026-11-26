package session

import (
	"testing"

	"github.com/kumoSleeping/iridium/internal/api"
)

func TestPayer(t *testing.T) {
	m := &Manager{o: Options{Owner: func(id string) string {
		if id == "s1" {
			return "kumo"
		}
		return ""
	}}}
	for _, c := range []struct{ id, from, want string }{
		{"s1", "alice", "alice"},          // a person's own message: theirs
		{"s1", api.HostID, "kumo"},        // a channel's relay: the person it is for
		{"s1", api.SourceIridium, "kumo"}, // Iridium's own prompt
		{"s1", "", "kumo"},                // no one known
		{"s2", api.HostID, api.HostID},    // no person known: as it was
	} {
		if got := m.payer(c.id, c.from); got != c.want {
			t.Errorf("payer(%q, %q) = %q, want %q", c.id, c.from, got, c.want)
		}
	}
	if got := (&Manager{}).payer("s1", api.HostID); got != api.HostID {
		t.Errorf("no Owner: %q", got)
	}
}
