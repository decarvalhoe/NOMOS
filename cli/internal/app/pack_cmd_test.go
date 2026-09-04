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
	write("docs/regulated/domain-packs/test-pack/facet-ontology.yaml", `schema_version: ckm-facet-ontology-v1
facet_axes:
  - id: activity
    root: http://purl.obolibrary.org/obo/BFO_0000015
    iof_class: https://spec.industrialontologies.org/ontology/core/Core/Process
    terms:
      - id: test.phase_a
        maps_to:
          bfo: http://purl.obolibrary.org/obo/BFO_0000015
          iof_core: https://spec.industrialontologies.org/ontology/core/Core/Process
  - id: discipline_role
    root: http://purl.obolibrary.org/obo/BFO_0000023
    iof_class: https://spec.industrialontologies.org/ontology/core/Core/AgentRole
    terms:
      - id: test.role_a
        maps_to:
          bfo: http://purl.obolibrary.org/obo/BFO_0000023
          iof_core: https://spec.industrialontologies.org/ontology/core/Core/AgentRole
orthogonality:
  owl_construct: owl:disjointUnionOf
  disjoint_axes: [activity, discipline_role]
claim_boundary: test
`)
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
ontology:
  file: docs/regulated/domain-packs/test-pack/facet-ontology.yaml
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

