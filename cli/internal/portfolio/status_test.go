package portfolio

// NRT-019 (#667) — the view is computed from files: edit a source and its hash
// and derived numbers move; remove one and the section is unavailable, never
// omitted; an old source is stale, never hidden; a candidate that is not
// pending is refused; a matrix that disagrees with the registry says so.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)

func copyMinirepo(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := "testdata/minirepo"
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func compute(t *testing.T, root string, mut func(o *Options)) Status {
	t.Helper()
	o := Options{RepoRoot: root, Now: fixedNow, StaleAfterDays: 90}
	if mut != nil {
		mut(&o)
	}
	st, err := Compute(o)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestMinirepoStatusIsComputedFromFiles(t *testing.T) {
	root := copyMinirepo(t)
	st := compute(t, root, func(o *Options) { o.ReleaseCandidatePath = "candidate-manifest.json" })
	caps := st.Capabilities.(Capabilities)
	if caps.Total != 2 || caps.Computed.Real != 1 || caps.Computed.Sidecar != 1 || !caps.ExpectedVsComputedAgree {
		t.Fatalf("capabilities: %+v", caps)
	}
	rm := st.Roadmap.(Roadmap)
	if rm.Lanes.Product.AutonomousOpen != 1 || rm.Lanes.Product.AutonomousClosed != 1 || rm.Lanes.Regulated.Human != 1 || rm.Lanes.Regulated.External != 1 || len(rm.RegulatedItems) != 2 || len(rm.Lanes.Product.Queue) != 1 {
		t.Fatalf("roadmap: %+v", rm)
	}
	g := st.Gaps.(Gaps)
	if g.Total != 2 || g.Open != 1 || g.Source.Freshness != "stale" {
		t.Fatalf("gaps must count and flag the 2026-05-02 ledger as stale: %+v", g)
	}
	capa := st.Capa.(CapaSection)
	if capa.Total != 3 || capa.Open != 1 || !capa.Records[0].EffectivenessVerified || capa.Records[0].Closed != "2026-06-11" || capa.Records[2].EffectivenessVerified {
		t.Fatalf("capa: %+v", capa)
	}
	rv := st.Reviews.(Reviews)
	if rv.Total != 2 || rv.Records[0].RecordID != "AUD-1" || rv.Records[0].Findings != 3 || rv.Records[1].Decisions != 2 || rv.Records[1].Actions != 3 {
		t.Fatalf("reviews: %+v", rv)
	}
	ci := st.RepeatedCI.(RepeatedCI)
	if ci.ConsecutiveGreenRuns != 4 || ci.TargetConsecutiveGreenRuns != 8 || ci.ClaimUnlocked || !strings.HasSuffix(ci.Source.Path, "index-2026-09-04.json") {
		t.Fatalf("repeated CI must pick the LATEST index: %+v", ci)
	}
	pg := st.PraxisGate.(PraxisGate)
	if pg.Status != "blocked" || pg.UnmetCount == 0 {
		t.Fatalf("praxis gate: %+v", pg)
	}
	comp := st.Competence.(Competence)
	if comp.AttestationFiles != 0 || comp.WaivedRecords != 0 {
		t.Fatalf("competence: %+v", comp)
	}
	dp := st.DomainPacks.(DomainPacks)
	if dp.Total != 2 || !dp.Packs[0].HasClaimBoundary || dp.Packs[1].HasClaimBoundary {
		t.Fatalf("domain packs: %+v", dp)
	}
	ps := st.PublicSources.(PublicSources)
	if ps.Total != 3 || ps.ByStatus["blocked"] != 1 || ps.ByStatus["captured_hash_only"] != 2 || ps.Source.Freshness != "fresh" {
		t.Fatalf("public sources: %+v", ps)
	}
	rc := st.ReleaseCandidate.(ReleaseCandidate)
	if rc.Version != "v9.9.9-TEST" || rc.ApprovalStatus != "pending" || rc.OpenGaps != 1 {
		t.Fatalf("release candidate: %+v", rc)
	}
	if st.SectionsUnavailable != 0 || st.SectionsStale != 1 {
		t.Fatalf("counts: unavailable %d stale %d", st.SectionsUnavailable, st.SectionsStale)
	}
	if !strings.HasPrefix(st.StatusDigest, "sha256:") || !strings.Contains(st.ClaimBoundary, "lifts no claim") {
		t.Fatalf("digest/claim: %+v", st)
	}
}

func TestStatusIsDeterministicAndMovesWithItsSources(t *testing.T) {
	root := copyMinirepo(t)
	a := compute(t, root, nil)
	b := compute(t, root, nil)
	if a.StatusDigest != b.StatusDigest {
		t.Fatal("same files, same clock → same digest")
	}
	// Edit one byte of one source: its sha256 and the digest move; a derived number moves too.
	p := filepath.Join(root, "docs/regulated/evidence-index/evidence-ledger.yaml")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(raw), "status: closed", "status: open", 1)), 0o644)
	c := compute(t, root, nil)
	if c.StatusDigest == a.StatusDigest || c.Gaps.(Gaps).Source.Sha256 == a.Gaps.(Gaps).Source.Sha256 || c.Gaps.(Gaps).Open != 2 {
		t.Fatal("an edited source must move its hash, the derived count and the status digest")
	}
}

