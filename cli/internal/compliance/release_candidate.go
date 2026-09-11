package compliance

// #639 — a release CANDIDATE bundle: assembled from the commit, the gates, the
// artifacts, the open gaps, the deviations and the waivers, and verified on
// CONTENT and STATUS, not on the presence of files alone.
//
// What a candidate is not: a release. The tool never establishes an approval
// (that is a human act under the release SOP, #561), never claims a release
// was executed, and never turns the VRC-14 repeated-CI measure into a
// technical precondition — it records it as a risk with its measured value.
// Every rule below refuses the candidate outright; a refused candidate writes
// nothing.

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	CandidateSpecSchema   = "nomos-release-candidate-spec-v1"
	CandidateFormat       = "nomos.release-candidate.v1"
	GateEvidenceSchema    = "nomos-release-candidate-gates-v1"
	CandidateManifestName = "candidate-manifest.json"
	CandidateGatesName    = "gates.json"
	// CandidateClaimBoundary travels with every manifest.
	CandidateClaimBoundary = "A release CANDIDATE: artifacts, gates, open gaps, deviations and waivers " +
		"bound to one commit and verifiable offline. approval_status is pending and can only be " +
		"changed by an authentic human approval recorded under the release SOP; no release, tag or " +
		"notes were published; the VRC-14 repeated-CI measure is recorded as a risk, not as a gate."
)

// Refusal codes — each one has an adversarial test.
const (
	CodeCandidateApprovalNotPending = "CANDIDATE_APPROVAL_NOT_PENDING"
	CodeCandidateApprovalInvented   = "CANDIDATE_APPROVAL_INVENTED"
	CodeCandidateReleaseClaimed     = "CANDIDATE_RELEASE_CLAIMED"
	CodeCandidateArtifactMissing    = "CANDIDATE_ARTIFACT_MISSING"
	CodeCandidateArtifactUnreadable = "CANDIDATE_ARTIFACT_UNREADABLE"
	CodeCandidateGapUnacknowledged  = "CANDIDATE_GAP_UNACKNOWLEDGED"
	CodeCandidateGapStatusMismatch  = "CANDIDATE_GAP_STATUS_MISMATCH"
	CodeCandidateGateMissing        = "CANDIDATE_GATE_MISSING"
	CodeCandidateGateFailed         = "CANDIDATE_GATE_FAILED"
	CodeCandidateGateCommitMismatch = "CANDIDATE_GATE_COMMIT_MISMATCH"
	CodeCandidateRiskUnreadable     = "CANDIDATE_RISK_UNREADABLE"
	CodeCandidateWaiverIncomplete   = "CANDIDATE_WAIVER_INCOMPLETE"
	CodeCandidateDeviationInvalid   = "CANDIDATE_DEVIATION_INVALID"
	CodeCandidateSpecInvalid        = "CANDIDATE_SPEC_INVALID"
	CodeCandidateTampered           = "CANDIDATE_TAMPERED"
	CodeCandidateManifestInvalid    = "CANDIDATE_MANIFEST_INVALID"
)

// CandidateError is a refusal with a code; refusals are never warnings.
type CandidateError struct {
	Code    string
	Message string
}

func (e *CandidateError) Error() string { return e.Code + ": " + e.Message }

