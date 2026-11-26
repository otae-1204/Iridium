package store

import (
	"slices"
	"testing"
)

func TestFavorites(t *testing.T) {
	s, _ := open(t)
	must(t, s.SetFavorites("web/a", []string{"x", "y", "x", "z"}))
	must(t, s.SetFavorites("web/b", []string{"z"}))
	got, err := s.Favorites("web/a")
	must(t, err)
	if !slices.Equal(got, []string{"x", "y", "z"}) {
		t.Fatalf("a: %v", got)
	}
	must(t, s.SetFavorites("web/a", []string{"z", "x"}))
	got, _ = s.Favorites("web/a")
	if !slices.Equal(got, []string{"z", "x"}) {
		t.Fatalf("a reordered: %v", got)
	}
	got, _ = s.Favorites("web/b")
	if !slices.Equal(got, []string{"z"}) {
		t.Fatalf("b: %v", got)
	}
	must(t, s.SetFavorites("web/a", nil))
	if got, _ = s.Favorites("web/a"); len(got) != 0 {
		t.Fatalf("a cleared: %v", got)
	}
	must(t, s.SetFavoritesColor("web/a", "blue"))
	if c, _ := s.FavoritesColor("web/a"); c != "blue" {
		t.Fatalf("a's colour: %q", c)
	}
	if c, _ := s.FavoritesColor("web/b"); c != "" {
		t.Fatalf("b's colour: %q", c)
	}
	must(t, s.SetFavoritesColor("web/a", ""))
	if c, _ := s.FavoritesColor("web/a"); c != "" {
		t.Fatalf("a's colour taken off: %q", c)
	}
}
