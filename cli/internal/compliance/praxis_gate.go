package compliance

// NRT-018 (#662) — the Praxis activation gate, computed from the record and the
// repository. It answers one question: could Praxis regulated reliance be
// activated on the Nomos proof as it stands? It resolves each required proof
// against the ACTUAL state of the named artifacts and human records, and it
// never answers "activated": the best it can say is "activatable", and the
// activation itself is a human decision under docs/28. Every unknown resolves
// to "not met" with its reason, and a record whose own status contradicts the
// checks is refused as forged rather than trusted.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	PraxisGateVerdictSchema     = "nomos-praxis-activation-verdict-v1"
	PraxisGateStatusBlocked     = "blocked"
	PraxisGateStatusActivatable = "activatable"
	PraxisGateClaimBoundary     = "A computed verdict on whether Praxis regulated reliance COULD be activated on the Nomos " +
		"proof as recorded in the repository. 'activatable' is not an activation: the decision, its approvals and " +
		"its record belong to the regulated roadmap (docs/28). 'blocked' names every unmet requirement."
	CodePraxisGateRecord  = "PRAXIS_GATE_RECORD_INVALID"
	CodePraxisGateForged  = "PRAXIS_GATE_RECORD_INCONSISTENT"
	CodePraxisGateVerdict = "PRAXIS_GATE_VERDICT_INVALID"
)

// PraxisGateRecord is the part of praxis-activation-gate.yaml the engine reads.
type PraxisGateRecord struct {
	SchemaVersion      string `yaml:"schema_version"`
	RecordType         string `yaml:"record_type"`
	ActivationID       string `yaml:"activation_id"`
	CurrentStatus      string `yaml:"current_status"`
	ClaimBoundary      string `yaml:"claim_boundary"`
	NomosRequiredProof struct {
		RequiredAQStatus              string `yaml:"required_aq_status"`
		RequiredReconstructionVerdict string `yaml:"required_reconstruction_verdict"`
		RequiredStrictGateStatus      string `yaml:"required_strict_gate_status"`
		RequiredArtifacts             []struct {
			Path          string `yaml:"path"`
			Role          string `yaml:"role"`
			RequiredState string `yaml:"required_state"`
		} `yaml:"required_artifacts"`
		RequiredReviews []string `yaml:"required_reviews"`
	} `yaml:"nomos_required_proof"`
	ConsumerGuard struct {
		PraxisMayConsumeUnverified *bool `yaml:"praxis_may_consume_unverified_nomos_atoms_as_regulated_evidence"`
	} `yaml:"consumer_guard"`
}

// PraxisGateCheck is one resolved requirement.
type PraxisGateCheck struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Required string `json:"required"`
	Actual   string `json:"actual"`
	Source   string `json:"source"`
	Met      bool   `json:"met"`
	Reason   string `json:"reason,omitempty"`
}

// PraxisActivationVerdict is the engine's output.
type PraxisActivationVerdict struct {
	SchemaVersion string            `json:"schema_version"`
	GeneratedAt   string            `json:"generated_at"`
	RecordPath    string            `json:"record_path"`
	RecordSha256  string            `json:"record_sha256"`
	ActivationID  string            `json:"activation_id"`
	RecordStatus  string            `json:"record_status"`
	Status        string            `json:"status"`
	Checks        []PraxisGateCheck `json:"checks"`
	UnmetCount    int               `json:"unmet_count"`
	Reasons       []string          `json:"reasons"`
	ClaimBoundary string            `json:"claim_boundary"`
}