func refuse(code, format string, args ...any) error {
	return &CandidateError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// CandidateSpec is the versioned, human-edited declaration of a candidate.
type CandidateSpec struct {
	SchemaVersion    string           `yaml:"schema_version" json:"schema_version"`
	Product          string           `yaml:"product" json:"product"`
	Version          string           `yaml:"version" json:"version"`
	TargetLevel      QualityLevel     `yaml:"target_level" json:"target_level"`
	ApprovalStatus   string           `yaml:"approval_status" json:"approval_status"`
	Approvals        []map[string]any `yaml:"approvals" json:"approvals"`
	ReleaseExecuted  bool             `yaml:"release_executed" json:"release_executed"`
	EvidenceLedger   string           `yaml:"evidence_ledger" json:"evidence_ledger"`
	GapsAcknowledged []string         `yaml:"gaps_acknowledged" json:"gaps_acknowledged"`
	Risks            []RiskSpec       `yaml:"risks" json:"risks"`
	Gates            GateSpec         `yaml:"gates" json:"gates"`
	Waivers          []string         `yaml:"waivers" json:"waivers"`
	Deviations       []DeviationSpec  `yaml:"deviations" json:"deviations"`
	ClaimBoundary    string           `yaml:"claim_boundary" json:"claim_boundary"`
}

// RiskSpec names a measured input that is recorded, never gated on.
type RiskSpec struct {
	ID     string `yaml:"id" json:"id"`
	Kind   string `yaml:"kind" json:"kind"`
	Source string `yaml:"source" json:"source"`
	Note   string `yaml:"note" json:"note"`
}

// GateSpec lists the gate ids whose evidence must be present and green.
type GateSpec struct {
	Required []string `yaml:"required" json:"required"`
}

// DeviationSpec is a declared deviation with its record.
type DeviationSpec struct {
	ID     string `yaml:"id" json:"id"`
	Status string `yaml:"status" json:"status"`
	Record string `yaml:"record" json:"record"`
}

// GateEvidence is produced by the rehearsal (scripts/release_candidate_gates.py):
// the gates it actually ran, on which commit, with which exit codes.
type GateEvidence struct {
	SchemaVersion string       `json:"schema_version"`
	Commit        string       `json:"commit"`
	MeasuredAt    string       `json:"measured_at"`
	Gates         []GateResult `json:"gates"`
}

// GateResult is one gate's measured outcome.
type GateResult struct {
	ID       string  `json:"id"`
	Command  string  `json:"command"`
	ExitCode int     `json:"exit_code"`
	Status   string  `json:"status"`
	Seconds  float64 `json:"seconds,omitempty"`
}

// GapRecord is an open blocking gap carried into the candidate as-is.
type GapRecord struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Status       string   `json:"status"`
	BlocksClaims []string `json:"blocks_claims,omitempty"`
}

// RiskRecord is the measured value of a risk input.
type RiskRecord struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Source   string         `json:"source"`
	Measured map[string]any `json:"measured"`
	Blocking bool           `json:"blocking"`
	Note     string         `json:"note,omitempty"`
}

// WaiverRecord summarises a waiver file: what it says, not what it means.
type WaiverRecord struct {
	Path          string `json:"path"`
	DocumentID    string `json:"document_id,omitempty"`
	Status        string `json:"status"`
	WaivedRecords int    `json:"waived_records"`
	Hash          string `json:"hash"`
}

// CandidateManifest is the verifiable output.
type CandidateManifest struct {
	Format          string           `json:"format"`
	Product         string           `json:"product"`
	Version         string           `json:"version"`
	TargetLevel     QualityLevel     `json:"target_level"`
	Commit          string           `json:"commit"`
	GeneratedAt     string           `json:"generated_at"`
	GeneratedBy     string           `json:"generated_by"`
	ApprovalStatus  string           `json:"approval_status"`
	ReleaseExecuted bool             `json:"release_executed"`
	Artifacts       []ArtifactResult `json:"artifacts"`
	Missing         []string         `json:"missing"`
	Complete        bool             `json:"complete"`
	Gates           []GateResult     `json:"gates"`
	GapsOpen        []GapRecord      `json:"gaps_open"`
	Risks           []RiskRecord     `json:"risks"`
	Waivers         []WaiverRecord   `json:"waivers"`
	Deviations      []DeviationSpec  `json:"deviations"`
	ArtifactsDigest string           `json:"artifacts_digest"`
	SpecDigest      string           `json:"spec_digest"`
	GatesDigest     string           `json:"gates_digest"`
	Verdict         string           `json:"verdict"`
	ClaimBoundary   string           `json:"claim_boundary"`
}

// CandidateInput drives AssembleCandidate.
type CandidateInput struct {
	SpecPath    string
	GatesPath   string
	RepoRoot    string
	Commit      string
	GeneratedBy string
	Now         time.Time
}

