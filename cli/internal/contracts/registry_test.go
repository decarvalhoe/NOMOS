package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// TestRealRegistryVerifies is the tripwire: the committed registry agrees with the tree.
func TestRealRegistryVerifies(t *testing.T) {
	rep, err := Verify(repoRoot(t), now)
	if err != nil {
		t.Fatalf("real registry: %v", err)
	}
	if rep.ByStability["stable"] < 10 || rep.CompatReads < 4 {
		t.Fatalf("unexpected shape: %+v", rep.ByStability)
	}
	for _, r := range rep.Rows {
		if r.Stability == "stable" && (r.ValidFixtures == 0 || r.SchemaVersion == "") {
			t.Fatalf("%s stable without fixture/version passed Verify", r.ID)
		}
	}
}

func sha(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

const cueA = "package specs\n\n#A: {\n\tschema_version: string | *\"1.0.0\"\n\tname: string\n}\n"

// mini builds a synthetic repo with one stable contract and one experimental.
func mini(t *testing.T, edit func(reg *string)) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("specs/a.cue", cueA)
	mk("specs/b.cue", "package specs\n#B: {x: int}\n")
	mk("specs/examples/a.valid.json", `{"name":"ok"}`)
	reg := `schema_version: nomos-contract-registry-v1
claim_boundary: test
rules: {stable_requires_valid_fixture: true}
contracts:
  - id: a
    path: specs/a.cue
    stability: stable
    version_kind: default
    schema_version: "1.0.0"
    sha256: "` + sha([]byte(cueA)) + `"
    definition: "#A"
    fixtures: {valid: [specs/examples/a.valid.json], invalid: []}
    readers: [x.go]
    compat_fixtures: []
  - id: b
    path: specs/b.cue
    stability: experimental
    version_kind: none
    schema_version: ""
    sha256: "` + sha([]byte("package specs\n#B: {x: int}\n")) + `"
    definition: "#B"
    fixtures: {valid: [], invalid: []}
    readers: []
    compat_fixtures: []
`
	if edit != nil {
		edit(&reg)
	}
	mk(RegistryPath, reg)
	return root
}

func wantCode(t *testing.T, err error, code, msgPart string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want refusal %s, got %v", code, err)
	}
	if e.Code != code {
		t.Fatalf("want %s, got %s: %s", code, e.Code, e.Message)
	}
	if !strings.Contains(e.Message, msgPart) {
		t.Fatalf("message %q lacks %q", e.Message, msgPart)
	}
}

func TestMiniPasses(t *testing.T) {
	rep, err := Verify(mini(t, nil), now)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 2 || rep.ByStability["stable"] != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestUnregisteredContractFileIsRed(t *testing.T) {
	root := mini(t, nil)
	_ = os.WriteFile(filepath.Join(root, "specs", "c.cue"), []byte("package specs\n"), 0o644)
	_, err := Verify(root, now)
	wantCode(t, err, CodeUnregistered, "specs/c.cue is a contract file with no registry entry")
	_ = os.Remove(filepath.Join(root, "specs", "c.cue"))
	_ = os.WriteFile(filepath.Join(root, "specs", "c.schema.json"), []byte("{}"), 0o644)
	_, err = Verify(root, now)
	wantCode(t, err, CodeUnregistered, "specs/c.schema.json")
}

func TestChangedWithoutBumpIsRed(t *testing.T) {
	root := mini(t, nil)
	_ = os.WriteFile(filepath.Join(root, "specs", "a.cue"), []byte(cueA+"// comment\n"), 0o644)
	_, err := Verify(root, now)
	wantCode(t, err, CodeChangedWithoutBump, "a: specs/a.cue changed")
	if !strings.Contains(err.Error(), "--accept a --new-version") {
		t.Fatalf("message must tell how to accept: %v", err)
	}
}

func TestAcceptBumpRequiresANewVersionDeclaredByTheFile(t *testing.T) {
	root := mini(t, nil)
	changed := strings.Replace(cueA, `*"1.0.0"`, `*"1.1.0"`, 1)
	_ = os.WriteFile(filepath.Join(root, "specs", "a.cue"), []byte(changed), 0o644)
	// same version → refused
	wantCode(t, AcceptBump(root, "a", "1.0.0"), CodeBumpRefused, "needs a NEW schema_version")
	// version the file does not declare → refused
	wantCode(t, AcceptBump(root, "a", "2.0.0"), CodeBumpRefused, `file declares schema_version "1.1.0", not the requested "2.0.0"`)
	// unknown id → refused
	wantCode(t, AcceptBump(root, "zzz", "1.1.0"), CodeBumpRefused, "zzz is not registered")
	// the right bump → registry updated and Verify green
	if err := AcceptBump(root, "a", "1.1.0"); err != nil {
		t.Fatal(err)
	}
	rep, err := Verify(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rows[0].SchemaVersion != "1.1.0" || rep.Rows[0].Sha256 != sha([]byte(changed)) {
		t.Fatalf("registry not updated: %+v", rep.Rows[0])
	}
}

func TestVersionMismatchIsRed(t *testing.T) {
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, `schema_version: "1.0.0"`, `schema_version: "9.9.9"`, 1)
	})
	_, err := Verify(root, now)
	wantCode(t, err, CodeVersionMismatch, `file declares schema_version "1.0.0", registry says "9.9.9"`)
}

