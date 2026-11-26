package store

import "database/sql"

// Favorites (kumo, 2026-11-26): each member's one list of channels (a session's is its own),
// in its own order; theirs alone, changing no one else's list. A channel favoured stays where it
// is in the list of sessions as well.

// MaxFavorites is the most channels one member's favorites hold.
const MaxFavorites = 200

// Favorites are the channels member favoured, in its order.
func (s *Store) Favorites(member string) ([]string, error) {
	rows, err := s.db.Query(`SELECT channel FROM favorites WHERE member = ? ORDER BY sort`, member)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// SetFavorites makes member's favorites channels, in that order (a channel named twice: its
// first place); none drops them all.
func (s *Store) SetFavorites(member string, channels []string) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM favorites WHERE member = ?`, member); err != nil {
			return err
		}
		for i, ch := range channels {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO favorites (member, channel, sort) VALUES (?, ?, ?)`, member, ch, i); err != nil {
				return err
			}
		}
		return nil
	})
}