// LoadPraxisGateRecord reads the record (unknown top-level keys tolerated: the
// record carries prose fields the engine does not interpret).
func LoadPraxisGateRecord(path string) (PraxisGateRecord, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PraxisGateRecord{}, nil, praxisMapErr(CodePraxisGateRecord, "read record: %v", err)
	}
	var rec PraxisGateRecord
	if err := yaml.Unmarshal(raw, &rec); err != nil {
		return PraxisGateRecord{}, raw, praxisMapErr(CodePraxisGateRecord, "parse record: %v", err)
	}
	if rec.RecordType != "praxis_activation_gate" {
		return rec, raw, praxisMapErr(CodePraxisGateRecord, "record_type %q, want praxis_activation_gate", rec.RecordType)
	}
	if strings.TrimSpace(rec.ActivationID) == "" || strings.TrimSpace(rec.CurrentStatus) == "" {
		return rec, raw, praxisMapErr(CodePraxisGateRecord, "activation_id and current_status are required")
	}
	if len(rec.NomosRequiredProof.RequiredArtifacts) == 0 || len(rec.NomosRequiredProof.RequiredReviews) == 0 {
		return rec, raw, praxisMapErr(CodePraxisGateRecord, "a gate without required artifacts or reviews gates nothing")
	}
	if rec.ConsumerGuard.PraxisMayConsumeUnverified == nil || *rec.ConsumerGuard.PraxisMayConsumeUnverified {
		return rec, raw, praxisMapErr(CodePraxisGateForged, "consumer_guard must state praxis_may_consume_unverified_nomos_atoms_as_regulated_evidence: false")
	}
	return rec, raw, nil
}

// EvaluatePraxisActivation resolves every requirement against repoRoot.
func EvaluatePraxisActivation(recordPath, repoRoot string, now time.Time) (PraxisActivationVerdict, error) {
	rec, raw, err := LoadPraxisGateRecord(recordPath)
	if err != nil {
		return PraxisActivationVerdict{}, err
	}
	sum := sha256.Sum256(raw)
	v := PraxisActivationVerdict{
		SchemaVersion: PraxisGateVerdictSchema, GeneratedAt: now.UTC().Format(time.RFC3339), RecordPath: recordPath,
		RecordSha256: "sha256:" + hex.EncodeToString(sum[:]), ActivationID: rec.ActivationID, RecordStatus: rec.CurrentStatus,
		ClaimBoundary: PraxisGateClaimBoundary,
	}
	p := rec.NomosRequiredProof
	v.Checks = append(v.Checks,
		resolveDecisionRecord(repoRoot, "aq_status", "acceptance_qualification_decision", p.RequiredAQStatus),
		resolveDecisionRecord(repoRoot, "reconstruction_verdict", "reconstruction_verdict", p.RequiredReconstructionVerdict),
		resolveDecisionRecord(repoRoot, "strict_gate_status", "qualified_corpus_strict_gate_verdict", p.RequiredStrictGateStatus),
	)
	for _, a := range p.RequiredArtifacts {
		v.Checks = append(v.Checks, resolveArtifactState(repoRoot, a.Path, a.Role, a.RequiredState))
	}
	for _, r := range p.RequiredReviews {
		v.Checks = append(v.Checks, resolveReview(repoRoot, r))
	}
	for _, c := range v.Checks {
		if !c.Met {
			v.UnmetCount++
			v.Reasons = append(v.Reasons, fmt.Sprintf("%s: required %s, actual %s (%s)", c.ID, c.Required, c.Actual, c.Reason))
		}
	}
	if v.UnmetCount == 0 {
		v.Status = PraxisGateStatusActivatable
	} else {
		v.Status = PraxisGateStatusBlocked
	}
	// A record that claims more than the checks support is forged, not trusted.
	if v.Status == PraxisGateStatusBlocked && !strings.HasPrefix(rec.CurrentStatus, "blocked") {
		return v, praxisMapErr(CodePraxisGateForged, "record current_status %q while %d requirement(s) are unmet — the record claims more than the proof supports", rec.CurrentStatus, v.UnmetCount)
	}
	if strings.EqualFold(rec.CurrentStatus, "activated") {
		return v, praxisMapErr(CodePraxisGateForged, "record current_status \"activated\": activation is a human decision recorded under docs/28, never a gate record state")
	}
	sort.Strings(v.Reasons)
	return v, nil
}

