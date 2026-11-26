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

// FavoritesColor is the colour tag of member's favorites ("" none).
func (s *Store) FavoritesColor(member string) (string, error) {
	var c string
	err := s.db.QueryRow(`SELECT color FROM favorite_colors WHERE member = ?`, member).Scan(&c)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return c, err
}

// SetFavoritesColor tags member's favorites with color ("" takes it off).
func (s *Store) SetFavoritesColor(member, color string) error {
	if color == "" {
		_, err := s.db.Exec(`DELETE FROM favorite_colors WHERE member = ?`, member)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO favorite_colors (member, color) VALUES (?, ?) ON CONFLICT (member) DO UPDATE SET color = excluded.color`, member, color)
	return err
}

// FavoritesName is the name member gave its favorites ("" none: the page's own).
func (s *Store) FavoritesName(member string) (string, error) {
	var n string
	err := s.db.QueryRow(`SELECT name FROM favorite_names WHERE member = ?`, member).Scan(&n)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return n, err
}

// SetFavoritesName names member's favorites ("" takes the name off).
func (s *Store) SetFavoritesName(member, name string) error {
	if name == "" {
		_, err := s.db.Exec(`DELETE FROM favorite_names WHERE member = ?`, member)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO favorite_names (member, name) VALUES (?, ?) ON CONFLICT (member) DO UPDATE SET name = excluded.name`, member, name)
	return err
}
