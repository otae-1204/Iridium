package store

// pushRulesSchema is migration 9: Web Push's rules, the default events (scope
// default, target "") and what a workspace (target its id, "0" sessions in none), a folder (its
// absolute path) or a session (its id) says over them: off, or on with its own events (NULL: the
// ones from above). No row: as above. push_settings' rows become workspace rules: none of its
// events is off, the defaults' are dropped. Web Push is gone (kumo, 2026-11-26): the table, like
// push_subscriptions, is left as it is and read by nothing.
const pushRulesSchema = `CREATE TABLE push_rules (
		scope TEXT NOT NULL,
		target TEXT NOT NULL,
		off INTEGER NOT NULL DEFAULT 0,
		needs_you INTEGER,
		error INTEGER,
		turn_end INTEGER,
		compact INTEGER,
		PRIMARY KEY (scope, target)
	);
	INSERT INTO push_rules (scope, target, off, needs_you, error, turn_end, compact)
		SELECT 'workspace', CAST(workspace AS TEXT), NOT (needs_you OR error OR turn_end OR compact),
			IIF(needs_you OR error OR turn_end OR compact, needs_you, NULL),
			IIF(needs_you OR error OR turn_end OR compact, error, NULL),
			IIF(needs_you OR error OR turn_end OR compact, turn_end, NULL),
			IIF(needs_you OR error OR turn_end OR compact, compact, NULL)
		FROM push_settings WHERE NOT (needs_you AND error AND NOT turn_end AND NOT compact);
	DROP TABLE push_settings;`