func TestStableWithoutFixtureOrVersionIsRed(t *testing.T) {
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "valid: [specs/examples/a.valid.json]", "valid: []", 1)
	})
	_, err := Verify(root, now)
	wantCode(t, err, CodeStableWithoutFixture, "a: a stable contract must have at least one valid fixture")

	root = mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    stability: experimental\n    version_kind: none\n", "    stability: stable\n    version_kind: none\n", 1)
		*reg = strings.Replace(*reg, "fixtures: {valid: [], invalid: []}", "fixtures: {valid: [specs/examples/a.valid.json], invalid: []}", 1)
	})
	_, err = Verify(root, now)
	wantCode(t, err, CodeStableWithoutVersion, "b: a stable contract must carry a schema_version")
}

func TestMissingFilesAreRed(t *testing.T) {
	root := mini(t, nil)
	_ = os.Remove(filepath.Join(root, "specs", "examples", "a.valid.json"))
	_, err := Verify(root, now)
	wantCode(t, err, CodeFixtureMissing, "a: fixture specs/examples/a.valid.json")
	root = mini(t, func(reg *string) { *reg = strings.Replace(*reg, "path: specs/b.cue", "path: specs/gone.cue", 1) })
	_, err = Verify(root, now)
	wantCode(t, err, CodeFileMissing, "b: specs/gone.cue")
}

func TestDeprecationNeedsDatesAndRespectsRemoval(t *testing.T) {
	dep := func(extra string) func(*string) {
		return func(reg *string) {
			*reg = strings.Replace(*reg, "    stability: experimental\n", "    stability: deprecated\n"+extra, 1)
		}
	}
	_, err := Verify(mini(t, dep("")), now)
	wantCode(t, err, CodeDeprecatedWithoutDate, "b: deprecated contracts need deprecated_since and removal_not_before")
	_, err = Verify(mini(t, dep("    deprecated_since: 2026-09-01\n    removal_not_before: 2026-08-01\n")), now)
	wantCode(t, err, CodeDeprecatedWithoutDate, "removal_not_before must be after deprecated_since")
	_, err = Verify(mini(t, dep("    deprecated_since: 2026-01-01\n    removal_not_before: 2026-06-01\n")), now)
	wantCode(t, err, CodeDeprecatedPastRemoval, "b: removal_not_before 2026-06-01 has passed")
	rep, err := Verify(mini(t, dep("    deprecated_since: 2026-09-01\n    removal_not_before: 2027-03-01\n")), now)
	if err != nil || rep.ByStability["deprecated"] != 1 || !rep.Rows[1].Deprecated {
		t.Fatalf("%v %+v", err, rep)
	}
}

func TestCompatFixtureMustBeReadByAKnownReader(t *testing.T) {
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    compat_fixtures: []\n  - id: b", "    compat_fixtures:\n      - {path: specs/examples/a.valid.json, reader: nobody, schema_version: \"1.0.0\"}\n  - id: b", 1)
	})
	_, err := Verify(root, now)
	wantCode(t, err, CodeCompatUnknownReader, `reader "nobody" is not a known Go reader`)
	root = mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    compat_fixtures: []\n  - id: b", "    compat_fixtures:\n      - {path: specs/examples/a.valid.json, reader: portfolio-status, schema_version: \"nomos-portfolio-status-v1\"}\n  - id: b", 1)
	})
	_, err = Verify(root, now)
	wantCode(t, err, CodeCompatUnread, "specs/examples/a.valid.json via portfolio-status: not a nomos-portfolio-status-v1 document")
}

