package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VRC-12 (#558) — the `pointintime resolve` CLI: resolves the in-force
// expression, exits 1 (not_in_force) for a date in a coverage gap.

const pitAtoms = `atoms:
  - atom_id: v1
    source_span: {hash: "sha256:aa"}
    metadata:
      temporal:
        work_id: LAT
        expression_id: exp-2014
        effective_from: "2014-05-01"
        effective_to: "2018-12-31"
  - atom_id: v2
    source_span: {hash: "sha256:bb"}
    metadata:
      temporal:
        work_id: LAT
        expression_id: exp-2020
        effective_from: "2020-01-01"
`

func writePitAtoms(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "atoms.yaml")
	if err := os.WriteFile(path, []byte(pitAtoms), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPIT_CLI_ResolvesInForce(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"pointintime", "resolve", "--atoms", writePitAtoms(t), "--work-id", "LAT", "--as-of", "2024-06-01"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("an in-force date must resolve (exit 0), got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"selected_atom_id": "v2"`) {
		t.Fatalf("expected v2 selected: %s", stdout.String())
	}
}

func TestPIT_CLI_RefusesCoverageGap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"pointintime", "resolve", "--atoms", writePitAtoms(t), "--work-id", "LAT", "--as-of", "2019-06-01"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("a date in a coverage gap must exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"status": "not_in_force"`) {
		t.Fatalf("expected not_in_force: %s", stdout.String())
	}
}

func TestPIT_CLI_RequiresFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"pointintime", "resolve", "--atoms", writePitAtoms(t)}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing required flags must exit 2, got %d", code)
	}
}
