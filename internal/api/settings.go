package api

// Settings is GET /v1/internal/settings: the config file's fields the settings page edits (plan "运行",
// "网页"). PUT /v1/internal/settings takes a SettingsPatch and answers the whole again.
type Settings struct {
	// IdleReap: a hosted agent idle this long is stopped (its session stays), as "1h"; "0" never.
	IdleReap string `json:"idleReap"`
	// QuotaHidden: the quota bars not shown, each "<agent>:<account>:<group>:<model>".
	QuotaHidden []string `json:"quotaHidden"`
	// Modes: the approval mode a new session of each agent starts in; an agent left out starts in
	// its own default.
	Modes map[string]string `json:"modes"`
	// Lang: the user's language, as a web page's settings last set it ("zh", "en"; "" not yet):
	// hosted agents are told it from their next start.
	Lang string `json:"lang"`
}

// SettingsPatch changes what it gives and nothing else. A mode "" goes back to the agent's own.
type SettingsPatch struct {
	IdleReap    *string           `json:"idleReap,omitempty"`
	QuotaHidden *[]string         `json:"quotaHidden,omitempty"`
	Modes       map[string]string `json:"modes,omitempty"`
	Lang        *string           `json:"lang,omitempty"`
}
