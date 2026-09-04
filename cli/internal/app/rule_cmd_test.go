package app

// VRC-42 (#578) — the bounded demo, end to end: a ```formula atom executed
// through a real external process, with its source trace.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const formulaDoc = "# Art. 7 Calcul\n\nLa contribution de base est fixée comme suit.\n\n" +
	"```formula\nbase + supplement\n```\n\n" +
	"# Art. 8 Cas non calculable\n\nLe seuil suit une condition rédigée en prose.\n\n" +
	"```formula\nsi le taux dépasse cinq pour cent alors majorer\n```\n\n" +
	"# Art. 9 Prose\n\nCeci n'est pas un bloc formula et ne doit jamais être exécuté.\n"

func writeDoc(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func demoSubstrate(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 unavailable")
	}
	script := filepath.Join("..", "..", "..", "scripts", "nomos_rule_substrate_demo.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the reference substrate adapter is missing: %v", err)
	}
	return "python3 " + script
}

func runRule(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"rule", "exec"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRuleExec_BoundedDemoRunsAFormulaAtomThroughTheSubstrate(t *testing.T) {
	path := writeDoc(t, formulaDoc)
	code, stdout, stderr := runRule(t,
		"--substrate-cmd", demoSubstrate(t),
		"--param", "base=100", "--param", "supplement=5",
		path)
	if code != 0 {
		t.Fatalf("bounded demo failed: %s %s", stdout, stderr)
	}

	var record struct {
		Substrate      string `json:"substrate"`
		FormulaCount   int    `json:"formula_count"`
		ComputedCount  int    `json:"computed_count"`
		UnsupportedNum int    `json:"unsupported_count"`
		Results        []struct {
			AtomID     string          `json:"atom_id"`
			Expression string          `json:"expression"`
			Status     string          `json:"status"`
			Value      json.RawMessage `json:"value"`
			Reason     string          `json:"reason"`
			Trace      struct {
				CanonicalRef string `json:"canonical_ref"`
				File         string `json:"file"`
				StartLine    int    `json:"start_line"`
			} `json:"source_trace"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &record); err != nil {
		t.Fatalf("record is not JSON: %v\n%s", err, stdout)
	}

	// Only the two formula blocks run; the prose paragraph never does.
	if record.FormulaCount != 2 {
		t.Fatalf("expected the two ```formula blocks, got %d", record.FormulaCount)
	}
	if record.ComputedCount != 1 || record.UnsupportedNum != 1 {
		t.Fatalf("counts wrong: %+v", record)
	}
	if record.Substrate == "" {
		t.Fatal("the record does not name the substrate that computed it")
	}

	var computed, unsupported bool
	for _, r := range record.Results {
		if r.Trace.CanonicalRef == "" || r.Trace.File == "" || r.Trace.StartLine == 0 {
			t.Fatalf("a result without its source trace: %+v", r)
		}
		switch r.Status {
		case "computed":
			computed = true
			if string(r.Value) != "105" {
				t.Fatalf("the substrate computed %s, expected 105", r.Value)
			}
		case "unsupported":
			unsupported = true
			if r.Reason == "" {
				t.Fatal("unsupported without a reason")
			}
			if len(r.Value) != 0 {
				t.Fatal("unsupported carrying a value")
			}
		}
	}
	if !computed || !unsupported {
		t.Fatal("the demo should exercise both outcomes")
	}
}

func TestRuleExec_WithoutASubstrateNothingRuns(t *testing.T) {
	// The anti-goal at the CLI door: NOMOS refuses rather than computing.
	path := writeDoc(t, formulaDoc)
	code, stdout, stderr := runRule(t, path)
	if code == 0 {
		t.Fatalf("execution proceeded without a substrate: %s", stdout)
	}
	if !strings.Contains(stderr, "never substitutes for one") {
		t.Fatalf("the refusal should say why: %s", stderr)
	}
	if strings.Contains(stdout, "computed") {
		t.Fatalf("a value was produced with no substrate: %s", stdout)
	}
}

func TestRuleExec_DocumentWithoutFormulaBlockIsNotAResult(t *testing.T) {
	// A document that declares nothing computable must not read as "nothing to
	// compute, all good".
	path := writeDoc(t, "# Titre\n\nQue de la prose, aucun bloc formula.\n")
	code, _, stderr := runRule(t, "--substrate-cmd", demoSubstrate(t), path)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d (%s)", code, stderr)
	}
	if !strings.Contains(stderr, "which is not a result") {
		t.Fatalf("the refusal should say why: %s", stderr)
	}
}

func TestRuleExec_FailingSubstrateEmitsNoRecord(t *testing.T) {
	path := writeDoc(t, formulaDoc)
	code, stdout, stderr := runRule(t, "--substrate-cmd", "false", path)
	if code == 0 {
		t.Fatal("a failing substrate produced a successful run")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("a record leaked on failure: %s", stdout)
	}
	if !strings.Contains(stderr, "RULE_SUBSTRATE_FAILED") {
		t.Fatalf("the failure code should be named: %s", stderr)
	}
}