// LoadCandidateSpec reads and shape-checks the spec.
func LoadCandidateSpec(path string) (CandidateSpec, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CandidateSpec{}, nil, refuse(CodeCandidateSpecInvalid, "read spec: %v", err)
	}
	var spec CandidateSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return CandidateSpec{}, nil, refuse(CodeCandidateSpecInvalid, "parse spec: %v", err)
	}
	if spec.SchemaVersion != CandidateSpecSchema {
		return spec, raw, refuse(CodeCandidateSpecInvalid, "schema_version %q, want %q", spec.SchemaVersion, CandidateSpecSchema)
	}
	for name, v := range map[string]string{"product": spec.Product, "version": spec.Version, "evidence_ledger": spec.EvidenceLedger} {
		if strings.TrimSpace(v) == "" {
			return spec, raw, refuse(CodeCandidateSpecInvalid, "%s is required", name)
		}
	}
	if _, ok := map[QualityLevel]bool{LevelNQ0: true, LevelNQ1: true, LevelNQ3: true, LevelNQ5: true}[spec.TargetLevel]; !ok {
		return spec, raw, refuse(CodeCandidateSpecInvalid, "target_level %q is not a known quality level", spec.TargetLevel)
	}
	return spec, raw, nil
}

// LoadGateEvidence reads the gates file.
func LoadGateEvidence(path string) (GateEvidence, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GateEvidence{}, nil, refuse(CodeCandidateGateMissing, "read gate evidence: %v", err)
	}
	var ev GateEvidence
	if err := json.Unmarshal(raw, &ev); err != nil {
		return GateEvidence{}, nil, refuse(CodeCandidateGateMissing, "parse gate evidence: %v", err)
	}
	if ev.SchemaVersion != GateEvidenceSchema {
		return ev, raw, refuse(CodeCandidateGateMissing, "gate evidence schema_version %q, want %q", ev.SchemaVersion, GateEvidenceSchema)
	}
	return ev, raw, nil
}

