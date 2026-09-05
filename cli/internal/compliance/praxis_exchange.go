package compliance

// NRT-016 (#660) — the Nomos ⇄ Praxis evidence exchange, verified on the Go
// side. The CUE contract (specs/nomos-praxis-evidence.cue) is the normative
// shape; this engine re-implements the rules that matter at runtime and adds
// what CUE cannot do: recompute the referenced artifact hashes against a
// repository root. Fail closed: every rule is a named refusal.
//
// The engine never activates anything. `reliance: regulated_evidence` is
// accepted only when every Nomos artifact is verified with a record AND an
// activation verdict is bound — and even then it records, it does not decide.

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

	"gopkg.in/yaml.v3"
)

const (
	PraxisExchangeSchema        = "nomos-praxis-evidence-exchange-v1"
	PraxisExchangeClaimBoundary = "A shared Nomos→Praxis evidence contract, verified for shape, referential " +
		"integrity and artifact hashes. It does not activate Praxis, does not qualify Praxis evidence, and " +
		"does not turn an unverified Nomos artifact into regulated evidence."
)

const (
	CodePraxisSchema              = "PRAXIS_SCHEMA_INVALID"
	CodePraxisShape               = "PRAXIS_SHAPE_INVALID"
	CodePraxisAuthority           = "PRAXIS_AUTHORITY_INVERTED"
	CodePraxisReference           = "PRAXIS_REFERENCE_DANGLING"
	CodePraxisRelianceUnsupported = "PRAXIS_RELIANCE_UNSUPPORTED"
	CodePraxisArtifactHash        = "PRAXIS_ARTIFACT_HASH_MISMATCH"
	CodePraxisArtifactMissing     = "PRAXIS_ARTIFACT_MISSING"
	CodePraxisRecordHash          = "PRAXIS_VERIFICATION_RECORD_MISMATCH"
)

// PraxisError is a named refusal.
type PraxisError struct {
	Code    string
	Message string
}

func (e *PraxisError) Error() string { return e.Code + ": " + e.Message }

func praxisErr(code, format string, args ...any) error {
	return &PraxisError{Code: code, Message: fmt.Sprintf(format, args...)}
}

var (
	praxisSha256 = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	praxisIdent  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	praxisTime   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)
)

var praxisArtifactKinds = map[string]bool{
	"corpus_attestation": true, "release_candidate_manifest": true, "control_matrix": true, "lawbook_feed": true,
	"atom_set": true, "body_ledger": true, "canonical_matrix": true, "evidence_ledger": true,
}

// PraxisVerification is an artifact's verification state as NOMOS records it.
type PraxisVerification struct {
	State        string `yaml:"state" json:"state"`
	RecordPath   string `yaml:"record_path,omitempty" json:"record_path,omitempty"`
	RecordSha256 string `yaml:"record_sha256,omitempty" json:"record_sha256,omitempty"`
}

// NomosArtifactRef names one artifact handed to Praxis.
type NomosArtifactRef struct {
	ArtifactID   string             `yaml:"artifact_id" json:"artifact_id"`
	Kind         string             `yaml:"kind" json:"kind"`
	Path         string             `yaml:"path" json:"path"`
	Sha256       string             `yaml:"sha256" json:"sha256"`
	Verification PraxisVerification `yaml:"verification" json:"verification"`
	ClaimIDs     []string           `yaml:"claim_ids,omitempty" json:"claim_ids,omitempty"`
}

// PraxisScenario is downstream runtime evidence.
type PraxisScenario struct {
	ScenarioID     string   `yaml:"scenario_id" json:"scenario_id"`
	TestID         string   `yaml:"test_id" json:"test_id"`
	NomosClaimIDs  []string `yaml:"nomos_claim_ids" json:"nomos_claim_ids"`
	NomosAtomIDs   []string `yaml:"nomos_atom_ids" json:"nomos_atom_ids"`
	Result         string   `yaml:"result" json:"result"`
	EvidenceSha256 string   `yaml:"evidence_sha256" json:"evidence_sha256"`
	EvidenceRef    string   `yaml:"evidence_ref" json:"evidence_ref"`
	ExecutedAt     string   `yaml:"executed_at" json:"executed_at"`
	PraxisVersion  string   `yaml:"praxis_version" json:"praxis_version"`
}

// RuntimeFinding is raised by a scenario.
type RuntimeFinding struct {
	FindingID  string `yaml:"finding_id" json:"finding_id"`
	ScenarioID string `yaml:"scenario_id" json:"scenario_id"`
	Severity   string `yaml:"severity" json:"severity"`
	Status     string `yaml:"status" json:"status"`
	Summary    string `yaml:"summary" json:"summary"`
	CapaID     string `yaml:"capa_id,omitempty" json:"capa_id,omitempty"`
}