// VerifyPraxisActivationVerdict re-reads a verdict file: shape, status vocabulary,
// and the record hash it was computed from. A verdict that says "activated" is refused.
func VerifyPraxisActivationVerdict(path, repoRoot string) (PraxisActivationVerdict, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PraxisActivationVerdict{}, praxisMapErr(CodePraxisGateVerdict, "read verdict: %v", err)
	}
	var v PraxisActivationVerdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return PraxisActivationVerdict{}, praxisMapErr(CodePraxisGateVerdict, "parse verdict: %v", err)
	}
	if v.SchemaVersion != PraxisGateVerdictSchema {
		return v, praxisMapErr(CodePraxisGateVerdict, "schema_version %q, want %q", v.SchemaVersion, PraxisGateVerdictSchema)
	}
	switch v.Status {
	case PraxisGateStatusBlocked, PraxisGateStatusActivatable:
	default:
		return v, praxisMapErr(CodePraxisGateVerdict, "status %q is not blocked|activatable — a verdict never records an activation", v.Status)
	}
	if v.Status == PraxisGateStatusActivatable && v.UnmetCount != 0 {
		return v, praxisMapErr(CodePraxisGateVerdict, "activatable with %d unmet requirement(s) is a contradiction", v.UnmetCount)
	}
	unmet := 0
	for _, c := range v.Checks {
		if !c.Met {
			unmet++
		}
	}
	if unmet != v.UnmetCount || (v.Status == PraxisGateStatusActivatable && unmet != 0) {
		return v, praxisMapErr(CodePraxisGateVerdict, "checks say %d unmet, verdict says %d", unmet, v.UnmetCount)
	}
	if repoRoot != "" {
		got, err := sha256File(filepath.Join(repoRoot, filepath.FromSlash(v.RecordPath)))
		if err != nil || got != v.RecordSha256 {
			return v, praxisMapErr(CodePraxisGateVerdict, "record %s no longer matches the verdict's record_sha256 (%v)", v.RecordPath, err)
		}
	}
	return v, nil
}

// ---- resolvers -----------------------------------------------------------

var mdStatusRe = regexp.MustCompile(`(?m)^Status:\s*([A-Za-z_-]+)`)

