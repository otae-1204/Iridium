package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

// Migration 9 keeps what push_settings said: a workspace with nothing on is off, one with the
// defaults says nothing, any other is on with its events.
func TestPushRulesFromSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "iridium.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := db.Exec(schema[i]); err != nil {
			t.Fatal(err)
		}
	}
	db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, 8))
	for _, q := range []string{
		`INSERT INTO push_settings VALUES (0, 0, 0, 0, 0)`,
		`INSERT INTO push_settings VALUES (1, 1, 1, 0, 0)`,
		`INSERT INTO push_settings VALUES (2, 1, 0, 1, 0)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	type rule struct {
		scope, target string
		off           bool
		needs, turn   sql.NullBool
	}
	var rules []rule
	rows, err := s.db.Query(`SELECT scope, target, off, needs_you, turn_end FROM push_rules ORDER BY scope, target`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.scope, &r.target, &r.off, &r.needs, &r.turn); err != nil {
			t.Fatal(err)
		}
		rules = append(rules, r)
	}
	rows.Close()
	if len(rules) != 2 {
		t.Fatalf("rules: %+v", rules)
	}
	if r := rules[0]; r.scope != "workspace" || r.target != "0" || !r.off || r.needs.Valid {
		t.Errorf("nothing on: %+v", r)
	}
	if r := rules[1]; r.target != "2" || r.off || !r.needs.Bool || !r.turn.Bool {
		t.Errorf("its own: %+v", r)
	}
}
