package portfolio

// NRT-020 (#668) — every open finding across the committed sources, normalised
// under a stable id, and the periodic review records indexed. Findings are
// READ from their sources and traceable to a path and hash; consistency
// findings are COMPUTED where two sources tell different stories. Nothing here
// closes, waives or prioritises a finding.

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
	FindingsSchema        = "nomos-portfolio-findings-v1"
	ReviewsSchema         = "nomos-portfolio-reviews-v1"
	FindingsClaimBoundary = "Open findings read from committed sources (ledger gaps, CAPA, audit findings, review actions, " +
		"Praxis gate requirements, public-source captures, wiring-matrix rows) plus consistency findings computed " +
		"where two sources disagree. Each is traceable to a path and hash. Nothing here closes, waives, prioritises " +
		"or resolves a finding."
)

// Finding is one normalised open item.
type Finding struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Source       Source   `json:"source"`
	Severity     string   `json:"severity"`
	Status       string   `json:"status"`
	Opened       string   `json:"opened,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	Lane         string   `json:"lane"`
	Title        string   `json:"title"`
	BlocksClaims []string `json:"blocks_claims"`
	Consistency  bool     `json:"consistency"`
}

// FindingsReport is the output of `nomos portfolio findings`.
type FindingsReport struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	RepoRoot      string         `json:"repo_root"`
	Total         int            `json:"total"`
	BySeverity    map[string]int `json:"by_severity"`
	ByKind        map[string]int `json:"by_kind"`
	ByLane        map[string]int `json:"by_lane"`
	Consistency   int            `json:"consistency_findings"`
	Unavailable   []Unavailable  `json:"sources_unavailable"`
	Findings      []Finding      `json:"findings"`
	ClaimBoundary string         `json:"claim_boundary"`
}

// FindingsFilter selects findings; every criterion is exact.
type FindingsFilter struct {
	Severities []string
	Statuses   []string
	Kinds      []string
	Lanes      []string
}

// ReviewAction is one action of a review record with its due state.
type ReviewAction struct {
	ID       string `json:"id"`
	Owner    string `json:"owner"`
	Due      string `json:"due,omitempty"`
	Status   string `json:"status"`
	Tracking string `json:"tracking,omitempty"`
	Overdue  bool   `json:"overdue"`
}

// ReviewRecord is an indexed periodic record.
type ReviewRecord struct {
	RecordID          string          `json:"record_id"`
	RecordType        string          `json:"record_type"`
	Date              string          `json:"date"`
	Source            Source          `json:"source"`
	Decisions         int             `json:"decisions"`
	Actions           []ReviewAction  `json:"actions"`
	Findings          int             `json:"findings"`
	Assignments       int             `json:"assignments"`
	NextReviewDue     string          `json:"next_review_due,omitempty"`
	NextReviewOverdue bool            `json:"next_review_overdue"`
	CitedArtifacts    []CitedArtifact `json:"cited_artifacts"`
}

// CitedArtifact is a repository path a record cites and whether it exists.
type CitedArtifact struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// ReviewsReport is the output of `nomos portfolio reviews`.
type ReviewsReport struct {
	SchemaVersion  string         `json:"schema_version"`
	GeneratedAt    string         `json:"generated_at"`
	RepoRoot       string         `json:"repo_root"`
	Total          int            `json:"total"`
	ByType         map[string]int `json:"by_type"`
	OverdueActions int            `json:"overdue_actions"`
	Records        []ReviewRecord `json:"records"`
	ClaimBoundary  string         `json:"claim_boundary"`
}

var citedPathRe = regexp.MustCompile(`(?:^|[^\w/])((?:docs|scripts|specs|cli|tests|templates|\.vrc-wiring-matrix|\.github)/(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+\.(?:md|yaml|yml|json|go|py|cue|ttl|sh))`)
var capaIDRe = regexp.MustCompile(`\bCAPA-[0-9]{4}-[0-9]{3}\b`)

func citedPaths(texts ...any) []string {
	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			for _, m := range citedPathRe.FindAllStringSubmatch(x, -1) {
				seen[strings.TrimRight(m[1], ".,;:)")] = true
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, t := range texts {
		walk(t)
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func hashOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func dateOnly(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if v == nil || s == "" || s == "<nil>" {
		return ""
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func isPast(date string, now time.Time) bool {
	t, err := time.Parse("2006-01-02", date)
	return err == nil && now.After(t.Add(24*time.Hour))
}

// ---- reviews -----------------------------------------------------------------

// IndexReviews reads the periodic records (management review, internal audit, role assignment).
func IndexReviews(root string, now time.Time) (ReviewsReport, error) {
	rep := ReviewsReport{SchemaVersion: ReviewsSchema, GeneratedAt: now.UTC().Format("2006-01-02T15:04:05Z"), RepoRoot: root, ByType: map[string]int{}, Records: []ReviewRecord{}, ClaimBoundary: FindingsClaimBoundary}
	dir := filepath.Join(root, "docs", "regulated", "operations", "records")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rep, fmt.Errorf("records directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		rel := "docs/regulated/operations/records/" + e.Name()
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var doc map[string]any
		if yaml.Unmarshal(raw, &doc) != nil {
			continue
		}
		rtype, _ := doc["record_type"].(string)
		switch rtype {
		case "management_review", "internal_audit", "role_assignment":
		default:
			continue
		}
		r := ReviewRecord{RecordID: fmt.Sprint(doc["record_id"]), RecordType: rtype, Date: dateOnly(doc["date"]), Source: Source{Path: rel, Sha256: hashOf(raw), Freshness: "undated"}, Actions: []ReviewAction{}, CitedArtifacts: []CitedArtifact{}}
		if r.Date != "" {
			r.Source.AsOf, r.Source.Freshness = r.Date, "fresh"
		}
		r.Decisions = lenOf(doc["decisions"])
		r.Findings = lenOf(doc["findings"])
		r.Assignments = lenOf(doc["assignments"])
		if acts, ok := doc["actions"].([]any); ok {
			for _, a := range acts {
				am, _ := a.(map[string]any)
				act := ReviewAction{ID: fmt.Sprint(am["id"]), Owner: fmt.Sprint(am["owner"]), Due: dateOnly(am["due"]), Tracking: strings.TrimSpace(fmt.Sprint(am["tracking"]))}
				if act.Tracking == "<nil>" {
					act.Tracking = ""
				}
				act.Status = strings.TrimSpace(fmt.Sprint(am["status"]))
				if act.Status == "" || act.Status == "<nil>" {
					act.Status = "status_unrecorded"
				}
				act.Overdue = act.Status != "closed" && act.Status != "done" && act.Due != "" && isPast(act.Due, now)
				if act.Overdue {
					rep.OverdueActions++
				}
				r.Actions = append(r.Actions, act)
			}
		}
		r.NextReviewDue = dateOnly(doc["next_review_due"])
		r.NextReviewOverdue = r.NextReviewDue != "" && isPast(r.NextReviewDue, now)
		for _, p := range citedPaths(doc["inputs"], doc["decisions"], doc["actions"], doc["findings"], doc["references"], doc["evidence"]) {
			_, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
			r.CitedArtifacts = append(r.CitedArtifacts, CitedArtifact{Path: p, Exists: statErr == nil})
		}
		rep.ByType[rtype]++
		rep.Records = append(rep.Records, r)
	}
	sort.Slice(rep.Records, func(i, j int) bool { return rep.Records[i].RecordID < rep.Records[j].RecordID })
	rep.Total = len(rep.Records)
	return rep, nil
}

func lenOf(v any) int {
	if l, ok := v.([]any); ok {
		return len(l)
	}
	return 0
}

// ---- findings ----------------------------------------------------------------

// CollectFindings reads every source and computes consistency findings.
func CollectFindings(root string, now time.Time) (FindingsReport, error) {
	rep := FindingsReport{SchemaVersion: FindingsSchema, GeneratedAt: now.UTC().Format("2006-01-02T15:04:05Z"), RepoRoot: root, BySeverity: map[string]int{}, ByKind: map[string]int{}, ByLane: map[string]int{}, Unavailable: []Unavailable{}, Findings: []Finding{}, ClaimBoundary: FindingsClaimBoundary}
	if strings.TrimSpace(root) == "" {
		return rep, fmt.Errorf("repo root is required")
	}
	add := func(f Finding) {
		if f.BlocksClaims == nil {
			f.BlocksClaims = []string{}
		}
		rep.Findings = append(rep.Findings, f)
	}
	miss := func(reason string) {
		rep.Unavailable = append(rep.Unavailable, Unavailable{Available: false, Reason: reason})
	}

	// 1. Evidence-ledger gaps that are not closed/resolved.
	ledgerRel := "docs/regulated/evidence-index/evidence-ledger.yaml"
	ledgerGaps := map[string]string{}
	if raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ledgerRel))); err != nil {
		miss("evidence ledger unreadable: " + err.Error())
	} else {
		var doc struct {
			GeneratedAt  string `yaml:"generated_at"`
			BlockingGaps []struct {
				ID, Severity, Status, Description string
				BlocksClaims                      []string `yaml:"blocks_claims"`
			} `yaml:"blocking_gaps"`
		}
		if yaml.Unmarshal(raw, &doc) != nil {
			miss("evidence ledger is not YAML")
		} else {
			src := Source{Path: ledgerRel, Sha256: hashOf(raw), AsOf: dateOnly(doc.GeneratedAt), Freshness: "undated"}
			for _, g := range doc.BlockingGaps {
				ledgerGaps[g.ID] = g.Status
				if g.Status == "closed" || g.Status == "resolved" {
					continue
				}
				add(Finding{ID: "ledger:" + g.ID, Kind: "evidence_gap", Source: src, Severity: g.Severity, Status: g.Status, Lane: "regulated",
					Title: firstNonEmpty(firstSentence(g.Description), "evidence gap "+g.ID+" is "+g.Status), BlocksClaims: g.BlocksClaims})
			}
		}
	}

	// 2. CAPA records: open ones are findings; closed ones with effectiveness evidence citing missing artifacts are consistency findings.
	capaStatus := map[string]string{}
	capaDir := filepath.Join(root, "docs", "regulated", "operations", "records", "capa")
	if entries, err := os.ReadDir(capaDir); err != nil {
		miss("CAPA directory unreadable: " + err.Error())
	} else {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			rel := "docs/regulated/operations/records/capa/" + e.Name()
			raw, err := os.ReadFile(filepath.Join(capaDir, e.Name()))
			if err != nil {
				continue
			}
			var doc map[string]any
			if yaml.Unmarshal(raw, &doc) != nil || fmt.Sprint(doc["record_type"]) != "deviation_capa" {
				continue
			}
			id := fmt.Sprint(doc["record_id"])
			status := fmt.Sprint(doc["status"])
			capaStatus[id] = status
			src := Source{Path: rel, Sha256: hashOf(raw), AsOf: dateOnly(doc["opened"]), Freshness: "undated"}
			if status != "closed" {
				add(Finding{ID: "capa:" + id, Kind: "capa_open", Source: src, Severity: fmt.Sprint(doc["severity"]), Status: status, Opened: dateOnly(doc["opened"]), Owner: stringOr(doc["owner"], ""), Lane: "regulated",
					Title: firstNonEmpty(firstSentence(nestedString(doc, "deviation", "summary")), "deviation/CAPA record "+id+" is "+status)})
				continue
			}
			ev, _ := doc["effectiveness_verification"].(map[string]any)
			verified, _ := ev["verified"].(bool)
			if !verified {
				add(Finding{ID: "capa:" + id + ":effectiveness", Kind: "consistency", Source: src, Severity: "major", Status: "open", Lane: "regulated", Title: "CAPA is closed but its effectiveness verification is not recorded as verified", Consistency: true})
			}
			for _, p := range citedPaths(ev) {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
					add(Finding{ID: "capa:" + id + ":evidence:" + p, Kind: "consistency", Source: src, Severity: "major", Status: "open", Lane: "regulated", Title: "closed CAPA cites effectiveness evidence that no longer exists: " + p, Consistency: true})
				}
			}
		}
	}

	// 3. Review records: audit findings without disposition or whose disposition names a CAPA that is not closed; overdue actions; missing cited artifacts.
	reviews, err := IndexReviews(root, now)
	if err != nil {
		miss("review records unreadable: " + err.Error())
	} else {
		for _, r := range reviews.Records {
			for _, a := range r.Actions {
				if a.Overdue {
					add(Finding{ID: "review:" + r.RecordID + ":" + a.ID, Kind: "action_overdue", Source: r.Source, Severity: "major", Status: a.Status, Opened: r.Date, Owner: a.Owner, Lane: "regulated", Title: "review action past due (" + a.Due + ") without a recorded closure" + trackingSuffix(a.Tracking)})
				}
			}
			for _, c := range r.CitedArtifacts {
				if !c.Exists {
					add(Finding{ID: "review:" + r.RecordID + ":cites:" + c.Path, Kind: "consistency", Source: r.Source, Severity: "major", Status: "open", Lane: "regulated", Title: "review record cites an artifact that does not exist: " + c.Path, Consistency: true})
				}
			}
			if r.NextReviewOverdue {
				add(Finding{ID: "review:" + r.RecordID + ":next_review", Kind: "review_overdue", Source: r.Source, Severity: "major", Status: "open", Lane: "regulated", Title: "next periodic review was due " + r.NextReviewDue + " and no later record exists"})
			}
			if r.RecordType != "internal_audit" {
				continue
			}
			raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.Source.Path)))
			var doc struct {
				Findings []struct {
					ID, Severity, Finding, Disposition string
				} `yaml:"findings"`
			}
			if yaml.Unmarshal(raw, &doc) != nil {
				continue
			}
			for _, f := range doc.Findings {
				if strings.TrimSpace(f.Disposition) == "" {
					add(Finding{ID: "audit:" + f.ID, Kind: "audit_finding_open", Source: r.Source, Severity: f.Severity, Status: "open", Opened: r.Date, Lane: "regulated", Title: firstSentence(f.Finding)})
					continue
				}
				for _, capaID := range capaIDRe.FindAllString(f.Disposition, -1) {
					st, known := capaStatus[capaID]
					if !known {
						add(Finding{ID: "audit:" + f.ID + ":" + capaID, Kind: "consistency", Source: r.Source, Severity: "major", Status: "open", Lane: "regulated", Title: "audit finding disposition names " + capaID + " which has no record", Consistency: true})
					} else if st != "closed" {
						add(Finding{ID: "audit:" + f.ID + ":" + capaID, Kind: "consistency", Source: r.Source, Severity: f.Severity, Status: st, Lane: "regulated", Title: "audit finding disposition relies on " + capaID + " which is still " + st, Consistency: true})
					}
				}
			}
		}
		// The #669 index, when committed, must agree with what the records say.
		idxRel := "docs/regulated/operations/records/index.json"
		if raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(idxRel))); err == nil {
			var idx struct {
				Total  int            `json:"total"`
				ByType map[string]int `json:"by_type"`
			}
			if json.Unmarshal(raw, &idx) == nil {
				capaCount := len(capaStatus)
				if idx.Total != reviews.Total+capaCount {
					add(Finding{ID: "index:records:total", Kind: "consistency", Source: Source{Path: idxRel, Sha256: hashOf(raw), Freshness: "undated"}, Severity: "minor", Status: "open", Lane: "devops", Title: fmt.Sprintf("records index says %d records, the tree has %d", idx.Total, reviews.Total+capaCount), Consistency: true})
				}
			}
		}
	}

	// 4. Praxis activation gate: every unmet requirement is a finding.
	gateRel := "docs/regulated/qualification/praxis-activation-gate.yaml"
	if raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateRel))); err != nil {
		miss("praxis activation record unreadable: " + err.Error())
	} else if v, err := compliance.EvaluatePraxisActivation(filepath.Join(root, filepath.FromSlash(gateRel)), root, now); err != nil {
		miss("praxis gate refused the record: " + err.Error())
	} else {
		src := Source{Path: gateRel, Sha256: hashOf(raw), Freshness: "undated"}
		for _, c := range v.Checks {
			if !c.Met {
				add(Finding{ID: "praxis-gate:" + c.ID, Kind: "praxis_requirement_unmet", Source: src, Severity: "major", Status: "open", Lane: "regulated", Title: "required " + c.Required + ", actual " + c.Actual + " (" + c.Reason + ")"})
			}
		}
	}

	// 5. Public-source snapshots that are blocked.
	snapRel := "docs/regulated/reference-basis/public-source-snapshots.yaml"
	if raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(snapRel))); err != nil {
		miss("public-source snapshots unreadable: " + err.Error())
	} else {
		var doc struct {
			Sources []struct {
				ReferenceID string `yaml:"reference_id"`
				Status      string `yaml:"status"`
				CapturedOn  string `yaml:"captured_on"`
				Reason      string `yaml:"reason"`
			} `yaml:"sources"`
		}
		if yaml.Unmarshal(raw, &doc) != nil {
			miss("public-source snapshots is not YAML")
		} else {
			src := Source{Path: snapRel, Sha256: hashOf(raw), Freshness: "undated"}
			for _, s := range doc.Sources {
				if s.Status == "blocked" {
					add(Finding{ID: "public-source:" + s.ReferenceID, Kind: "public_source_blocked", Source: src, Severity: "minor", Status: "blocked", Opened: dateOnly(s.CapturedOn), Lane: "devops", Title: firstNonEmpty(s.Reason, "public source capture blocked")})
				}
			}
		}
	}

	// 6. Wiring-matrix rows in mismatch.
	matRel := ".vrc-wiring-matrix/wiring-matrix.json"
	if raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(matRel))); err != nil {
		miss("generated matrix unreadable: " + err.Error())
	} else {
		var doc struct {
			Capabilities []struct {
				ID, Expected, Computed string
				Mismatch               bool
			} `json:"capabilities"`
		}
		if json.Unmarshal(raw, &doc) != nil {
			miss("generated matrix is not JSON")
		} else {
			src := Source{Path: matRel, Sha256: hashOf(raw), Freshness: "undated"}
			for _, c := range doc.Capabilities {
				if c.Mismatch {
					add(Finding{ID: "matrix:" + c.ID, Kind: "wiring_mismatch", Source: src, Severity: "major", Status: "open", Lane: "product", Title: "registry expects " + c.Expected + ", tree computes " + c.Computed})
				}
			}
		}
	}

	sort.Slice(rep.Findings, func(i, j int) bool { return rep.Findings[i].ID < rep.Findings[j].ID })
	for _, f := range rep.Findings {
		rep.BySeverity[f.Severity]++
		rep.ByKind[f.Kind]++
		rep.ByLane[f.Lane]++
		if f.Consistency {
			rep.Consistency++
		}
	}
	rep.Total = len(rep.Findings)
	return rep, nil
}

// FilterFindings keeps findings matching every criterion; counts are recomputed.
func FilterFindings(rep FindingsReport, f FindingsFilter) FindingsReport {
	out := rep
	out.Findings = []Finding{}
	out.BySeverity, out.ByKind, out.ByLane, out.Consistency = map[string]int{}, map[string]int{}, map[string]int{}, 0
	for _, x := range rep.Findings {
		if (len(f.Severities) > 0 && !contains(f.Severities, x.Severity)) || (len(f.Statuses) > 0 && !contains(f.Statuses, x.Status)) ||
			(len(f.Kinds) > 0 && !contains(f.Kinds, x.Kind)) || (len(f.Lanes) > 0 && !contains(f.Lanes, x.Lane)) {
			continue
		}
		out.Findings = append(out.Findings, x)
		out.BySeverity[x.Severity]++
		out.ByKind[x.Kind]++
		out.ByLane[x.Lane]++
		if x.Consistency {
			out.Consistency++
		}
	}
	out.Total = len(out.Findings)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if i := strings.Index(s, ". "); i > 0 && i < 200 {
		return s[:i+1]
	}
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

func nestedString(doc map[string]any, keys ...string) string {
	var cur any = doc
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func trackingSuffix(t string) string {
	if t == "" {
		return ""
	}
	return " — tracked as " + t
}

func stringOr(v any, fallback string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}