func TestMissingSourcesAreUnavailableNotOmitted(t *testing.T) {
	root := copyMinirepo(t)
	os.Remove(filepath.Join(root, "scripts/vrc_wiring_matrix_registry.json"))
	os.RemoveAll(filepath.Join(root, "docs/regulated/evidence-index/repeated-ci-evidence"))
	os.Remove(filepath.Join(root, "docs/regulated/reference-basis/public-source-snapshots.yaml"))
	st := compute(t, root, nil)
	for name, sec := range map[string]any{"capabilities": st.Capabilities, "repeated_ci": st.RepeatedCI, "public_sources": st.PublicSources, "release_candidate": st.ReleaseCandidate} {
		u, ok := sec.(Unavailable)
		if !ok || u.Available || strings.TrimSpace(u.Reason) == "" {
			t.Fatalf("%s must be an explicit unavailable section with a reason, got %+v", name, sec)
		}
	}
	if st.SectionsUnavailable != 4 {
		t.Fatalf("sections_unavailable = %d, want 4", st.SectionsUnavailable)
	}
	// JSON keeps every section key even when unavailable.
	raw, _ := json.Marshal(st)
	for _, key := range []string{`"capabilities":{"available":false`, `"repeated_ci":{"available":false`, `"release_candidate":{"available":false`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("JSON must carry %s", key)
		}
	}
}

func TestStalenessFollowsTheClockAndPolicy(t *testing.T) {
	root := copyMinirepo(t)
	fresh := compute(t, root, func(o *Options) { o.Now = time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC); o.StaleAfterDays = 3650 })
	if fresh.SectionsStale != 0 {
		t.Fatalf("with a 10-year policy nothing is stale, got %d", fresh.SectionsStale)
	}
	old := compute(t, root, func(o *Options) { o.Now = time.Date(2027, 9, 6, 0, 0, 0, 0, time.UTC) })
	if old.SectionsStale < 3 || old.RepeatedCI.(RepeatedCI).Source.Freshness != "stale" {
		t.Fatalf("a year later the dated sources are stale, got %d", old.SectionsStale)
	}
	if old.Roadmap.(Roadmap).Source.Freshness != "undated" {
		t.Fatal("an undated source is reported undated, never fresh by default")
	}
}

func TestRegistryMatrixDisagreementIsVisible(t *testing.T) {
	root := copyMinirepo(t)
	p := filepath.Join(root, ".vrc-wiring-matrix/wiring-matrix.json")
	raw, _ := os.ReadFile(p)
	var m map[string]any
	json.Unmarshal(raw, &m)
	row := m["capabilities"].([]any)[1].(map[string]any)
	row["computed"], row["mismatch"] = "absent", true
	out, _ := json.Marshal(m)
	os.WriteFile(p, out, 0o644)
	caps := compute(t, root, nil).Capabilities.(Capabilities)
	if caps.ExpectedVsComputedAgree || caps.Mismatches != 1 || caps.Computed.Absent != 1 {
		t.Fatalf("a mismatching row must be counted and break agreement: %+v", caps)
	}
	// Registry adds a capability the matrix does not know → size disagreement.
	root2 := copyMinirepo(t)
	rp := filepath.Join(root2, "scripts/vrc_wiring_matrix_registry.json")
	raw, _ = os.ReadFile(rp)
	var reg map[string]any
	json.Unmarshal(raw, &reg)
	reg["capabilities"] = append(reg["capabilities"].([]any), map[string]any{"id": "gamma", "expected": "real"})
	out, _ = json.Marshal(reg)
	os.WriteFile(rp, out, 0o644)
	if compute(t, root2, nil).Capabilities.(Capabilities).ExpectedVsComputedAgree {
		t.Fatal("a registry/matrix size difference must break agreement")
	}
	// A row whose mismatch flag contradicts expected != computed is also a disagreement.
	root3 := copyMinirepo(t)
	p3 := filepath.Join(root3, ".vrc-wiring-matrix/wiring-matrix.json")
	raw, _ = os.ReadFile(p3)
	json.Unmarshal(raw, &m)
	row = m["capabilities"].([]any)[0].(map[string]any)
	row["computed"] = "partial" // expected real, mismatch still false → lying summary
	out, _ = json.Marshal(m)
	os.WriteFile(p3, out, 0o644)
	if compute(t, root3, nil).Capabilities.(Capabilities).ExpectedVsComputedAgree {
		t.Fatal("a row with expected != computed but mismatch=false must break agreement")
	}
}

