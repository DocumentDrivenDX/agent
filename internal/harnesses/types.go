package harnesses

// AccountInfo captures provider account metadata from local auth files.
type AccountInfo struct {
	Email    string `json:"email,omitempty"`
	PlanType string `json:"plan_type,omitempty"`
	OrgName  string `json:"org_name,omitempty"`
}

// QuotaWindow captures one quota window (e.g. 5h, weekly, model-specific).
type QuotaWindow struct {
	Name          string  `json:"name"`               // e.g. "5h", "7d", "spark"
	LimitID       string  `json:"limit_id,omitempty"` // provider limit_id
	WindowMinutes int     `json:"window_minutes"`
	UsedPercent   float64 `json:"used_percent"`
	ResetsAt      string  `json:"resets_at,omitempty"`      // human-readable
	ResetsAtUnix  int64   `json:"resets_at_unix,omitempty"` // unix timestamp
	State         string  `json:"state"`
}

// QuotaStateFromUsedPercent maps a usage percentage to a quota state string.
func QuotaStateFromUsedPercent(usedPercent int) string {
	if usedPercent >= 95 {
		return "blocked"
	}
	if usedPercent >= 0 {
		return "ok"
	}
	return "unknown"
}