// AssembleCandidate applies every rule and returns the manifest, or a refusal.
func AssembleCandidate(in CandidateInput) (CandidateManifest, error) {
	spec, specRaw, err := LoadCandidateSpec(in.SpecPath)
	if err != nil {
		return CandidateManifest{}, err
	}
	// Rule 1–3: the tool cannot approve, cannot record an approval, cannot claim a release.
	if spec.ApprovalStatus != "pending" {
		return CandidateManifest{}, refuse(CodeCandidateApprovalNotPending,
			"approval_status is %q; a candidate is always pending — an approval is a human act under the release SOP (#561), never a tool output", spec.ApprovalStatus)
	}
	if len(spec.Approvals) != 0 {
		return CandidateManifest{}, refuse(CodeCandidateApprovalInvented,
			"%d approval entry(ies) declared in the candidate spec; the tool has no way to establish an approval and refuses to carry one", len(spec.Approvals))
	}
	if spec.ReleaseExecuted {
		return CandidateManifest{}, refuse(CodeCandidateReleaseClaimed, "release_executed is true; a candidate bundle never records an executed release")
	}
	if strings.TrimSpace(in.Commit) == "" {
		return CandidateManifest{}, refuse(CodeCandidateSpecInvalid, "commit is required: a candidate is bound to one commit")
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	// Rule 4–5: artifacts present for the target level, and readable as what they claim to be.
	bundle, err := AssembleBundle(BundleInput{
		Product: spec.Product, Version: spec.Version, TargetLevel: spec.TargetLevel,
		Commit: in.Commit, RepoRoot: in.RepoRoot, GeneratedBy: in.GeneratedBy, Now: now,
	})
	if err != nil {
		return CandidateManifest{}, err
	}
	if !bundle.Manifest.Complete {
		return CandidateManifest{}, refuse(CodeCandidateArtifactMissing, "required artifact(s) missing for %s: %s",
			spec.TargetLevel, strings.Join(bundle.Manifest.Missing, ", "))
	}
	for _, a := range bundle.Manifest.Artifacts {
		if a.Status != StatusPresent {
			continue
		}
		if err := checkArtifactContent(filepath.Join(in.RepoRoot, filepath.FromSlash(a.Path))); err != nil {
			return CandidateManifest{}, refuse(CodeCandidateArtifactUnreadable, "%s (%s): %v", a.ID, a.Path, err)
		}
	}

	// Rule 6: every open blocking gap in the ledger is acknowledged, and nothing
	// acknowledged is closed or unknown in the ledger.
	gaps, err := loadOpenBlockingGaps(filepath.Join(in.RepoRoot, filepath.FromSlash(spec.EvidenceLedger)))
	if err != nil {
		return CandidateManifest{}, err
	}
	ack := map[string]bool{}
	for _, id := range spec.GapsAcknowledged {
		ack[id] = true
	}
	var open []GapRecord
	openIDs := map[string]bool{}
	for _, g := range gaps.all {
		if g.Status == "open" {
			openIDs[g.ID] = true
			if !ack[g.ID] {
				return CandidateManifest{}, refuse(CodeCandidateGapUnacknowledged,
					"ledger gap %s is open (%s) but not acknowledged by the candidate", g.ID, g.Severity)
			}
			open = append(open, g)
		}
	}
	for id := range ack {
		if !openIDs[id] {
			st := "unknown to the ledger"
			if g, ok := gaps.byID[id]; ok {
				st = "status " + g.Status + " in the ledger"
			}
			return CandidateManifest{}, refuse(CodeCandidateGapStatusMismatch, "candidate acknowledges %s as open but it is %s", id, st)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].ID < open[j].ID })

	// Rule 7: gates — evidence exists for this commit, every required gate ran and passed.
	ev, gatesRaw, err := LoadGateEvidence(in.GatesPath)
	if err != nil {
		return CandidateManifest{}, err
	}
	if ev.Commit != in.Commit {
		return CandidateManifest{}, refuse(CodeCandidateGateCommitMismatch, "gate evidence is for commit %s, candidate is %s", ev.Commit, in.Commit)
	}
	byGate := map[string]GateResult{}
	for _, g := range ev.Gates {
		byGate[g.ID] = g
	}
	var gates []GateResult
	for _, id := range spec.Gates.Required {
		g, ok := byGate[id]
		if !ok {
			return CandidateManifest{}, refuse(CodeCandidateGateMissing, "required gate %q has no evidence for commit %s", id, in.Commit)
		}
		if g.ExitCode != 0 || g.Status != "pass" {
			return CandidateManifest{}, refuse(CodeCandidateGateFailed, "required gate %q: status %q, exit %d", id, g.Status, g.ExitCode)
		}
		gates = append(gates, g)
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].ID < gates[j].ID })

	// Rule 8: risks are READ and recorded with their measured values; none blocks.
	var risks []RiskRecord
	for _, r := range spec.Risks {
		measured, err := readRisk(filepath.Join(in.RepoRoot, filepath.FromSlash(r.Source)), r.Kind)
		if err != nil {
			return CandidateManifest{}, refuse(CodeCandidateRiskUnreadable, "%s (%s): %v", r.ID, r.Source, err)
		}
		risks = append(risks, RiskRecord{ID: r.ID, Kind: r.Kind, Source: r.Source, Measured: measured, Blocking: false, Note: r.Note})
	}

	// Rule 9: waivers parse; a waived record without date and approver is refused.
	var waivers []WaiverRecord
	for _, w := range spec.Waivers {
		rec, err := readWaiver(filepath.Join(in.RepoRoot, filepath.FromSlash(w)))
		if err != nil {
			return CandidateManifest{}, err
		}
		rec.Path = w
		waivers = append(waivers, rec)
	}

	// Rule 10: deviations have an id, a known status and an existing record.
	for _, d := range spec.Deviations {
		if strings.TrimSpace(d.ID) == "" {
			return CandidateManifest{}, refuse(CodeCandidateDeviationInvalid, "a deviation has no id")
		}
		if d.Status != "open" && d.Status != "closed" && d.Status != "capa_open" {
			return CandidateManifest{}, refuse(CodeCandidateDeviationInvalid, "%s: status %q is not open|closed|capa_open", d.ID, d.Status)
		}
		if _, err := os.Stat(filepath.Join(in.RepoRoot, filepath.FromSlash(d.Record))); err != nil {
			return CandidateManifest{}, refuse(CodeCandidateDeviationInvalid, "%s: record %q not found", d.ID, d.Record)
		}
	}

	m := CandidateManifest{
		Format: CandidateFormat, Product: spec.Product, Version: spec.Version, TargetLevel: spec.TargetLevel,
		Commit: in.Commit, GeneratedAt: now.Format(time.RFC3339), GeneratedBy: in.GeneratedBy,
		ApprovalStatus: "pending", ReleaseExecuted: false,
		Artifacts: bundle.Manifest.Artifacts, Missing: []string{}, Complete: true,
		Gates: gates, GapsOpen: open, Risks: risks, Waivers: waivers, Deviations: spec.Deviations,
		SpecDigest: sha256Hex(specRaw), GatesDigest: sha256Hex(gatesRaw),
		Verdict: "candidate_verified", ClaimBoundary: CandidateClaimBoundary,
	}
	if m.Risks == nil {
		m.Risks = []RiskRecord{}
	}
	if m.Waivers == nil {
		m.Waivers = []WaiverRecord{}
	}
	if m.Deviations == nil {
		m.Deviations = []DeviationSpec{}
	}
	if m.Gates == nil {
		m.Gates = []GateResult{}
	}
	if m.GapsOpen == nil {
		m.GapsOpen = []GapRecord{}
	}
	m.ArtifactsDigest = artifactsDigest(m.Artifacts)
	return m, nil
}

