package server

import (
	"strings"
	"testing"

	"github.com/kumoSleeping/iridium/internal/api"
)

// The favorites' name and colour are the caller's own (kumo, 2026-11-26): one line, at most 40
// characters, "" the page's own; all of them taken off at once by an empty list.
func TestFavoritesNamed(t *testing.T) {
	t.Parallel()
	l, ch, _ := two(t)
	var f api.Favorites
	l.want(200, "web", "PUT", "/v1/internal/favorites", map[string]any{"channels": []string{ch}}, &f)
	l.want(200, "web", "PATCH", "/v1/internal/favorites", map[string]any{"name": "  常用\n  频道 "}, &f)
	if f.Name != "常用 频道" || len(f.Channels) != 1 {
		t.Fatalf("named: %+v", f)
	}
	l.want(400, "web", "PATCH", "/v1/internal/favorites", map[string]any{"name": strings.Repeat("长", api.FavoritesNameMax+1)}, nil)
	f = api.Favorites{}
	l.want(200, "web", "PATCH", "/v1/internal/favorites", map[string]any{"color": "green"}, &f)
	if f.Name != "常用 频道" || f.Color != "green" {
		t.Fatalf("its colour beside its name: %+v", f)
	}
	f = api.Favorites{}
	l.want(200, "web", "PATCH", "/v1/internal/favorites", map[string]any{"name": ""}, &f)
	if f.Name != "" {
		t.Fatalf("its name taken off: %+v", f)
	}
	f = api.Favorites{}
	l.want(200, "web", "PUT", "/v1/internal/favorites", map[string]any{"channels": []string{}}, &f)
	if len(f.Channels) != 0 || f.Color != "green" {
		t.Fatalf("all taken off: %+v", f)
	}
}
