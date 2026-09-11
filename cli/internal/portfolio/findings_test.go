package portfolio

// NRT-020 (#668) — findings are read, consistency is computed, filters are
// exact, and every finding names its source. On the real repository the
// Praxis gate's unmet requirements and the ledger gaps must appear.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func findingIDs(rep FindingsReport, kind string) []string {
	var ids []string
	for _, f := range rep.Findings {
		if kind == "" || f.Kind == kind {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

func TestFindingsOnMinirepo(t *testing.T) {
	root := copyMinirepo(t)
	rep, err := CollectFindings(root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unavailable) != 0 {
		t.Fatalf("all sources present, got unavailable %+v", rep.Unavailable)
	}
	want := map[string][]string{
		"evidence_gap":             {"ledger:GAP-A"},
		"capa_open":                {"capa:CAPA-2026-002"},
		"audit_finding_open":       {"audit:F1"},
		"action_overdue":           {"review:MR-1:A1"},
		"public_source_blocked":    {"public-source:B"},
		"praxis_requirement_unmet": nil, // counted below
	}
	for kind, ids := range want {
		if ids == nil {
			continue
		}
		if got := findingIDs(rep, kind); strings.Join(got, ",") != strings.Join(ids, ",") {
			t.Errorf("%s: got %v want %v", kind, got, ids)
		}
	}
	if n := len(findingIDs(rep, "praxis_requirement_unmet")); n != 11 {
		t.Errorf("praxis gate on the minirepo: %d unmet, want 11", n)
	}
	cons := findingIDs(rep, "consistency")
	for _, must := range []string{"capa:CAPA-2026-001:evidence:docs/gone/evidence.md", "capa:CAPA-2026-003:effectiveness", "audit:F2:CAPA-2026-002", "audit:F3:CAPA-2026-009", "review:MR-1:cites:docs/missing/ghost.md"} {
		if !contains(cons, must) {
			t.Errorf("consistency findings must include %s, got %v", must, cons)
		}
	}
	// Each consistency rule carries its own diagnostic; a mutation dropping one must not hide behind a neighbour.
	titles := map[string]string{}
	for _, f := range rep.Findings {
		titles[f.ID] = f.Title
	}
	for id, frag := range map[string]string{
		"audit:F3:CAPA-2026-009":                            "has no record",
		"audit:F2:CAPA-2026-002":                            "still open",
		"capa:CAPA-2026-003:effectiveness":                  "not recorded as verified",
		"capa:CAPA-2026-001:evidence:docs/gone/evidence.md": "no longer exists",
	} {
		if !strings.Contains(titles[id], frag) {
			t.Errorf("%s must say %q, got %q", id, frag, titles[id])
		}
	}
	if rep.Consistency != len(cons) || rep.Total != len(rep.Findings) || rep.BySeverity["major"] == 0 || rep.ByLane["regulated"] == 0 {
		t.Fatalf("counts: %+v", rep)
	}
	for _, f := range rep.Findings {
		if f.Source.Path == "" || !strings.HasPrefix(f.Source.Sha256, "sha256:") || f.Title == "" || f.Lane == "" {
			t.Fatalf("every finding names its source, hash, title and lane: %+v", f)
		}
	}
	// A2 is due in 2027: not overdue at the fixed clock; A3 is closed: never overdue.
	if contains(findingIDs(rep, "action_overdue"), "review:MR-1:A2") || contains(findingIDs(rep, "action_overdue"), "review:MR-1:A3") {
		t.Fatal("a future or closed action is not overdue")
	}
	// Move the clock: A2 becomes overdue and the next review too.
	later, _ := CollectFindings(root, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC))
	if !contains(findingIDs(later, "action_overdue"), "review:MR-1:A2") || len(findingIDs(later, "review_overdue")) != 1 {
		t.Fatalf("clock-driven overdue: %v %v", findingIDs(later, "action_overdue"), findingIDs(later, "review_overdue"))
	}
}

func TestFindingsFiltersAreExact(t *testing.T) {
	root := copyMinirepo(t)
	rep, _ := CollectFindings(root, fixedNow)
	out := FilterFindings(rep, FindingsFilter{Kinds: []string{"capa_open"}})
	if out.Total != 1 || out.Findings[0].ID != "capa:CAPA-2026-002" || out.ByKind["capa_open"] != 1 || len(out.ByKind) != 1 {
		t.Fatalf("kind filter: %+v", out)
	}
	out = FilterFindings(rep, FindingsFilter{Lanes: []string{"devops"}})
	for _, f := range out.Findings {
		if f.Lane != "devops" {
			t.Fatalf("lane filter leaked %+v", f)
		}
	}
	out = FilterFindings(rep, FindingsFilter{Severities: []string{"critical"}})
	if out.Total != 0 || out.Consistency != 0 {
		t.Fatalf("no critical finding in the minirepo, got %d", out.Total)
	}
	out = FilterFindings(rep, FindingsFilter{Kinds: []string{"consistency"}, Statuses: []string{"open"}})
	if out.Total == 0 || out.Consistency != out.Total {
		t.Fatalf("combined filter: %+v", out)
	}
}

func TestFindingsSourcesUnavailableAreListed(t *testing.T) {
	root := copyMinirepo(t)
	os.Remove(filepath.Join(root, "docs/regulated/evidence-index/evidence-ledger.yaml"))
	os.Remove(filepath.Join(root, ".vrc-wiring-matrix/wiring-matrix.json"))
	rep, _ := CollectFindings(root, fixedNow)
	if len(rep.Unavailable) != 2 || len(findingIDs(rep, "evidence_gap")) != 0 {
		t.Fatalf("missing sources are listed, not silently empty: %+v", rep.Unavailable)
	}
}

func TestRecordsIndexDisagreementIsAFinding(t *testing.T) {
	root := copyMinirepo(t)
	os.WriteFile(filepath.Join(root, "docs/regulated/operations/records/index.json"), []byte(`{"total": 99, "by_type": {}}`), 0o644)
	rep, _ := CollectFindings(root, fixedNow)
	if !contains(findingIDs(rep, "consistency"), "index:records:total") {
		t.Fatalf("an index that disagrees with the records is a consistency finding: %v", findingIDs(rep, "consistency"))
	}
}

func TestReviewsIndexOnMinirepo(t *testing.T) {
	root := copyMinirepo(t)
	rep, err := IndexReviews(root, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 2 || rep.ByType["management_review"] != 1 || rep.OverdueActions != 1 {
		t.Fatalf("reviews: %+v", rep)
	}
	var mr ReviewRecord
	for _, r := range rep.Records {
		if r.RecordType == "management_review" {
			mr = r
		}
	}
	if mr.Decisions != 2 || len(mr.Actions) != 3 || mr.Actions[0].Status != "status_unrecorded" || mr.Actions[2].Status != "closed" || mr.NextReviewDue != "2026-12-11" || mr.NextReviewOverdue {
		t.Fatalf("management review: %+v", mr)
	}
	missing := 0
	for _, c := range mr.CitedArtifacts {
		if !c.Exists {
			missing++
		}
	}
	if missing != 1 {
		t.Fatalf("one ghost citation expected, got %d in %+v", missing, mr.CitedArtifacts)
	}
}

func TestFindingsOnTheRealRepository(t *testing.T) {
	rep, err := CollectFindings("../../..", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unavailable) != 0 {
		t.Fatalf("real repository: sources unavailable %+v", rep.Unavailable)
	}
	if len(findingIDs(rep, "praxis_requirement_unmet")) == 0 || len(findingIDs(rep, "evidence_gap")) == 0 || len(findingIDs(rep, "public_source_blocked")) != 3 {
		t.Fatalf("expected the known open items on the real tree: %v", rep.ByKind)
	}
	if len(findingIDs(rep, "wiring_mismatch")) != 0 {
		t.Fatalf("the committed matrix has no mismatch, got %v", findingIDs(rep, "wiring_mismatch"))
	}
	var sb strings.Builder
	WriteFindingsMarkdown(&sb, rep)
	if !strings.Contains(sb.String(), "| `ledger:GAP-") || !strings.Contains(sb.String(), "prioritises") {
		t.Fatal("markdown must list findings and carry the claim boundary")
	}
}