func TestPackValidate_UnregisteredAxisFailsClosed(t *testing.T) {
	// VRC-45 (D4): the ontology drops the discipline_role axis while the
	// pack still declares it — « axe non aligné → pack rejeté ».
	root, manifest := scaffoldPack(t)
	onto := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "facet-ontology.yaml")
	raw, err := os.ReadFile(onto)
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Split(string(raw), "  - id: discipline_role")[0] + "orthogonality:\n  owl_construct: owl:disjointUnionOf\n  disjoint_axes: [activity]\nclaim_boundary: test\n"
	if err := os.WriteFile(onto, []byte(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("axe non enregistré: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [ontology]") || !strings.Contains(stderr, "discipline_role") {
		t.Fatalf("expected the ontology rung to name the unregistered axis: %s", stderr)
	}
}

func TestPackValidate_UnmappedTermFailsClosed(t *testing.T) {
	// VRC-45 (D4): the vocabulary grows a term the ontology never maps.
	root, manifest := scaffoldPack(t)
	vocab := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "vocab.yaml")
	raw, err := os.ReadFile(vocab)
	if err != nil {
		t.Fatal(err)
	}
	// Insert the new ACTIVITY term right before the discipline_role block.
	if err := os.WriteFile(vocab, []byte(strings.Replace(string(raw),
		"discipline_role:",
		"  - id: test.phase_b\n    label_fr: \"Phase B (jamais alignée)\"\ndiscipline_role:", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("terme non mappé: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [ontology]") || !strings.Contains(stderr, "test.phase_b") {
		t.Fatalf("expected the ontology rung to name the unmapped term: %s", stderr)
	}
}

func TestPackValidate_DisjointAxisOverlapFailsClosed(t *testing.T) {
	// VRC-45 (D4): the same term registered on BOTH disjoint axes violates
	// owl:disjointUnionOf — the gate renders the sidecar's verdict itself.
	root, manifest := scaffoldPack(t)
	onto := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "facet-ontology.yaml")
	raw, err := os.ReadFile(onto)
	if err != nil {
		t.Fatal(err)
	}
	overlapped := strings.Replace(string(raw),
		"      - id: test.role_a",
		"      - id: test.phase_a\n        maps_to:\n          bfo: http://purl.obolibrary.org/obo/BFO_0000023\n          iof_core: https://spec.industrialontologies.org/ontology/core/Core/AgentRole\n      - id: test.role_a", 1)
	if err := os.WriteFile(onto, []byte(overlapped), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("chevauchement d'axes disjoints: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [ontology]") || !strings.Contains(stderr, "disjointUnionOf") {
		t.Fatalf("expected the ontology rung to name the disjointness breach: %s", stderr)
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

// --- VRC-22 (#565): risk_tier, the third open-term axis ---------------------
//
// The D6 measurement of VRC-22 is "how much core changed to admit a second
// vertical". The answer is one named change: risk_tier joined the open axes,
// because the EU AI Act classifies by risk and no closed axis carries that
// meaning. These tests pin what that change did AND what it deliberately did
// not do — the core still owns the axis list.

// riskTierPack turns the scaffold into a pack that also declares risk_tier.
func riskTierPack(t *testing.T) (string, string) {
	t.Helper()
	root, manifest := scaffoldPack(t)
	packDir := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack")
	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(packDir, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("vocab.yaml", `record_type: pack_vocabulary
schema_version: "0.1.0"
domain_profile: test-pack
activity:
  - id: test.phase_a
    label_fr: "Phase A"
discipline_role:
  - id: test.role_a
    label_fr: "Rôle A"
risk_tier:
  - id: test.risque_eleve
    label_fr: "Risque élevé"
`)
	write("facet-ontology.yaml", `schema_version: ckm-facet-ontology-v1
facet_axes:
  - id: activity
    root: http://purl.obolibrary.org/obo/BFO_0000015
    iof_class: https://spec.industrialontologies.org/ontology/core/Core/Process
    terms:
      - id: test.phase_a
        maps_to:
          bfo: http://purl.obolibrary.org/obo/BFO_0000015
          iof_core: https://spec.industrialontologies.org/ontology/core/Core/Process
  - id: discipline_role
    root: http://purl.obolibrary.org/obo/BFO_0000023
    iof_class: https://spec.industrialontologies.org/ontology/core/Core/AgentRole
    terms:
      - id: test.role_a
        maps_to:
          bfo: http://purl.obolibrary.org/obo/BFO_0000023
          iof_core: https://spec.industrialontologies.org/ontology/core/Core/AgentRole
  - id: risk_tier
    root: http://purl.obolibrary.org/obo/BFO_0000019
    iof_class: https://spec.industrialontologies.org/ontology/core/Core/Quality
    terms:
      - id: test.risque_eleve
        maps_to:
          bfo: http://purl.obolibrary.org/obo/BFO_0000019
          iof_core: https://spec.industrialontologies.org/ontology/core/Core/Quality
orthogonality:
  owl_construct: owl:disjointUnionOf
  disjoint_axes: [activity, discipline_role, risk_tier]
claim_boundary: test
`)
	mutateManifest(t, manifest,
		"axes: [activity, discipline_role]",
		"axes: [activity, discipline_role, risk_tier]")
	return root, manifest
}

func TestPackValidate_RiskTierAxisIsAccepted(t *testing.T) {
	// The change actually admits a pack that classifies by risk.
	root, manifest := riskTierPack(t)
	code, stdout, stderr := runPackValidate(t, root, manifest)
	if code != 0 {
		t.Fatalf("risk_tier pack should be green: %s %s", stdout, stderr)
	}
}

func TestPackValidate_EmptyRiskTierAxisFailsClosed(t *testing.T) {
	// ADVERSARIAL: declaring the axis without terms is the same defect as for
	// the two older axes — the new axis gets no softer treatment.
	root, manifest := riskTierPack(t)
	vocab := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "vocab.yaml")
	raw, err := os.ReadFile(vocab)
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Index(string(raw), "risk_tier:")
	if cut < 0 {
		t.Fatal("fixture lost its risk_tier block")
	}
	if err := os.WriteFile(vocab, []byte(string(raw)[:cut]+"risk_tier: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("empty risk_tier: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [vocabulary]") {
		t.Fatalf("expected the vocabulary rung: %s", stderr)
	}
}

func TestPackValidate_UnnamespacedRiskTierTermFailsClosed(t *testing.T) {
	// ADVERSARIAL: packs own terms, and a term still has to be pack-namespaced.
	root, manifest := riskTierPack(t)
	vocab := filepath.Join(root, "docs", "regulated", "domain-packs", "test-pack", "vocab.yaml")
	raw, err := os.ReadFile(vocab)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(raw), "id: test.risque_eleve", "id: risque_eleve", 1)
	if err := os.WriteFile(vocab, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("unnamespaced risk_tier term: the gate passed")
	}
	if !strings.Contains(stderr, "FAIL [vocabulary]") && !strings.Contains(stderr, "FAIL [ontology]") {
		t.Fatalf("expected vocabulary or ontology rung: %s", stderr)
	}
}

func TestPackValidate_CoreStillOwnsTheAxisList(t *testing.T) {
	// THE POINT OF THE D6 MEASUREMENT: one axis was added to the core, and the
	// door did NOT open. A pack still cannot invent an axis of its own — that
	// remains a core change, made deliberately, not a pack-side extension.
	root, manifest := riskTierPack(t)
	mutateManifest(t, manifest,
		"axes: [activity, discipline_role, risk_tier]",
		"axes: [activity, discipline_role, risk_tier, sector]")
	code, _, stderr := runPackValidate(t, root, manifest)
	if code == 0 {
		t.Fatal("pack-invented axis: the gate passed")
	}
	if !strings.Contains(stderr, "packs own TERMS; core owns AXES") {
		t.Fatalf("expected the ownership boundary to be named: %s", stderr)
	}
}

// Non-regression for the first vertical after the core change is already
// covered by TestPackValidate_RealBuiltEnvironmentPackIsGreen above, which runs
// the shipped built-environment pack — which declares no risk_tier — through
// the same gate on the real tree.

func TestPackValidate_RealEUAIActPackIsGreen(t *testing.T) {
	// VRC-22 (#565): the SECOND vertical passes the SAME gate, unchanged, on
	// the real tree. Two verticals through one gate is what makes the
	// generality claim measurable instead of asserted.
	root := filepath.Join("..", "..", "..")
	manifest := filepath.Join(root, "docs", "regulated", "domain-packs", "eu-ai-act", "pack.yaml")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("the EU AI Act pack manifest is missing: %v", err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"pack", "validate",
		"--manifest", manifest,
		"--repo-root", root,
		"--repo", "RBOKproject/NOMOS",
		"--commit", testPackCommit,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("the EU AI Act pack fails its own gate: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "eu-ai-act") {
		t.Fatalf("unexpected verdict: %s", out)
	}
	// The vertical must actually exercise the axis it cost the core.
	if !strings.Contains(out, "3 axe(s)") {
		t.Fatalf("the pack should align three axes (risk_tier included): %s", out)
	}
}