// VerifyCandidateManifest re-checks the invariants and every artifact hash
// against the files at repoRoot (or, when repoRoot is empty, only the
// self-consistency of the manifest). Any change is a refusal.
func VerifyCandidateManifest(m CandidateManifest, repoRoot string) error {
	if m.Format != CandidateFormat {
		return refuse(CodeCandidateManifestInvalid, "format %q, want %q", m.Format, CandidateFormat)
	}
	if m.ApprovalStatus != "pending" {
		return refuse(CodeCandidateApprovalNotPending, "manifest approval_status is %q", m.ApprovalStatus)
	}
	if m.ReleaseExecuted {
		return refuse(CodeCandidateReleaseClaimed, "manifest claims release_executed")
	}
	if m.Verdict != "candidate_verified" {
		return refuse(CodeCandidateManifestInvalid, "verdict %q", m.Verdict)
	}
	if !m.Complete || len(m.Missing) != 0 {
		return refuse(CodeCandidateManifestInvalid, "manifest is marked incomplete (%d missing)", len(m.Missing))
	}
	if m.ArtifactsDigest != artifactsDigest(m.Artifacts) {
		return refuse(CodeCandidateTampered, "artifacts_digest does not match the artifact list")
	}
	for _, g := range m.Gates {
		if g.ExitCode != 0 || g.Status != "pass" {
			return refuse(CodeCandidateGateFailed, "manifest carries a failed gate %q", g.ID)
		}
	}
	for _, r := range m.Risks {
		if r.Blocking {
			return refuse(CodeCandidateManifestInvalid, "risk %s is marked blocking; risks are recorded, never gated on", r.ID)
		}
	}
	if repoRoot == "" {
		return nil
	}
	for _, a := range m.Artifacts {
		if a.Status != StatusPresent {
			continue
		}
		h, err := hashFileContent(filepath.Join(repoRoot, filepath.FromSlash(a.Path)))
		if err != nil {
			return refuse(CodeCandidateTampered, "%s: %v", a.Path, err)
		}
		if h != a.Hash {
			return refuse(CodeCandidateTampered, "%s: hash %s, manifest says %s", a.Path, h, a.Hash)
		}
	}
	return nil
}

