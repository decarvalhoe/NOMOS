// Package portfolio computes the portfolio status (NRT-019 #667) from committed
// machine sources only. It reads files, never prose; every section names its
// source by path and sha256 and carries a freshness verdict; a source that
// cannot be read yields an explicit `unavailable` section with its reason —
// never a silent omission, never a default. Nothing here approves, validates
// or lifts a claim: it is a view.
package portfolio

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

	"github.com/RBOKproject/Nomos/cli/internal/compliance"
)

const (
	Schema        = "nomos-portfolio-status-v1"
	ClaimBoundary = "A computed view of committed machine sources: registry and generated matrix, roadmap lanes, " +
		"evidence-ledger gaps, CAPA and review records, repeated-CI index, Praxis gate verdict, competence files, " +
		"domain packs, public-source snapshots, optional release candidate. It creates no evidence, approves nothing " +
		"and lifts no claim; an unavailable or stale source is shown as such, never hidden."
	DefaultStaleAfterDays = 90
)

// Source names a machine source and its freshness.
type Source struct {
	Path      string `json:"path"`
	Sha256    string `json:"sha256"`
	AsOf      string `json:"as_of,omitempty"`
	Freshness string `json:"freshness"`
}

// Unavailable is an explicit missing section.
type Unavailable struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

type Capabilities struct {
	Available bool   `json:"available"`
	Registry  Source `json:"registry"`
	Matrix    Source `json:"matrix"`
	Total     int    `json:"total"`
	Computed  struct {
		Real    int `json:"real"`
		Partial int `json:"partial"`
		Sidecar int `json:"sidecar"`
		Stub    int `json:"stub"`
		Absent  int `json:"absent"`
	} `json:"computed"`
	Mismatches              int  `json:"mismatches"`
	GenericCheckFailures    int  `json:"generic_check_failures"`
	ExpectedVsComputedAgree bool `json:"expected_vs_computed_agree"`
}

type LaneCounts struct {
	AutonomousOpen   int   `json:"autonomous_open"`
	AutonomousClosed int   `json:"autonomous_closed"`
	Passive          int   `json:"passive"`
	Human            int   `json:"human"`
	External         int   `json:"external"`
	Queue            []int `json:"queue"`
}

type RegulatedItem struct {
	Issue         int    `json:"issue"`
	Dispatch      string `json:"dispatch"`
	DeliveryState string `json:"delivery_state"`
	ClaimState    string `json:"claim_state"`
}

type Roadmap struct {
	Available bool   `json:"available"`
	Source    Source `json:"source"`
	Lanes     struct {
		Product   LaneCounts `json:"product"`
		Devops    LaneCounts `json:"devops"`
		Regulated LaneCounts `json:"regulated"`
	} `json:"lanes"`
	RegulatedItems []RegulatedItem `json:"regulated_items"`
}

type Gap struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Status       string   `json:"status"`
	BlocksClaims []string `json:"blocks_claims"`
}

type Gaps struct {
	Available bool   `json:"available"`
	Source    Source `json:"source"`
	Total     int    `json:"total"`
	Open      int    `json:"open"`
	Items     []Gap  `json:"items"`
}

type Capa struct {
	RecordID              string `json:"record_id"`
	Status                string `json:"status"`
	Severity              string `json:"severity"`
	Opened                string `json:"opened"`
	Closed                string `json:"closed,omitempty"`
	EffectivenessVerified bool   `json:"effectiveness_verified"`
	RetroDocumented       bool   `json:"retro_documented"`
	Source                Source `json:"source"`
}

type CapaSection struct {
	Available bool   `json:"available"`
	Directory string `json:"directory"`
	Total     int    `json:"total"`
	Open      int    `json:"open"`
	Records   []Capa `json:"records"`
}

type Review struct {
	RecordID   string `json:"record_id"`
	RecordType string `json:"record_type"`
	Date       string `json:"date"`
	Decisions  int    `json:"decisions"`
	Actions    int    `json:"actions"`
	Findings   int    `json:"findings"`
	Source     Source `json:"source"`
}

type Reviews struct {
	Available bool     `json:"available"`
	Directory string   `json:"directory"`
	Total     int      `json:"total"`
	Records   []Review `json:"records"`
}

