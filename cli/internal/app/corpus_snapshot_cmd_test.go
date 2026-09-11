package app

// #611 — the snapshot CLI and the strict gate section, end to end on the
// committed fixture.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func snapshotFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join("..", "corpus", "testdata", "external-snapshot")
	env := filepath.Join(dir, "snapshot.json")
	if _, err := os.Stat(env); err != nil {
		t.Fatalf("committed fixture missing: %v", err)
	}
	return env, filepath.Join(dir, "sources.jsonl")
}

func mutatedRecords(t *testing.T, records string) string {
	t.Helper()
	raw, err := os.ReadFile(records)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "mutated.jsonl")
	mut := strings.Replace(string(raw), "2222222222222222222222222222222222222222222222222222222222222222",
		"2222222222222222222222222222222222222222222222222222222222222223", 1)
	if err := os.WriteFile(out, []byte(mut), 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSnapshotCLI_VerifyPassesOnFixtureAndRefusesAMutatedByte(t *testing.T) {
	env, records := snapshotFixture(t)
	var out, errb bytes.Buffer
	if code := Run([]string{"corpus", "snapshot", "verify", "--envelope", env}, &out, &errb); code != 0 {
		t.Fatalf("fixture refused: %s %s", out.String(), errb.String())
	}
	if !strings.Contains(out.String(), `"status": "pass"`) {
		t.Fatalf("verdict not pass: %s", out.String())
	}
	out.Reset()
	errb.Reset()
	code := Run([]string{"corpus", "snapshot", "verify", "--envelope", env, "--records", mutatedRecords(t, records)}, &out, &errb)
	if code == 0 {
		t.Fatal("a mutated record verified")
	}
	// The verdict is still written, with the problem named: refused evidence is evidence.
	if !strings.Contains(out.String(), "SNAPSHOT_ROOT_MISMATCH") || !strings.Contains(errb.String(), "REFUSED") {
		t.Fatalf("refusal not named: %s %s", out.String(), errb.String())
	}
}

func TestSnapshotCLI_ImportWritesNothingWhenRefused(t *testing.T) {
	env, records := snapshotFixture(t)
	target := filepath.Join(t.TempDir(), "manifest.yaml")
	var out, errb bytes.Buffer
	code := Run([]string{"corpus", "snapshot", "import", "--envelope", env, "--records", mutatedRecords(t, records), "--out", target}, &out, &errb)
	if code == 0 {
		t.Fatal("import of a refused snapshot succeeded")
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("a manifest was written from a refused snapshot")
	}
	if !strings.Contains(errb.String(), "nothing written") {
		t.Fatalf("refusal should say nothing was written: %s", errb.String())
	}
	// And the healthy one imports every record.
	out.Reset()
	errb.Reset()
	if code := Run([]string{"corpus", "snapshot", "import", "--envelope", env, "--out", target}, &out, &errb); code != 0 {
		t.Fatalf("import failed: %s", errb.String())
	}
	manifest, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, must := range []string{"version_id=2026-06", "version_id=2026-09", "canonical_url: https://example.invalid/reglement/art-7", "snapshot=fixture-external-snapshot-001"} {
		if !strings.Contains(string(manifest), must) {
			t.Fatalf("import lost %q", must)
		}
	}
}

func TestStrictGate_ExternalSnapshotSectionBlocksAMutatedSnapshot(t *testing.T) {
	env, records := snapshotFixture(t)
	var out, errb bytes.Buffer
	if code := Run([]string{"strict", "--external-snapshot", env, "--format", "json"}, &out, &errb); code != 0 {
		t.Fatalf("healthy snapshot blocked the gate: %s %s", out.String(), errb.String())
	}
	if !strings.Contains(out.String(), `"name": "external-snapshot"`) {
		t.Fatalf("section missing from the gate result: %s", out.String())
	}
	out.Reset()
	code := Run([]string{"strict", "--external-snapshot", env, "--external-snapshot-records", mutatedRecords(t, records), "--format", "json"}, &out, &errb)
	if code == 0 {
		t.Fatal("the strict gate passed a mutated snapshot")
	}
	if !strings.Contains(out.String(), `"code": "SNAPSHOT_ROOT_MISMATCH"`) {
		t.Fatalf("the gate should name the stable code: %s", out.String())
	}
}

func TestSnapshotCLI_SealRefusesBadRecords(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	_ = os.WriteFile(bad, []byte(`{"source_id":"a","version_id":"v1","locator":"a.md","content_hash":"placeholder:not-fetched","captured_at":"2026-09-05T00:00:00Z"}`+"\n"), 0o644)
	var out, errb bytes.Buffer
	code := Run([]string{"corpus", "snapshot", "seal", "--records", bad, "--snapshot-id", "x", "--producer", "p", "--out", filepath.Join(t.TempDir(), "env.json")}, &out, &errb)
	if code == 0 || !strings.Contains(errb.String(), "SNAPSHOT_UNSTABLE_HASH") {
		t.Fatalf("a producer sealed a placeholder hash: %d %s", code, errb.String())
	}
}