// CapaStatus closes the loop on findings against controls.
type CapaStatus struct {
	CapaID     string   `yaml:"capa_id" json:"capa_id"`
	FindingIDs []string `yaml:"finding_ids" json:"finding_ids"`
	ControlIDs []string `yaml:"control_ids" json:"control_ids"`
	Status     string   `yaml:"status" json:"status"`
	Owner      string   `yaml:"owner" json:"owner"`
	OpenedAt   string   `yaml:"opened_at" json:"opened_at"`
	ClosedAt   string   `yaml:"closed_at,omitempty" json:"closed_at,omitempty"`
}

// PraxisProduct names a side of the exchange.
type PraxisProduct struct {
	Product string `yaml:"product" json:"product"`
	Version string `yaml:"version" json:"version"`
}

// PraxisEvidenceExchange is the whole document.
type PraxisEvidenceExchange struct {
	SchemaVersion           string             `yaml:"schema_version" json:"schema_version"`
	ExchangeID              string             `yaml:"exchange_id" json:"exchange_id"`
	GeneratedAt             string             `yaml:"generated_at" json:"generated_at"`
	Producer                PraxisProduct      `yaml:"producer" json:"producer"`
	Consumer                PraxisProduct      `yaml:"consumer" json:"consumer"`
	NomosArtifacts          []NomosArtifactRef `yaml:"nomos_artifacts" json:"nomos_artifacts"`
	PraxisScenarios         []PraxisScenario   `yaml:"praxis_scenarios" json:"praxis_scenarios"`
	RuntimeFindings         []RuntimeFinding   `yaml:"runtime_findings" json:"runtime_findings"`
	Capa                    []CapaStatus       `yaml:"capa" json:"capa"`
	Reliance                string             `yaml:"reliance" json:"reliance"`
	ActivationVerdictPath   string             `yaml:"activation_verdict_path,omitempty" json:"activation_verdict_path,omitempty"`
	ActivationVerdictSha256 string             `yaml:"activation_verdict_sha256,omitempty" json:"activation_verdict_sha256,omitempty"`
	ClaimBoundary           string             `yaml:"claim_boundary" json:"claim_boundary"`
}

// LoadPraxisExchange reads YAML or JSON (JSON is YAML).
func LoadPraxisExchange(path string) (PraxisEvidenceExchange, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PraxisEvidenceExchange{}, praxisErr(CodePraxisShape, "read exchange: %v", err)
	}
	var ex PraxisEvidenceExchange
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&ex); err != nil {
		return PraxisEvidenceExchange{}, praxisErr(CodePraxisShape, "parse exchange: %v", err)
	}
	return ex, nil
}

// PraxisExchangeReport is the verifier's output.
type PraxisExchangeReport struct {
	SchemaVersion     string   `json:"schema_version"`
	ExchangeID        string   `json:"exchange_id"`
	Reliance          string   `json:"reliance"`
	Artifacts         int      `json:"artifacts"`
	VerifiedArtifacts int      `json:"verified_artifacts"`
	Scenarios         int      `json:"scenarios"`
	Findings          int      `json:"findings"`
	Capa              int      `json:"capa"`
	HashesChecked     bool     `json:"hashes_checked_against_tree"`
	ExchangeDigest    string   `json:"exchange_digest"`
	Checks            []string `json:"checks"`
	ClaimBoundary     string   `json:"claim_boundary"`
}

