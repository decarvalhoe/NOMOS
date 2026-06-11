package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VRC-21 (#564) — `nomos pack validate` fails CLOSED on every mutilation the
// issue names: vocab manquant, golden corpus rouge, preset cassé, claim
// boundary absent — plus the two VRC-20 mutants mirrored in-engine (code
// artifact, smuggled field). The happy path runs the REAL built-environment
// pack through the REAL bundle chain.

const testPackCommit = "0123456789abcdef0123456789abcdef01234567"

// scaffoldPack writes a minimal VALID pack into a temp repo root and returns
// (root, manifestPath). Each adversarial test then mutilates exactly one
// element before running the gate.
func scaffoldPack(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	packDir := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack")
	corpusDir := filepath.Join(root, "cli", "internal", "corpus", "testdata", "test-pack")
	for _, dir := range []string{filepath.Join(packDir, "presets"), corpusDir, filepath.Join(root, "specs", "examples")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("specs/examples/test-profile.valid.yaml", "domain_profile: test-pack\n")
	write("docs/regulated/domain-packs/test-pack/vocab.yaml", `record_type: pack_vocabulary
schema_version: "0.1.0"
domain_profile: test-pack
activity:
  - id: test.phase_a
    label_fr: "Phase A"
discipline_role:
  - id: test.role_a
    label_fr: "Rôle A"
`)
	write("docs/regulated/domain-packs/test-pack/sources.yaml", "domain_profile: test-pack\nconnectors: []\n")
	write("docs/regulated/domain-packs/test-pack/presets/a.lens.yaml", `id: LENS-TEST-A
include:
  all_of:
    - activity:
        - test.phase_a
exclude:
  any_of:
    - applicability: blocked
`)
	write("cli/internal/corpus/testdata/test-pack/case.md", "# Cas de preuve\n\nUn paragraphe suffisant pour produire un atome réel.\n")
	write("docs/regulated/domain-packs/test-pack/pack.yaml", `schema_version: nomos-domain-pack-v1
pack_id: test-pack
domain_profile: test-pack
profile_ref: specs/examples/test-profile.valid.yaml
claim_boundary: >-
  Pack de test — aucune conformité réglementaire revendiquée.
vocabularies:
  file: docs/regulated/domain-packs/test-pack/vocab.yaml
  axes: [activity, discipline_role]
source_register:
  file: docs/regulated/domain-packs/test-pack/sources.yaml
  contract: "#BuiltEnvironmentSourceConnectors"
lens_presets:
  - id: LENS-TEST-A
    file: docs/regulated/domain-packs/test-pack/presets/a.lens.yaml
golden_corpus:
  root: cli/internal/corpus/testdata/test-pack
  documents:
    - case.md
scorecard:
  - area: "test"
    status: applicable
    note: "fixture"
`)
	return root, filepath.Join(packDir, "pack.yaml")
}

func runPackValidate(t *testing.T, root, manifest string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"pack", "validate",
		"--manifest", manifest,
		"--repo-root", root,
		"--repo", "example/test-pack",
		"--commit", testPackCommit,
	}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func mutateManifest(t *testing.T, manifest, old, new string) {
	t.Helper()
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("manifest does not contain %q", old)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(raw), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackValidate_ScaffoldPackIsGreen(t *testing.T) {
	root, manifest := scaffoldPack(t)
	code, stdout, stderr := runPackValidate(t, root, manifest)
	if code != 0 {
		t.Fatalf("expected green gate, got %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "pack validate: OK") || !strings.Contains(stdout, "test-pack") {
		t.Fatalf("unexpected verdict: %s", stdout)
	}
}

func TestPackValidate_RealBuiltEnvironmentPackIsGreen(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	manifest := filepath.Join(root, "docs", "regulated", "domain-packs", "built-environment", "pack.yaml")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("the real pack manifest is missing: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"pack", "validate",
		"--manifest", manifest,
		"--repo-root", root,
		// Explicit trace context: the gate must stay green in checkouts
		// whose origin remote is absent (CI tarballs, detached worktrees).
		"--repo", "RBOKproject/NOMOS",
		"--commit", testPackCommit,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("the real pack fails its own gate: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "built-environment") {
		t.Fatalf("unexpected verdict: %s", stdout.String())
	}
}

func TestPackValidate_MissingVocabularyFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	if err := os.Remove(filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "vocab.yaml")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("vocab manquant: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [artifacts]") {
		t.Fatalf("expected the artifacts rung to name the failure: %s", stderr)
	}
}

func TestPackValidate_EmptyAxisFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	// The file exists but the declared axis carries no terms.
	vocab := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "vocab.yaml")
	if err := os.WriteFile(vocab, []byte("record_type: pack_vocabulary\nschema_version: \"0.1.0\"\ndomain_profile: test-pack\nactivity: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("empty axis: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [vocabulary]") {
		t.Fatalf("expected the vocabulary rung: %s", stderr)
	}
}

func TestPackValidate_RedGoldenCorpusFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	// An EMPTY document atomizes to zero nodes — knowledge the chain cannot
	// cite is a red corpus, not a tolerated gap.
	if err := os.WriteFile(filepath.Join(root, "cli", "internal", "corpus", "testdata", "test-pack", "case.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("golden rouge (zéro node): the gate passed")
	}
	// The chain may refuse at the bundle level ("no content-bearing atoms")
	// or at the per-document count ("zero nodes") — both are the same rung
	// failing closed.
	if !strings.Contains(stderr, "FAIL [golden_corpus]") {
		t.Fatalf("expected the golden_corpus rung to fail closed: %s", stderr)
	}
}

func TestPackValidate_MalformedMarkupGoldenCorpusFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	// A malformed XML document fails the WHOLE bundle build (W23-1 rule) —
	// the gate must surface that as a red corpus too.
	if err := os.WriteFile(filepath.Join(root, "cli", "internal", "corpus", "testdata", "test-pack", "broken.xml"), []byte("<a><b>"), 0o644); err != nil {
		t.Fatal(err)
	}
	mutateManifest(t, manifest, "    - case.md", "    - case.md\n    - broken.xml")
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("golden rouge (markup malformé): the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [golden_corpus]") {
		t.Fatalf("expected the golden_corpus rung: %s", stderr)
	}
}

func TestPackValidate_BrokenPresetUnresolvedTermFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	preset := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "presets", "a.lens.yaml")
	if err := os.WriteFile(preset, []byte("id: LENS-TEST-A\ninclude:\n  all_of:\n    - activity:\n        - test.phantom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("preset cassé (terme fantôme): the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [lens_presets]") || !strings.Contains(stderr, "test.phantom") {
		t.Fatalf("expected the lens_presets rung to name the phantom term: %s", stderr)
	}
}

func TestPackValidate_BrokenPresetWithoutPredicatesFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	preset := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "presets", "a.lens.yaml")
	if err := os.WriteFile(preset, []byte("id: LENS-TEST-A\ndescription: rien\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("preset cassé (aucun prédicat): the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [lens_presets]") {
		t.Fatalf("expected the lens_presets rung: %s", stderr)
	}
}

func TestPackValidate_AbsentClaimBoundaryFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	mutateManifest(t, manifest,
		"claim_boundary: >-\n  Pack de test — aucune conformité réglementaire revendiquée.",
		"claim_boundary: \"   \"")
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("claim boundary absent: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [claim_boundary]") {
		t.Fatalf("expected the claim_boundary rung: %s", stderr)
	}
}

func TestPackValidate_CodeArtifactFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	mutateManifest(t, manifest,
		"file: docs/regulated/domain-packs/test-pack/vocab.yaml",
		"file: docs/regulated/domain-packs/test-pack/vocab.py")
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("artefact code (.py): the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [declarative]") {
		t.Fatalf("expected the declarative rung: %s", stderr)
	}
}

func TestPackValidate_SmuggledFieldFailsClosed(t *testing.T) {
	root, manifest := scaffoldPack(t)
	mutateManifest(t, manifest,
		"schema_version: nomos-domain-pack-v1",
		"schema_version: nomos-domain-pack-v1\nentrypoint: cli/cmd/pack/main.go")
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("champ hors contrat: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [manifest]") {
		t.Fatalf("expected the manifest rung (strict decode): %s", stderr)
	}
}