type RepeatedCI struct {
	Available                  bool   `json:"available"`
	Source                     Source `json:"source"`
	ConsecutiveGreenRuns       int    `json:"consecutive_green_runs"`
	TargetConsecutiveGreenRuns int    `json:"target_consecutive_green_runs"`
	ClaimUnlocked              bool   `json:"claim_unlocked"`
}

type PraxisGate struct {
	Available  bool   `json:"available"`
	Record     Source `json:"record"`
	Status     string `json:"status"`
	UnmetCount int    `json:"unmet_count"`
	Checks     int    `json:"checks"`
}

type Competence struct {
	Available            bool   `json:"available"`
	AttestationFiles     int    `json:"attestation_files"`
	Waiver               Source `json:"waiver"`
	WaivedRecords        int    `json:"waived_records"`
	RoleStatusComputedBy string `json:"role_status_computed_by"`
}

type DomainPack struct {
	PackID           string `json:"pack_id"`
	Source           Source `json:"source"`
	HasClaimBoundary bool   `json:"has_claim_boundary"`
}

type DomainPacks struct {
	Available bool         `json:"available"`
	Directory string       `json:"directory"`
	Total     int          `json:"total"`
	Packs     []DomainPack `json:"packs"`
}

type PublicSources struct {
	Available bool           `json:"available"`
	Source    Source         `json:"source"`
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
}

type ReleaseCandidate struct {
	Available      bool   `json:"available"`
	Source         Source `json:"source"`
	Version        string `json:"version"`
	ApprovalStatus string `json:"approval_status"`
	Verdict        string `json:"verdict"`
	OpenGaps       int    `json:"open_gaps"`
}

// Status is the whole view. Sections are `any` so each is either its struct or Unavailable.
type Status struct {
	SchemaVersion   string `json:"schema_version"`
	GeneratedAt     string `json:"generated_at"`
	RepoRoot        string `json:"repo_root"`
	FreshnessPolicy struct {
		StaleAfterDays int `json:"stale_after_days"`
	} `json:"freshness_policy"`
	Capabilities        any    `json:"capabilities"`
	Roadmap             any    `json:"roadmap"`
	Gaps                any    `json:"gaps"`
	Capa                any    `json:"capa"`
	Reviews             any    `json:"reviews"`
	RepeatedCI          any    `json:"repeated_ci"`
	PraxisGate          any    `json:"praxis_gate"`
	Competence          any    `json:"competence"`
	DomainPacks         any    `json:"domain_packs"`
	PublicSources       any    `json:"public_sources"`
	ReleaseCandidate    any    `json:"release_candidate"`
	SectionsUnavailable int    `json:"sections_unavailable"`
	SectionsStale       int    `json:"sections_stale"`
	StatusDigest        string `json:"status_digest"`
	ClaimBoundary       string `json:"claim_boundary"`
}

// Options drive Compute.
type Options struct {
	RepoRoot             string
	Now                  time.Time
	StaleAfterDays       int
	ReleaseCandidatePath string // optional: a candidate-manifest.json (run output, not committed)
	PraxisGateRecord     string // default docs/regulated/qualification/praxis-activation-gate.yaml
}

type ctx struct {
	root        string
	now         time.Time
	stale       time.Duration
	unavailable int
	staleCount  int
}

var dateRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`)

func (c *ctx) source(rel, asOf string) (Source, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(rel)))
	if err != nil {
		return Source{}, nil, err
	}
	sum := sha256.Sum256(raw)
	s := Source{Path: rel, Sha256: "sha256:" + hex.EncodeToString(sum[:]), Freshness: "undated"}
	asOf = strings.TrimSpace(asOf)
	if m := dateRe.FindString(asOf); m != "" {
		s.AsOf = normaliseDate(asOf)
		if t, err := time.Parse("2006-01-02", m); err == nil {
			if c.now.Sub(t) > c.stale {
				s.Freshness = "stale"
				c.staleCount++
			} else {
				s.Freshness = "fresh"
			}
		}
	}
	return s, raw, nil
}

func normaliseDate(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 20 && v[10] == 'T' {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

func (c *ctx) miss(reason string) Unavailable {
	c.unavailable++
	return Unavailable{Available: false, Reason: reason}
}

// Compute builds the status. It never returns an error for a missing source:
// that is a section's job to report.
func Compute(opts Options) (Status, error) {
	if strings.TrimSpace(opts.RepoRoot) == "" {
		return Status{}, fmt.Errorf("portfolio: repo root is required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	days := opts.StaleAfterDays
	if days <= 0 {
		days = DefaultStaleAfterDays
	}
	c := &ctx{root: opts.RepoRoot, now: now, stale: time.Duration(days) * 24 * time.Hour}
	st := Status{SchemaVersion: Schema, GeneratedAt: now.UTC().Format("2006-01-02T15:04:05Z"), RepoRoot: opts.RepoRoot, ClaimBoundary: ClaimBoundary}
	st.FreshnessPolicy.StaleAfterDays = days

	st.Capabilities = c.capabilities()
	st.Roadmap = c.roadmap()
	st.Gaps = c.gaps()
	st.Capa = c.capa()
	st.Reviews = c.reviews()
	st.RepeatedCI = c.repeatedCI()
	record := opts.PraxisGateRecord
	if record == "" {
		record = "docs/regulated/qualification/praxis-activation-gate.yaml"
	}
	st.PraxisGate = c.praxisGate(record, now)
	st.Competence = c.competence()
	st.DomainPacks = c.domainPacks()
	st.PublicSources = c.publicSources()
	st.ReleaseCandidate = c.releaseCandidate(opts.ReleaseCandidatePath)
	st.SectionsUnavailable, st.SectionsStale = c.unavailable, c.staleCount

	// The digest covers everything but itself and the generation time.
	clone := st
	clone.GeneratedAt, clone.StatusDigest = "", ""
	raw, err := json.Marshal(clone)
	if err != nil {
		return Status{}, err
	}
	sum := sha256.Sum256(raw)
	st.StatusDigest = "sha256:" + hex.EncodeToString(sum[:])
	return st, nil
}

// ---- sections ----------------------------------------------------------------

func (c *ctx) capabilities() any {
	reg, regRaw, err := c.source("scripts/vrc_wiring_matrix_registry.json", "")
	if err != nil {
		return c.miss("registry unreadable: " + err.Error())
	}
	var registry struct {
		Capabilities []struct {
			ID       string `json:"id"`
			Expected string `json:"expected"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(regRaw, &registry); err != nil {
		return c.miss("registry is not the wiring registry JSON: " + err.Error())
	}
	var matrixDoc struct {
		Summary struct {
			Capabilities         int            `json:"capabilities"`
			Computed             map[string]int `json:"computed"`
			Mismatches           int            `json:"mismatches"`
			GenericCheckFailures int            `json:"generic_check_failures"`
		} `json:"summary"`
		Capabilities []struct {
			ID       string `json:"id"`
			Expected string `json:"expected"`
			Computed string `json:"computed"`
			Mismatch bool   `json:"mismatch"`
		} `json:"capabilities"`
	}
	mat, matRaw, err := c.source(".vrc-wiring-matrix/wiring-matrix.json", "")
	if err != nil {
		return c.miss("generated matrix unreadable: " + err.Error())
	}
	if err := json.Unmarshal(matRaw, &matrixDoc); err != nil {
		return c.miss("generated matrix is not JSON: " + err.Error())
	}
	cap := Capabilities{Available: true, Registry: reg, Matrix: mat}
	// Counts are computed from the per-capability rows, then cross-checked with the summary.
	cap.Total = len(matrixDoc.Capabilities)
	agree := len(registry.Capabilities) == len(matrixDoc.Capabilities) && matrixDoc.Summary.Capabilities == cap.Total
	expectedByID := map[string]string{}
	for _, r := range registry.Capabilities {
		expectedByID[r.ID] = r.Expected
	}
	for _, row := range matrixDoc.Capabilities {
		switch row.Computed {
		case "real":
			cap.Computed.Real++
		case "partial":
			cap.Computed.Partial++
		case "sidecar":
			cap.Computed.Sidecar++
		case "stub":
			cap.Computed.Stub++
		case "absent":
			cap.Computed.Absent++
		}
		if row.Mismatch {
			cap.Mismatches++
		}
		if e, ok := expectedByID[row.ID]; !ok || e != row.Expected || (row.Expected != row.Computed) != row.Mismatch {
			agree = false
		}
	}
	if cap.Mismatches != matrixDoc.Summary.Mismatches {
		agree = false
	}
	cap.GenericCheckFailures = matrixDoc.Summary.GenericCheckFailures
	cap.ExpectedVsComputedAgree = agree && cap.Mismatches == 0
	return cap
}