// resolveDecisionRecord looks for a decision record of the given type under
// docs/regulated/qualification/decisions/. None → not met, honestly.
func resolveDecisionRecord(repoRoot, id, recordType, required string) PraxisGateCheck {
	c := PraxisGateCheck{ID: id, Kind: "decision_record", Required: required, Source: "docs/regulated/qualification/decisions/*.yaml (record_type " + recordType + ")"}
	dir := filepath.Join(repoRoot, "docs", "regulated", "qualification", "decisions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.Actual, c.Reason = "no_record", "no decision record directory exists"
		return c
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		var doc struct {
			RecordType string `yaml:"record_type"`
			Status     string `yaml:"status"`
			DecidedBy  string `yaml:"decided_by"`
			DecidedAt  string `yaml:"decided_at"`
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || yaml.Unmarshal(raw, &doc) != nil || doc.RecordType != recordType {
			continue
		}
		c.Source = filepath.ToSlash(filepath.Join("docs/regulated/qualification/decisions", e.Name()))
		if strings.TrimSpace(doc.DecidedBy) == "" || strings.TrimSpace(doc.DecidedAt) == "" {
			c.Actual, c.Reason = doc.Status+"_unsigned", "decision record lacks decided_by/decided_at"
			return c
		}
		c.Actual = doc.Status
		c.Met = doc.Status == required
		if !c.Met {
			c.Reason = "decision recorded with another status"
		}
		return c
	}
	c.Actual, c.Reason = "no_record", "no decision record of this type exists"
	return c
}

// resolveArtifactState reads the ACTUAL state of a required artifact by role.
func resolveArtifactState(repoRoot, rel, role, required string) PraxisGateCheck {
	c := PraxisGateCheck{ID: "artifact:" + role, Kind: "artifact", Required: required, Source: rel}
	full := filepath.Join(repoRoot, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		c.Actual, c.Reason = "missing", "artifact file is absent"
		return c
	}
	switch role {
	case "installation_baseline", "operational_gate_protocol":
		var doc struct {
			Acceptance struct {
				Status     string `yaml:"status"`
				AcceptedBy string `yaml:"accepted_by"`
				AcceptedAt string `yaml:"accepted_at"`
			} `yaml:"acceptance"`
		}
		if yaml.Unmarshal(raw, &doc) != nil {
			c.Actual, c.Reason = "unreadable", "record does not parse"
			return c
		}
		if doc.Acceptance.Status == "" {
			c.Actual, c.Reason = "recorded_without_acceptance", "the record exists but carries no acceptance block (status, accepted_by, accepted_at)"
			return c
		}
		if doc.Acceptance.AcceptedBy == "" || doc.Acceptance.AcceptedAt == "" {
			c.Actual, c.Reason = doc.Acceptance.Status+"_unsigned", "acceptance lacks accepted_by/accepted_at"
			return c
		}
		c.Actual = doc.Acceptance.Status
		c.Met = c.Actual == required
	case "production_journey_evidence":
		var doc struct {
			OverallResult string `yaml:"overall_result"`
		}
		if yaml.Unmarshal(raw, &doc) != nil || doc.OverallResult == "" {
			c.Actual, c.Reason = "unreadable", "no overall_result"
			return c
		}
		c.Actual = strings.ToLower(doc.OverallResult)
		c.Met = c.Actual == required
		if !c.Met {
			c.Reason = "overall_result is not " + required
		}
	case "validation_inventory":
		var doc struct {
			Validations []struct {
				ID           string `yaml:"id"`
				RiskLevel    string `yaml:"risk_level"`
				LastVerified string `yaml:"last_verified"`
				Waiver       string `yaml:"waiver"`
			} `yaml:"validations"`
		}
		if yaml.Unmarshal(raw, &doc) != nil {
			c.Actual, c.Reason = "unreadable", "inventory does not parse"
			return c
		}
		var open []string
		for _, val := range doc.Validations {
			if (val.RiskLevel == "high" || val.RiskLevel == "critical") && strings.TrimSpace(val.LastVerified) == "" && strings.TrimSpace(val.Waiver) == "" {
				open = append(open, val.ID)
			}
		}
		if len(open) == 0 {
			c.Actual, c.Met = required, true
		} else {
			c.Actual = fmt.Sprintf("%d_open_high_or_critical_unverified", len(open))
			c.Reason = "high/critical validations with no last_verified and no waiver: " + strings.Join(open, ", ")
		}
	case "evidence_contract":
		m := mdStatusRe.FindSubmatch(raw)
		if m == nil {
			c.Actual, c.Reason = "no_status_line", "document has no 'Status:' line"
			return c
		}
		c.Actual = strings.ToLower(string(m[1]))
		c.Met = c.Actual == required
		if !c.Met {
			c.Reason = "document status is not " + required
		}
	default:
		c.Actual, c.Reason = "unknown_role", "no resolver for this role — unknown is not met"
	}
	return c
}

// resolveReview looks for a completed, signed human review record.
func resolveReview(repoRoot, reviewID string) PraxisGateCheck {
	rel := filepath.ToSlash(filepath.Join("docs/regulated/qualification/reviews", reviewID+".yaml"))
	c := PraxisGateCheck{ID: "review:" + reviewID, Kind: "human_review", Required: "completed", Source: rel}
	raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		c.Actual, c.Reason = "missing", "no review record; a review is a human act and cannot be generated"
		return c
	}
	var doc struct {
		Status     string `yaml:"status"`
		ReviewedBy string `yaml:"reviewed_by"`
		ReviewedAt string `yaml:"reviewed_at"`
	}
	if yaml.Unmarshal(raw, &doc) != nil || doc.Status == "" {
		c.Actual, c.Reason = "unreadable", "review record lacks a status"
		return c
	}
	if doc.ReviewedBy == "" || doc.ReviewedAt == "" {
		c.Actual, c.Reason = doc.Status+"_unsigned", "review lacks reviewed_by/reviewed_at"
		return c
	}
	c.Actual = doc.Status
	c.Met = doc.Status == "completed"
	if !c.Met {
		c.Reason = "review not completed"
	}
	return c
}