// WriteCandidateZip writes manifest + gates evidence + present artifacts.
func WriteCandidateZip(m CandidateManifest, gatesRaw []byte, repoRoot, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	manifestRaw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{CandidateManifestName: manifestRaw, CandidateGatesName: gatesRaw} {
		zw, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := zw.Write(data); err != nil {
			return err
		}
	}
	for _, a := range m.Artifacts {
		if a.Status != StatusPresent {
			continue
		}
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(a.Path)))
		if err != nil {
			return fmt.Errorf("%s: %w", a.Path, err)
		}
		zw, err := w.Create("evidence/" + a.Path)
		if err != nil {
			return err
		}
		if _, err := zw.Write(data); err != nil {
			return err
		}
	}
	return w.Close()
}

// VerifyCandidateZip reads the manifest from the archive and checks every
// evidence entry's bytes against the manifest hashes, plus the manifest
// invariants. A byte changed inside the archive is a refusal.
func VerifyCandidateZip(path string) (CandidateManifest, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return CandidateManifest{}, refuse(CodeCandidateManifestInvalid, "open bundle: %v", err)
	}
	defer r.Close()
	files := map[string]*zip.File{}
	for _, f := range r.File {
		files[f.Name] = f
	}
	mf, ok := files[CandidateManifestName]
	if !ok {
		return CandidateManifest{}, refuse(CodeCandidateManifestInvalid, "bundle has no %s", CandidateManifestName)
	}
	raw, err := readZipFile(mf)
	if err != nil {
		return CandidateManifest{}, refuse(CodeCandidateManifestInvalid, "read manifest: %v", err)
	}
	var m CandidateManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return CandidateManifest{}, refuse(CodeCandidateManifestInvalid, "parse manifest: %v", err)
	}
	if err := VerifyCandidateManifest(m, ""); err != nil {
		return m, err
	}
	gf, ok := files[CandidateGatesName]
	if !ok {
		return m, refuse(CodeCandidateGateMissing, "bundle has no %s", CandidateGatesName)
	}
	graw, err := readZipFile(gf)
	if err != nil {
		return m, refuse(CodeCandidateGateMissing, "read gates: %v", err)
	}
	if sha256Hex(graw) != m.GatesDigest {
		return m, refuse(CodeCandidateTampered, "%s does not match gates_digest", CandidateGatesName)
	}
	for _, a := range m.Artifacts {
		if a.Status != StatusPresent {
			continue
		}
		f, ok := files["evidence/"+a.Path]
		if !ok {
			return m, refuse(CodeCandidateTampered, "evidence/%s missing from the bundle", a.Path)
		}
		data, err := readZipFile(f)
		if err != nil {
			return m, refuse(CodeCandidateTampered, "evidence/%s: %v", a.Path, err)
		}
		if h := sha256Hex(data); h != a.Hash {
			return m, refuse(CodeCandidateTampered, "evidence/%s: hash %s, manifest says %s", a.Path, h, a.Hash)
		}
	}
	return m, nil
}

// ---- helpers ---------------------------------------------------------------

type gapSet struct {
	all  []GapRecord
	byID map[string]GapRecord
}

