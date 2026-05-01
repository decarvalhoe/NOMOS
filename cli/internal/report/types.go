package report

// NomosReport is the top-level Nomos execution report matching
// specs/nomos-report.schema.json (schema_version 0.1.0).
type NomosReport struct {
	SchemaVersion string         `json:"schema_version"`
	ReportType    string         `json:"report_type"`
	GeneratedAt   string         `json:"generated_at"`
	Run           Run            `json:"run"`
	Project       Project        `json:"project"`
	Summary       Summary        `json:"summary"`
	Verdict       Verdict        `json:"verdict"`
	Checks        []CheckResult  `json:"checks"`
	Findings      []Finding      `json:"findings"`
	Evidence      []EvidenceItem `json:"evidence"`
}

type Run struct {
	ID          string       `json:"id"`
	Mode        string       `json:"mode"`
	Tool        Tool         `json:"tool"`
	Command     []string     `json:"command,omitempty"`
	StartedAt   string       `json:"started_at,omitempty"`
	CompletedAt string       `json:"completed_at,omitempty"`
	DurationMs  int          `json:"duration_ms,omitempty"`
	Git         *GitContext  `json:"git,omitempty"`
	Environment *Environment `json:"environment,omitempty"`
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type GitContext struct {
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
}

type Environment struct {
	CI     bool   `json:"ci,omitempty"`
	Runner string `json:"runner,omitempty"`
	OS     string `json:"os,omitempty"`
}

type Project struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Domain       string `json:"domain"`
	RiskLevel    string `json:"risk_level"`
	ManifestPath string `json:"manifest_path,omitempty"`
	ManifestHash string `json:"manifest_hash,omitempty"`
}

type Summary struct {
	CheckCount          int             `json:"check_count"`
	FindingCount        int             `json:"finding_count"`
	BlockingFindingCount int            `json:"blocking_finding_count"`
	EvidenceCount       int             `json:"evidence_count"`
	Coverage            CoverageSummary `json:"coverage"`
}

type CoverageSummary struct {
	UnitTotal         int     `json:"unit_total"`
	UnitCovered       int     `json:"unit_covered"`
	UnitPartial       int     `json:"unit_partial"`
	UnitMissing       int     `json:"unit_missing"`
	UnitNotApplicable int     `json:"unit_not_applicable"`
	CoverageRatio     float64 `json:"coverage_ratio"`
}

type Verdict struct {
	Status      string   `json:"status"`
	Severity    string   `json:"severity"`
	Blocking    bool     `json:"blocking"`
	Summary     string   `json:"summary"`
	NextActions []string `json:"next_actions,omitempty"`
}

type CheckResult struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Status      string   `json:"status"`
	Severity    string   `json:"severity"`
	StartedAt   string   `json:"started_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
	DurationMs  int      `json:"duration_ms,omitempty"`
	FindingIDs  []string `json:"finding_ids,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Message     string   `json:"message,omitempty"`
}

type Finding struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Severity    string   `json:"severity"`
	Status      string   `json:"status"`
	Blocking    bool     `json:"blocking"`
	Message     string   `json:"message"`
	Remediation string   `json:"remediation,omitempty"`
	Target      Target   `json:"target"`
	EvidenceIDs []string `json:"evidence_ids"`
	FirstSeenAt string   `json:"first_seen_at,omitempty"`
}

type Target struct {
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Path    string `json:"path,omitempty"`
	Locator string `json:"locator,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
}

type EvidenceItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	URI         string    `json:"uri,omitempty"`
	Hash        string    `json:"hash,omitempty"`
	Target      *Target   `json:"target,omitempty"`
	CollectedAt string    `json:"collected_at,omitempty"`
	Producer    string    `json:"producer,omitempty"`
}