func TestSummaryAndRowsMustTellTheSameStory(t *testing.T) {
	// A summary that claims mismatches the rows do not show: disagreement.
	root := copyMinirepo(t)
	p := filepath.Join(root, ".vrc-wiring-matrix/wiring-matrix.json")
	raw, _ := os.ReadFile(p)
	var m map[string]any
	json.Unmarshal(raw, &m)
	m["summary"].(map[string]any)["mismatches"] = float64(5)
	out, _ := json.Marshal(m)
	os.WriteFile(p, out, 0o644)
	if compute(t, root, nil).Capabilities.(Capabilities).ExpectedVsComputedAgree {
		t.Fatal("summary.mismatches != counted mismatches must break agreement")
	}
	// A consistent, honestly flagged mismatch is still a disagreement between registry and reality.
	root2 := copyMinirepo(t)
	p2 := filepath.Join(root2, ".vrc-wiring-matrix/wiring-matrix.json")
	raw, _ = os.ReadFile(p2)
	json.Unmarshal(raw, &m)
	row := m["capabilities"].([]any)[0].(map[string]any)
	row["computed"], row["mismatch"] = "sidecar", true
	m["summary"].(map[string]any)["mismatches"] = float64(1)
	out, _ = json.Marshal(m)
	os.WriteFile(p2, out, 0o644)
	caps := compute(t, root2, nil).Capabilities.(Capabilities)
	if caps.ExpectedVsComputedAgree || caps.Mismatches != 1 {
		t.Fatalf("one honestly flagged mismatch: agree must be false, got %+v", caps)
	}
}

func TestReleaseCandidateMustBePending(t *testing.T) {
	root := copyMinirepo(t)
	st := compute(t, root, func(o *Options) { o.ReleaseCandidatePath = "candidate-approved.json" })
	u, ok := st.ReleaseCandidate.(Unavailable)
	if !ok || !strings.Contains(u.Reason, "always pending") {
		t.Fatalf("an approved candidate is refused as a view input: %+v", st.ReleaseCandidate)
	}
}

func TestBrokenPraxisRecordIsUnavailable(t *testing.T) {
	root := copyMinirepo(t)
	p := filepath.Join(root, "docs/regulated/qualification/praxis-activation-gate.yaml")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(raw), "current_status: blocked_until_nomos_verified", "current_status: activated", 1)), 0o644)
	u, ok := compute(t, root, nil).PraxisGate.(Unavailable)
	if !ok || !strings.Contains(u.Reason, "refused") {
		t.Fatalf("a forged gate record must surface as unavailable with the refusal: %+v", u)
	}
}

func TestRealRepositoryStatusHasNoSilentSection(t *testing.T) {
	st := compute(t, "../../..", nil)
	raw, _ := json.Marshal(st)
	var m map[string]any
	json.Unmarshal(raw, &m)
	for _, key := range []string{"capabilities", "roadmap", "gaps", "capa", "reviews", "repeated_ci", "praxis_gate", "competence", "domain_packs", "public_sources", "release_candidate"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("section %s missing from the real status", key)
		}
	}
	if !st.Capabilities.(Capabilities).ExpectedVsComputedAgree {
		t.Fatal("on the real repository the registry and the generated matrix must agree")
	}
}

func TestMarkdownRendersEveryComputedSection(t *testing.T) {
	root := copyMinirepo(t)
	var sb strings.Builder
	if err := WriteMarkdown(&sb, compute(t, root, nil)); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	for _, must := range []string{"| capabilities | yes |", "| gaps | yes | 2 blocking gaps, 1 open", "(stale)", "| release_candidate | no |", "lifts no claim"} {
		if !strings.Contains(out, must) {
			t.Fatalf("markdown must contain %q:\n%s", must, out)
		}
	}
}
