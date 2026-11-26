package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/store"
)

// Favorites (kumo, 2026-11-26): each member's one list of channels, its own, at the top of its
// list of sessions while it holds any. Kept by member, so a person's follows them to every device.

func (s *Server) favoritesGet(w http.ResponseWriter, r *http.Request) {
	chs, err := s.st.Favorites(memberOf(callerOf(r)))
	if err != nil {
		s.fault(w, err)
		return
	}
	reply(w, http.StatusOK, api.Favorites{Channels: chs})
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
	out, err := s.st.Favorites(member)
	if err != nil {
		s.fault(w, err)
		return
	}
	reply(w, http.StatusOK, api.Favorites{Channels: out})
}