func (c *ctx) roadmap() any {
	src, raw, err := c.source("docs/roadmap-lanes.yaml", "")
	if err != nil {
		return c.miss("roadmap registry unreadable: " + err.Error())
	}
	var doc struct {
		Selection struct {
			Queues map[string][]int `yaml:"dispatch_queues"`
		} `yaml:"selection_policy"`
		Items []struct {
			Issue         int    `yaml:"issue"`
			State         string `yaml:"state"`
			Lane          string `yaml:"lane"`
			Dispatch      string `yaml:"dispatch"`
			DeliveryState string `yaml:"delivery_state"`
			ClaimState    string `yaml:"claim_state"`
		} `yaml:"items"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return c.miss("roadmap registry is not YAML: " + err.Error())
	}
	r := Roadmap{Available: true, Source: src}
	lanes := map[string]*LaneCounts{"product": &r.Lanes.Product, "devops": &r.Lanes.Devops, "regulated": &r.Lanes.Regulated}
	for name, lc := range lanes {
		lc.Queue = append([]int{}, doc.Selection.Queues[name]...)
		if lc.Queue == nil {
			lc.Queue = []int{}
		}
	}
	for _, it := range doc.Items {
		lc, ok := lanes[it.Lane]
		if !ok {
			continue
		}
		switch it.Dispatch {
		case "autonomous":
			if it.State == "open" {
				lc.AutonomousOpen++
			} else {
				lc.AutonomousClosed++
			}
		case "passive":
			lc.Passive++
		case "human":
			lc.Human++
		case "external":
			lc.External++
		}
		if it.Dispatch != "autonomous" {
			r.RegulatedItems = append(r.RegulatedItems, RegulatedItem{Issue: it.Issue, Dispatch: it.Dispatch, DeliveryState: it.DeliveryState, ClaimState: it.ClaimState})
		}
	}
	sort.Slice(r.RegulatedItems, func(i, j int) bool { return r.RegulatedItems[i].Issue < r.RegulatedItems[j].Issue })
	if r.RegulatedItems == nil {
		r.RegulatedItems = []RegulatedItem{}
	}
	return r
}

func (c *ctx) gaps() any {
	var doc struct {
		GeneratedAt  string `yaml:"generated_at"`
		BlockingGaps []struct {
			ID           string   `yaml:"id"`
			Severity     string   `yaml:"severity"`
			Status       string   `yaml:"status"`
			BlocksClaims []string `yaml:"blocks_claims"`
		} `yaml:"blocking_gaps"`
	}
	_, raw, err := c.source("docs/regulated/evidence-index/evidence-ledger.yaml", "")
	if err != nil {
		return c.miss("evidence ledger unreadable: " + err.Error())
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return c.miss("evidence ledger is not YAML: " + err.Error())
	}
	src, _, _ := c.source("docs/regulated/evidence-index/evidence-ledger.yaml", doc.GeneratedAt)
	g := Gaps{Available: true, Source: src, Items: []Gap{}}
	for _, bg := range doc.BlockingGaps {
		g.Total++
		if bg.Status == "open" {
			g.Open++
		}
		claims := bg.BlocksClaims
		if claims == nil {
			claims = []string{}
		}
		g.Items = append(g.Items, Gap{ID: bg.ID, Severity: bg.Severity, Status: bg.Status, BlocksClaims: claims})
	}
	return g
}

func (c *ctx) capa() any {
	dir := "docs/regulated/operations/records/capa"
	entries, err := os.ReadDir(filepath.Join(c.root, dir))
	if err != nil {
		return c.miss("CAPA directory unreadable: " + err.Error())
	}
	sec := CapaSection{Available: true, Directory: dir, Records: []Capa{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		rel := dir + "/" + e.Name()
		var doc struct {
			RecordID        string `yaml:"record_id"`
			RecordType      string `yaml:"record_type"`
			Status          string `yaml:"status"`
			Severity        string `yaml:"severity"`
			Opened          string `yaml:"opened"`
			Closed          string `yaml:"closed"`
			RetroDocumented bool   `yaml:"retro_documented"`
			Effectiveness   struct {
				Verified bool `yaml:"verified"`
			} `yaml:"effectiveness_verification"`
		}
		_, raw, err := c.source(rel, "")
		if err != nil || yaml.Unmarshal(raw, &doc) != nil || doc.RecordType != "deviation_capa" {
			continue
		}
		src, _, _ := c.source(rel, firstNonEmpty(doc.Closed, doc.Opened))
		sec.Total++
		if doc.Status != "closed" {
			sec.Open++
		}
		sec.Records = append(sec.Records, Capa{RecordID: doc.RecordID, Status: doc.Status, Severity: doc.Severity, Opened: normaliseDate(doc.Opened),
			Closed: normaliseDateOpt(doc.Closed), EffectivenessVerified: doc.Effectiveness.Verified, RetroDocumented: doc.RetroDocumented, Source: src})
	}
	sort.Slice(sec.Records, func(i, j int) bool { return sec.Records[i].RecordID < sec.Records[j].RecordID })
	return sec
}

func (c *ctx) reviews() any {
	dir := "docs/regulated/operations/records"
	entries, err := os.ReadDir(filepath.Join(c.root, dir))
	if err != nil {
		return c.miss("records directory unreadable: " + err.Error())
	}
	sec := Reviews{Available: true, Directory: dir, Records: []Review{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		rel := dir + "/" + e.Name()
		var doc struct {
			RecordID   string `yaml:"record_id"`
			RecordType string `yaml:"record_type"`
			Date       string `yaml:"date"`
			Decisions  []any  `yaml:"decisions"`
			Actions    []any  `yaml:"actions"`
			Findings   []any  `yaml:"findings"`
		}
		_, raw, err := c.source(rel, "")
		if err != nil || yaml.Unmarshal(raw, &doc) != nil {
			continue
		}
		switch doc.RecordType {
		case "management_review", "internal_audit", "role_assignment":
		default:
			continue
		}
		src, _, _ := c.source(rel, doc.Date)
		sec.Total++
		sec.Records = append(sec.Records, Review{RecordID: doc.RecordID, RecordType: doc.RecordType, Date: normaliseDate(doc.Date),
			Decisions: len(doc.Decisions), Actions: len(doc.Actions), Findings: len(doc.Findings), Source: src})
	}
	sort.Slice(sec.Records, func(i, j int) bool { return sec.Records[i].RecordID < sec.Records[j].RecordID })
	return sec
}

func (c *ctx) repeatedCI() any {
	dir := "docs/regulated/evidence-index/repeated-ci-evidence"
	entries, err := os.ReadDir(filepath.Join(c.root, dir))
	if err != nil {
		return c.miss("repeated-CI directory unreadable: " + err.Error())
	}
	var latest string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "index-") && strings.HasSuffix(e.Name(), ".json") && e.Name() > latest {
			latest = e.Name()
		}
	}
	if latest == "" {
		return c.miss("no repeated-CI index published (index-<date>.json)")
	}
	rel := dir + "/" + latest
	var doc struct {
		PublishedOn string `json:"published_on"`
		Measurement struct {
			Consecutive   int  `json:"consecutive_green_runs"`
			Target        int  `json:"target_consecutive_green_runs"`
			ClaimUnlocked bool `json:"claim_unlocked"`
		} `json:"measurement"`
	}
	_, raw, err := c.source(rel, "")
	if err != nil || json.Unmarshal(raw, &doc) != nil {
		return c.miss("repeated-CI index unreadable: " + rel)
	}
	if doc.Measurement.Target <= 0 {
		return c.miss("repeated-CI index has no target: " + rel)
	}
	src, _, _ := c.source(rel, doc.PublishedOn)
	return RepeatedCI{Available: true, Source: src, ConsecutiveGreenRuns: doc.Measurement.Consecutive, TargetConsecutiveGreenRuns: doc.Measurement.Target, ClaimUnlocked: doc.Measurement.ClaimUnlocked}
}

func (c *ctx) praxisGate(record string, now time.Time) any {
	src, _, err := c.source(record, "")
	if err != nil {
		return c.miss("praxis activation record unreadable: " + err.Error())
	}
	v, err := compliance.EvaluatePraxisActivation(filepath.Join(c.root, filepath.FromSlash(record)), c.root, now)
	if err != nil {
		return c.miss("praxis gate refused the record: " + err.Error())
	}
	return PraxisGate{Available: true, Record: src, Status: v.Status, UnmetCount: v.UnmetCount, Checks: len(v.Checks)}
}

func (c *ctx) competence() any {
	dir := "docs/regulated/operations/training-records"
	waiverRel := dir + "/independence-waiver.yaml"
	var waiver struct {
		Waived []any `yaml:"waived_records"`
	}
	_, raw, err := c.source(waiverRel, "")
	if err != nil || yaml.Unmarshal(raw, &waiver) != nil {
		return c.miss("independence waiver unreadable: " + waiverRel)
	}
	wsrc, _, _ := c.source(waiverRel, "")
	entries, err := os.ReadDir(filepath.Join(c.root, dir, "attestations"))
	if err != nil {
		return c.miss("attestations directory unreadable: " + err.Error())
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
			n++
		}
	}
	return Competence{Available: true, AttestationFiles: n, Waiver: wsrc, WaivedRecords: len(waiver.Waived), RoleStatusComputedBy: "scripts/training_competence_gate.py"}
}

func (c *ctx) domainPacks() any {
	dir := "docs/regulated/domain-packs"
	matches, err := filepath.Glob(filepath.Join(c.root, filepath.FromSlash(dir), "*", "pack.yaml"))
	if err != nil {
		return c.miss("domain packs unreadable: " + err.Error())
	}
	sec := DomainPacks{Available: true, Directory: dir, Packs: []DomainPack{}}
	for _, m := range matches {
		rel, _ := filepath.Rel(c.root, m)
		rel = filepath.ToSlash(rel)
		var doc struct {
			PackID        string `yaml:"pack_id"`
			ClaimBoundary string `yaml:"claim_boundary"`
		}
		src, raw, err := c.source(rel, "")
		if err != nil || yaml.Unmarshal(raw, &doc) != nil || doc.PackID == "" {
			continue
		}
		sec.Total++
		sec.Packs = append(sec.Packs, DomainPack{PackID: doc.PackID, Source: src, HasClaimBoundary: strings.TrimSpace(doc.ClaimBoundary) != ""})
	}
	sort.Slice(sec.Packs, func(i, j int) bool { return sec.Packs[i].PackID < sec.Packs[j].PackID })
	return sec
}

func (c *ctx) publicSources() any {
	rel := "docs/regulated/reference-basis/public-source-snapshots.yaml"
	var doc struct {
		Sources []struct {
			Status     string `yaml:"status"`
			CapturedOn string `yaml:"captured_on"`
		} `yaml:"sources"`
	}
	_, raw, err := c.source(rel, "")
	if err != nil || yaml.Unmarshal(raw, &doc) != nil {
		return c.miss("public-source snapshots unreadable: " + rel)
	}
	latest := ""
	by := map[string]int{}
	for _, s := range doc.Sources {
		by[s.Status]++
		if d := normaliseDate(s.CapturedOn); d > latest {
			latest = d
		}
	}
	src, _, _ := c.source(rel, latest)
	return PublicSources{Available: true, Source: src, Total: len(doc.Sources), ByStatus: by}
}

func (c *ctx) releaseCandidate(path string) any {
	if strings.TrimSpace(path) == "" {
		return c.miss("release candidate manifest is a run output (rehearsal artifact), none supplied")
	}
	var doc struct {
		Version        string `json:"version"`
		ApprovalStatus string `json:"approval_status"`
		Verdict        string `json:"verdict"`
		GeneratedAt    string `json:"generated_at"`
		GapsOpen       []any  `json:"gaps_open"`
	}
	src, raw, err := c.source(path, "")
	if err != nil || json.Unmarshal(raw, &doc) != nil {
		return c.miss("release candidate manifest unreadable: " + path)
	}
	src, _, _ = c.source(path, doc.GeneratedAt)
	if doc.ApprovalStatus != "pending" {
		return c.miss("release candidate manifest has approval_status " + doc.ApprovalStatus + "; a candidate is always pending")
	}
	return ReleaseCandidate{Available: true, Source: src, Version: doc.Version, ApprovalStatus: doc.ApprovalStatus, Verdict: doc.Verdict, OpenGaps: len(doc.GapsOpen)}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normaliseDateOpt(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return normaliseDate(v)
}

func sha256Of(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
