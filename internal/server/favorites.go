package server

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/store"
)

// Favorites (kumo, 2026-11-26): each member's one list of channels, its own, at the top of its
// list of sessions while it holds any. Kept by member, so a person's follows them to every device.

func (s *Server) favoritesGet(w http.ResponseWriter, r *http.Request) {
	s.favoritesReply(w, memberOf(callerOf(r)))
}

// favoritesReply answers with member's favorites as they are now.
func (s *Server) favoritesReply(w http.ResponseWriter, member string) {
	chs, err := s.st.Favorites(member)
	if err != nil {
		s.fault(w, err)
		return
	}
	c, err := s.st.FavoritesColor(member)
	if err != nil {
		s.fault(w, err)
		return
	}
	n, err := s.st.FavoritesName(member)
	if err != nil {
		s.fault(w, err)
		return
	}
	reply(w, http.StatusOK, api.Favorites{Channels: chs, Color: api.Color(c), Name: n})
}

// favoritesPatch tags the caller's favorites' folder with a colour, or takes it off.
func (s *Server) favoritesPatch(w http.ResponseWriter, r *http.Request) {
	var req api.FavoritesPatch
	if !readJSON(w, r, &req) {
		return
	}
	member := memberOf(callerOf(r))
	if req.Color != nil {
		if *req.Color != "" && !slices.Contains(api.Colors, *req.Color) {
			refuse(w, http.StatusBadRequest, "bad_request", "color: red, orange, yellow, green, blue, purple, or \"\"")
			return
		}
		if err := s.st.SetFavoritesColor(member, string(*req.Color)); err != nil {
			s.fault(w, err)
			return
		}
		s.hub.Set("favorites", s.favoritesSeq.Add(1))
	}
	if req.Name != nil {
		// One line, spaces at its ends dropped.
		name := strings.TrimSpace(strings.Join(strings.Fields(*req.Name), " "))
		if utf8.RuneCountInString(name) > api.FavoritesNameMax {
			refuse(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("name: at most %d characters", api.FavoritesNameMax))
			return
		}
		if err := s.st.SetFavoritesName(member, name); err != nil {
			s.fault(w, err)
			return
		}
		s.hub.Set("favorites", s.favoritesSeq.Add(1))
	}
	s.favoritesReply(w, member)
}

// favoritesPut makes the caller's favorites the list given, in its order.
func (s *Server) favoritesPut(w http.ResponseWriter, r *http.Request) {
	var req api.Favorites
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Channels) > store.MaxFavorites {
		refuse(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("channels: at most %d", store.MaxFavorites))
		return
	}
	chs := make([]string, 0, len(req.Channels))
	for _, ch := range req.Channels {
		ch = strings.TrimSpace(ch)
		if ch == "" || len(ch) > 200 {
			refuse(w, http.StatusBadRequest, "bad_request", "channels: channel ids")
			return
		}
		chs = append(chs, ch)
	}
	member := memberOf(callerOf(r))
	if err := s.st.SetFavorites(member, chs); err != nil {
		s.fault(w, err)
		return
	}
	s.hub.Set("favorites", s.favoritesSeq.Add(1))
	s.favoritesReply(w, member)
}