// VerifyPraxisExchange applies every rule; repoRoot == "" skips hash recomputation
// (and the report says so). Any failure is a *PraxisError.
func VerifyPraxisExchange(ex PraxisEvidenceExchange, repoRoot string) (PraxisExchangeReport, error) {
	rep := PraxisExchangeReport{SchemaVersion: "nomos-praxis-exchange-report-v1", ExchangeID: ex.ExchangeID, Reliance: ex.Reliance, ClaimBoundary: PraxisExchangeClaimBoundary}
	if ex.SchemaVersion != PraxisExchangeSchema {
		return rep, praxisErr(CodePraxisSchema, "schema_version %q, want %q", ex.SchemaVersion, PraxisExchangeSchema)
	}
	if !praxisIdent.MatchString(ex.ExchangeID) {
		return rep, praxisErr(CodePraxisShape, "exchange_id %q is not an identifier", ex.ExchangeID)
	}
	if !praxisTime.MatchString(ex.GeneratedAt) {
		return rep, praxisErr(CodePraxisShape, "generated_at %q is not an RFC3339 UTC timestamp", ex.GeneratedAt)
	}
	// Authority: Nomos produces, Praxis consumes. Never the other way round.
	if ex.Producer.Product != "nomos" || ex.Consumer.Product != "praxis" {
		return rep, praxisErr(CodePraxisAuthority, "producer must be nomos and consumer praxis, got producer=%q consumer=%q", ex.Producer.Product, ex.Consumer.Product)
	}
	if strings.TrimSpace(ex.Producer.Version) == "" || strings.TrimSpace(ex.Consumer.Version) == "" {
		return rep, praxisErr(CodePraxisShape, "producer and consumer versions are required")
	}
	if len(ex.NomosArtifacts) == 0 {
		return rep, praxisErr(CodePraxisShape, "an exchange without Nomos artifacts is not an exchange")
	}
	if len(strings.Fields(ex.ClaimBoundary)) < 6 {
		return rep, praxisErr(CodePraxisShape, "claim_boundary must be a real sentence, got %q", ex.ClaimBoundary)
	}
	rep.Checks = append(rep.Checks, "schema", "authority")

	seenArtifacts := map[string]bool{}
	verified := 0
	for _, a := range ex.NomosArtifacts {
		if !praxisIdent.MatchString(a.ArtifactID) || seenArtifacts[a.ArtifactID] {
			return rep, praxisErr(CodePraxisShape, "artifact_id %q missing, malformed or duplicated", a.ArtifactID)
		}
		seenArtifacts[a.ArtifactID] = true
		if !praxisArtifactKinds[a.Kind] {
			return rep, praxisErr(CodePraxisShape, "artifact %s: kind %q is not a Nomos artifact kind", a.ArtifactID, a.Kind)
		}
		if strings.TrimSpace(a.Path) == "" || !praxisSha256.MatchString(a.Sha256) {
			return rep, praxisErr(CodePraxisShape, "artifact %s: path and sha256:<64 hex> are required", a.ArtifactID)
		}
		switch a.Verification.State {
		case "verified":
			if strings.TrimSpace(a.Verification.RecordPath) == "" || !praxisSha256.MatchString(a.Verification.RecordSha256) {
				return rep, praxisErr(CodePraxisShape, "artifact %s: a verified artifact must name its verification record and its sha256", a.ArtifactID)
			}
			verified++
		case "unverified", "failed":
		default:
			return rep, praxisErr(CodePraxisShape, "artifact %s: verification.state %q is not verified|unverified|failed", a.ArtifactID, a.Verification.State)
		}
		for _, c := range a.ClaimIDs {
			if !praxisIdent.MatchString(c) {
				return rep, praxisErr(CodePraxisShape, "artifact %s: claim id %q malformed", a.ArtifactID, c)
			}
		}
	}
	rep.Artifacts, rep.VerifiedArtifacts = len(ex.NomosArtifacts), verified
	rep.Checks = append(rep.Checks, "artifacts")

	// Scenarios, findings, CAPA: shape + referential integrity.
	scenarios := map[string]bool{}
	for _, s := range ex.PraxisScenarios {
		if !praxisIdent.MatchString(s.ScenarioID) || scenarios[s.ScenarioID] {
			return rep, praxisErr(CodePraxisShape, "scenario_id %q missing, malformed or duplicated", s.ScenarioID)
		}
		scenarios[s.ScenarioID] = true
		if !praxisIdent.MatchString(s.TestID) || !praxisSha256.MatchString(s.EvidenceSha256) || strings.TrimSpace(s.EvidenceRef) == "" ||
			!praxisTime.MatchString(s.ExecutedAt) || strings.TrimSpace(s.PraxisVersion) == "" {
			return rep, praxisErr(CodePraxisShape, "scenario %s: test_id, evidence_sha256, evidence_ref, executed_at and praxis_version are required", s.ScenarioID)
		}
		switch s.Result {
		case "pass", "fail", "blocked", "not_run":
		default:
			return rep, praxisErr(CodePraxisShape, "scenario %s: result %q is not pass|fail|blocked|not_run", s.ScenarioID, s.Result)
		}
		for _, id := range append(append([]string{}, s.NomosClaimIDs...), s.NomosAtomIDs...) {
			if !praxisIdent.MatchString(id) {
				return rep, praxisErr(CodePraxisShape, "scenario %s: Nomos id %q malformed", s.ScenarioID, id)
			}
		}
	}
	findings := map[string]bool{}
	for _, f := range ex.RuntimeFindings {
		if !praxisIdent.MatchString(f.FindingID) || findings[f.FindingID] {
			return rep, praxisErr(CodePraxisShape, "finding_id %q missing, malformed or duplicated", f.FindingID)
		}
		findings[f.FindingID] = true
		if !scenarios[f.ScenarioID] {
			return rep, praxisErr(CodePraxisReference, "finding %s references scenario %q which is not in the exchange", f.FindingID, f.ScenarioID)
		}
		if !inSet(f.Severity, "critical", "major", "minor", "observation") || !inSet(f.Status, "open", "mitigated", "closed") || strings.TrimSpace(f.Summary) == "" {
			return rep, praxisErr(CodePraxisShape, "finding %s: severity, status and summary are required and enumerated", f.FindingID)
		}
	}
	capas := map[string]bool{}
	for _, c := range ex.Capa {
		if !praxisIdent.MatchString(c.CapaID) || capas[c.CapaID] {
			return rep, praxisErr(CodePraxisShape, "capa_id %q missing, malformed or duplicated", c.CapaID)
		}
		capas[c.CapaID] = true
		if len(c.FindingIDs) == 0 {
			return rep, praxisErr(CodePraxisShape, "capa %s: a CAPA without findings has no cause", c.CapaID)
		}
		for _, fid := range c.FindingIDs {
			if !findings[fid] {
				return rep, praxisErr(CodePraxisReference, "capa %s references finding %q which is not in the exchange", c.CapaID, fid)
			}
		}
		if !inSet(c.Status, "open", "in_progress", "verified_effective", "closed") || strings.TrimSpace(c.Owner) == "" || !praxisTime.MatchString(c.OpenedAt) {
			return rep, praxisErr(CodePraxisShape, "capa %s: status, owner and opened_at are required and enumerated", c.CapaID)
		}
		if c.Status == "closed" && !praxisTime.MatchString(c.ClosedAt) {
			return rep, praxisErr(CodePraxisShape, "capa %s: a closed CAPA must carry closed_at", c.CapaID)
		}
	}
	for _, f := range ex.RuntimeFindings {
		if f.CapaID != "" && !capas[f.CapaID] {
			return rep, praxisErr(CodePraxisReference, "finding %s references CAPA %q which is not in the exchange", f.FindingID, f.CapaID)
		}
	}
	rep.Scenarios, rep.Findings, rep.Capa = len(ex.PraxisScenarios), len(ex.RuntimeFindings), len(ex.Capa)
	rep.Checks = append(rep.Checks, "scenarios", "findings", "capa", "references")

	// Reliance: the rule this contract exists for.
	switch ex.Reliance {
	case "not_qualified_external_input":
		if ex.ActivationVerdictPath != "" || ex.ActivationVerdictSha256 != "" {
			return rep, praxisErr(CodePraxisShape, "not-qualified input carries no activation verdict")
		}
	case "regulated_evidence":
		if verified != len(ex.NomosArtifacts) {
			return rep, praxisErr(CodePraxisRelianceUnsupported,
				"regulated_evidence requires every Nomos artifact verified; %d of %d are", verified, len(ex.NomosArtifacts))
		}
		if strings.TrimSpace(ex.ActivationVerdictPath) == "" || !praxisSha256.MatchString(ex.ActivationVerdictSha256) {
			return rep, praxisErr(CodePraxisRelianceUnsupported, "regulated_evidence requires a bound activation verdict (path + sha256)")
		}
	default:
		return rep, praxisErr(CodePraxisShape, "reliance %q is not not_qualified_external_input|regulated_evidence", ex.Reliance)
	}
	rep.Checks = append(rep.Checks, "reliance")

	// Hashes against the tree, when a root is given: the artifact bytes, the
	// verification records, the activation verdict.
	if repoRoot != "" {
		for _, a := range ex.NomosArtifacts {
			got, err := sha256File(filepath.Join(repoRoot, filepath.FromSlash(a.Path)))
			if err != nil {
				return rep, praxisErr(CodePraxisArtifactMissing, "artifact %s: %v", a.ArtifactID, err)
			}
			if got != a.Sha256 {
				return rep, praxisErr(CodePraxisArtifactHash, "artifact %s: tree has %s, exchange says %s", a.ArtifactID, got, a.Sha256)
			}
			if a.Verification.State == "verified" {
				got, err := sha256File(filepath.Join(repoRoot, filepath.FromSlash(a.Verification.RecordPath)))
				if err != nil || got != a.Verification.RecordSha256 {
					return rep, praxisErr(CodePraxisRecordHash, "artifact %s: verification record %s does not match (%v)", a.ArtifactID, a.Verification.RecordPath, err)
				}
			}
		}
		if ex.Reliance == "regulated_evidence" {
			got, err := sha256File(filepath.Join(repoRoot, filepath.FromSlash(ex.ActivationVerdictPath)))
			if err != nil || got != ex.ActivationVerdictSha256 {
				return rep, praxisErr(CodePraxisRelianceUnsupported, "activation verdict %s does not match its bound sha256 (%v)", ex.ActivationVerdictPath, err)
			}
		}
		rep.HashesChecked = true
		rep.Checks = append(rep.Checks, "artifact_hashes")
	}
	canon, _ := json.Marshal(ex)
	sum := sha256.Sum256(canon)
	rep.ExchangeDigest = "sha256:" + hex.EncodeToString(sum[:])
	sort.Strings(rep.Checks)
	return rep, nil
}

func inSet(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func sha256File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
