package remediation

// Backlog is an ordered list of remediation items extracted from a Nomos report.
type Backlog struct {
	SchemaVersion string           `json:"schema_version"`
	ProjectID     string           `json:"project_id"`
	GeneratedAt   string           `json:"generated_at"`
	TotalItems    int              `json:"total_items"`
	BlockingItems int              `json:"blocking_items"`
	Items         []RemediationItem `json:"items"`
}

// RemediationItem is a single gap that needs to be closed.
type RemediationItem struct {
	ID          string `json:"id"`
	FindingID   string `json:"finding_id"`
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Blocking    bool   `json:"blocking"`
	Priority    int    `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
	Target      Target `json:"target"`
}

// Target identifies the source, code, or contract that a remediation item points to.
type Target struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Path    string `json:"path,omitempty"`
	Locator string `json:"locator,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
}