func TestCompatFixtureWithWrongDeclaredVersionIsRed(t *testing.T) {
	root := repoRoot(t)
	err := readCompat(root, CompatFixture{Path: "specs/examples/portfolio-status.valid.json", Reader: "portfolio-status", SchemaVersion: "nomos-portfolio-status-v0"})
	wantCode(t, err, CodeCompatUnread, "not a nomos-portfolio-status-v0 document")
	err = readCompat(root, CompatFixture{Path: "specs/examples/nomos-praxis-evidence.valid.yaml", Reader: "nomos-praxis-evidence", SchemaVersion: "other"})
	wantCode(t, err, CodeCompatUnread, `registry expects "other"`)
	err = readCompat(root, CompatFixture{Path: "specs/examples/nomos-praxis-mapping.valid.json", Reader: "nomos-praxis-mapping", SchemaVersion: "other"})
	wantCode(t, err, CodeCompatUnread, `registry expects "other"`)
	err = readCompat(root, CompatFixture{Path: "cli/internal/corpus/testdata/external-snapshot/snapshot.json", Reader: "external-snapshot", SchemaVersion: "other"})
	wantCode(t, err, CodeCompatUnread, `registry expects "other"`)
	err = readCompat(root, CompatFixture{Path: "cli/internal/corpus/testdata/external-snapshot/nope.json", Reader: "external-snapshot", SchemaVersion: "nomos.external-snapshot.v1"})
	wantCode(t, err, CodeCompatUnread, "via external-snapshot")
}

func TestRegistryShapeIsRefused(t *testing.T) {
	_, err := Verify(mini(t, func(reg *string) { *reg = strings.Replace(*reg, "nomos-contract-registry-v1", "v0", 1) }), now)
	wantCode(t, err, CodeRegistryInvalid, `schema_version "v0"`)
	_, err = Verify(mini(t, func(reg *string) { *reg += "unknown_top_level: 1\n" }), now)
	wantCode(t, err, CodeRegistryInvalid, "parse registry")
	_, err = Verify(mini(t, func(reg *string) { *reg = strings.Replace(*reg, "  - id: b", "  - id: a", 1) }), now)
	wantCode(t, err, CodeRegistryInvalid, `contract id "a" missing or duplicated`)
	_, err = Verify(mini(t, func(reg *string) { *reg = strings.Replace(*reg, "stability: experimental", "stability: solid", 1) }), now)
	wantCode(t, err, CodeRegistryInvalid, `stability "solid"`)
}

func TestCompatSnapshotTamperedRecordIsRed(t *testing.T) {
	src := filepath.Join(repoRoot(t), "cli", "internal", "corpus", "testdata", "external-snapshot")
	dir := t.TempDir()
	for _, n := range []string{"snapshot.json", "sources.jsonl"} {
		b, err := os.ReadFile(filepath.Join(src, n))
		if err != nil {
			t.Fatal(err)
		}
		if n == "sources.jsonl" {
			b = []byte(strings.Replace(string(b), "sha256:1111", "sha256:9111", 1))
		}
		_ = os.WriteFile(filepath.Join(dir, n), b, 0o644)
	}
	err := readCompat("/", CompatFixture{Path: filepath.ToSlash(filepath.Join(dir, "snapshot.json")), Reader: "external-snapshot", SchemaVersion: "nomos.external-snapshot.v1"})
	wantCode(t, err, CodeCompatUnread, "snapshot.json")
}

func TestCompatPortfolioStatusWithoutDigestIsRed(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "st.json"), []byte(`{"schema_version":"nomos-portfolio-status-v1"}`), 0o644)
	err := readCompat("/", CompatFixture{Path: filepath.ToSlash(filepath.Join(dir, "st.json")), Reader: "portfolio-status", SchemaVersion: "nomos-portfolio-status-v1"})
	wantCode(t, err, CodeCompatUnread, "not a nomos-portfolio-status-v1 document")
}