func loadOpenBlockingGaps(path string) (gapSet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return gapSet{}, refuse(CodeCandidateArtifactUnreadable, "evidence ledger: %v", err)
	}
	var doc struct {
		BlockingGaps []struct {
			ID           string   `yaml:"id"`
			Severity     string   `yaml:"severity"`
			Status       string   `yaml:"status"`
			BlocksClaims []string `yaml:"blocks_claims"`
		} `yaml:"blocking_gaps"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return gapSet{}, refuse(CodeCandidateArtifactUnreadable, "evidence ledger: %v", err)
	}
	set := gapSet{byID: map[string]GapRecord{}}
	for _, g := range doc.BlockingGaps {
		rec := GapRecord{ID: g.ID, Severity: g.Severity, Status: g.Status, BlocksClaims: g.BlocksClaims}
		set.all = append(set.all, rec)
		set.byID[g.ID] = rec
	}
	return set, nil
}

// readRisk reads a measured risk input. Only kinds the tool knows how to read
// are accepted, so a risk can never be "declared" free-form.
func readRisk(path, kind string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "repeated_ci_evidence":
		var idx struct {
			SchemaVersion string         `json:"schema_version"`
			PublishedOn   string         `json:"published_on"`
			Measurement   map[string]any `json:"measurement"`
		}
		if err := json.Unmarshal(raw, &idx); err != nil {
			return nil, err
		}
		if idx.Measurement == nil {
			return nil, errors.New("index has no measurement")
		}
		out := map[string]any{"schema_version": idx.SchemaVersion, "published_on": idx.PublishedOn}
		for _, k := range []string{"consecutive_green_runs", "target_consecutive_green_runs", "runs_remaining_to_target", "claim_unlocked", "streak_break_reason", "newest_run_age_days"} {
			if v, ok := idx.Measurement[k]; ok {
				out[k] = v
			}
		}
		if _, ok := out["consecutive_green_runs"]; !ok {
			return nil, errors.New("measurement lacks consecutive_green_runs")
		}
		if _, ok := out["claim_unlocked"]; !ok {
			return nil, errors.New("measurement lacks claim_unlocked")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown risk kind %q", kind)
	}
}

func readWaiver(path string) (WaiverRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return WaiverRecord{}, refuse(CodeCandidateWaiverIncomplete, "waiver: %v", err)
	}
	var doc struct {
		DocumentID string `yaml:"document_id"`
		Status     string `yaml:"status"`
		Waived     []struct {
			RecordID   string `yaml:"record_id"`
			WaivedOn   string `yaml:"waived_on"`
			ApprovedBy string `yaml:"approved_by"`
		} `yaml:"waived_records"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return WaiverRecord{}, refuse(CodeCandidateWaiverIncomplete, "waiver %s: %v", path, err)
	}
	if strings.TrimSpace(doc.Status) == "" {
		return WaiverRecord{}, refuse(CodeCandidateWaiverIncomplete, "waiver %s has no status", filepath.Base(path))
	}
	for _, w := range doc.Waived {
		if strings.TrimSpace(w.RecordID) == "" || strings.TrimSpace(w.WaivedOn) == "" || strings.TrimSpace(w.ApprovedBy) == "" {
			return WaiverRecord{}, refuse(CodeCandidateWaiverIncomplete,
				"waiver %s: a waived record lacks record_id, waived_on or approved_by — an incomplete waiver is not a waiver", filepath.Base(path))
		}
	}
	return WaiverRecord{DocumentID: doc.DocumentID, Status: doc.Status, WaivedRecords: len(doc.Waived), Hash: sha256Hex(raw)}, nil
}

// checkArtifactContent refuses empty or unparsable artifacts of a known type.
func checkArtifactContent(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return errors.New("file is empty")
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		var v any
		if err := yaml.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("not valid YAML: %v", err)
		}
		if _, ok := v.(map[string]any); !ok {
			return errors.New("YAML document is not a mapping")
		}
	case ".json":
		if !json.Valid(raw) {
			return errors.New("not valid JSON")
		}
	case ".md":
		if !strings.Contains(string(raw), "\n# ") && !strings.HasPrefix(string(raw), "# ") {
			return errors.New("markdown has no top-level heading")
		}
	}
	return nil
}

func artifactsDigest(arts []ArtifactResult) string {
	sorted := make([]ArtifactResult, len(arts))
	copy(sorted, arts)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	var b strings.Builder
	for _, a := range sorted {
		fmt.Fprintf(&b, "%s\x00%s\x00%s\x00%s\n", a.ID, a.Path, a.Status, a.Hash)
	}
	return sha256Hex([]byte(b.String()))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// MarshalCandidateManifest writes the manifest as indented JSON.
func MarshalCandidateManifest(w io.Writer, m CandidateManifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}
