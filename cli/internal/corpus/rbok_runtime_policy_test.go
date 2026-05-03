package corpus

import "testing"

// --- Layer classification ---

func TestLayerDoctrine01Rbok(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/referentiel/garanties.yaml")
	assertLayer(t, r, LayerDoctrine, "primary", RoleLawbook, false)
	assertUse(t, r, "structured_contract")
	assertUse(t, r, "citation_external")
	assertUse(t, r, "golden_case")
}

func TestLayerDoctrine01Referentiel(t *testing.T) {
	r := ClassifyRuntimeLayer("01_referentiel/source-manifest.yaml")
	assertLayer(t, r, LayerDoctrine, "primary", RoleLawbook, false)
}

func TestLayerDoctrine01RbokSubdir(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/domaines/assurance-habitation/clauses.md")
	assertLayer(t, r, LayerDoctrine, "primary", RoleLawbook, false)
}

func TestLayerRuntime02Parcours(t *testing.T) {
	r := ClassifyRuntimeLayer("02_parcours/assurance-habitation/souscription.yaml")
	assertLayer(t, r, LayerRuntime, "primary", RoleLawbook, false)
	assertUse(t, r, "structured_contract")
	assertUse(t, r, "golden_case")
}

func TestLayerRuntime02Domaines(t *testing.T) {
	r := ClassifyRuntimeLayer("02_domaines/habitation/garanties.yaml")
	assertLayer(t, r, LayerRuntime, "primary", RoleLawbook, false)
}

func TestLayerWorkbooks03(t *testing.T) {
	r := ClassifyRuntimeLayer("03_workbooks/output/report.xlsx")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
	assertUse(t, r, "citation_internal")
	if len(r.AllowedUses) != 1 {
		t.Fatalf("expected 1 allowed_use, got %d: %v", len(r.AllowedUses), r.AllowedUses)
	}
}

func TestLayerWorkbooks03Generated(t *testing.T) {
	r := ClassifyRuntimeLayer("03_generated/build/output.json")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
}

func TestLayerMeta00(t *testing.T) {
	r := ClassifyRuntimeLayer("00_meta/glossaire.yaml")
	assertLayer(t, r, LayerMeta, "primary", RoleLawbook, false)
	assertUse(t, r, "vector_index")
}

func TestLayerSchemas98(t *testing.T) {
	r := ClassifyRuntimeLayer("98_schemas/source-manifest.cue")
	assertLayer(t, r, LayerSchemas, "reference", RoleSchema, false)
	assertUse(t, r, "structured_contract")
}

func TestLayerReference99(t *testing.T) {
	r := ClassifyRuntimeLayer("99_RBOK_initial_pdf/CG-habitation-2026.pdf")
	assertLayer(t, r, LayerReference, "reference", RoleReference, false)
	assertUse(t, r, "human_review_only")
}

func TestLayerReference99Initial(t *testing.T) {
	r := ClassifyRuntimeLayer("99_initial_sources/code-assurances.pdf")
	assertLayer(t, r, LayerReference, "reference", RoleReference, false)
}

// --- Out of scope ---

func TestRuntimeOutOfScopeScript(t *testing.T) {
	r := ClassifyRuntimeLayer("scripts/deploy.sh")
	assertRuntimeOutOfScope(t, r)
}

func TestRuntimeOutOfScopeDSStore(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/.DS_Store")
	assertRuntimeOutOfScope(t, r)
}

func TestRuntimeOutOfScopeShellFile(t *testing.T) {
	r := ClassifyRuntimeLayer("setup.ps1")
	assertRuntimeOutOfScope(t, r)
}

func TestRuntimeOutOfScopeLockFile(t *testing.T) {
	r := ClassifyRuntimeLayer("package-lock.json")
	assertRuntimeOutOfScope(t, r)
}

func TestRuntimeOutOfScopeGit(t *testing.T) {
	r := ClassifyRuntimeLayer(".git/config")
	assertRuntimeOutOfScope(t, r)
}

// --- Derived override ---

func TestRuntimeDerivedGenerated(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/generated/output.json")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
}

func TestRuntimeDerivedTranslation(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/garanties.de.yaml")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
}

func TestRuntimeDerivedTestdata(t *testing.T) {
	r := ClassifyRuntimeLayer("02_parcours/testdata/fixture.yaml")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
}

func TestRuntimeDerivedGenFile(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/model_gen.go")
	assertLayer(t, r, LayerWorkbooks, "derived", RoleDerived, true)
}

// --- Unknown / secondary ---

func TestRuntimeUnknownLayer(t *testing.T) {
	r := ClassifyRuntimeLayer("04_archives/old-notes.md")
	assertLayer(t, r, LayerUnknown, "secondary", RoleLawbook, false)
}

func TestRuntimeUnknownRootFile(t *testing.T) {
	r := ClassifyRuntimeLayer("README.md")
	assertLayer(t, r, LayerUnknown, "secondary", RoleLawbook, false)
}

// --- Windows backslash ---

func TestRuntimeWindowsBackslash(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok\\domaines\\habitation.yaml")
	assertLayer(t, r, LayerDoctrine, "primary", RoleLawbook, false)
}

// --- Determinism ---

func TestRuntimeDeterminism(t *testing.T) {
	paths := []string{
		"01_rbok/referentiel/source.yaml",
		"02_parcours/habitation/etapes.yaml",
		"03_workbooks/report.xlsx",
		"00_meta/glossaire.yaml",
		"98_schemas/matrix.cue",
		"99_RBOK_initial_pdf/doc.pdf",
		"scripts/build.sh",
		"README.md",
	}
	for _, p := range paths {
		r1 := ClassifyRuntimeLayer(p)
		r2 := ClassifyRuntimeLayer(p)
		if r1.Layer != r2.Layer || r1.Priority != r2.Priority ||
			r1.Role != r2.Role || r1.Mutable != r2.Mutable {
			t.Fatalf("non-deterministic for %q", p)
		}
	}
}

// --- Mutability contract ---

func TestDoctrineMutabilityFalse(t *testing.T) {
	r := ClassifyRuntimeLayer("01_rbok/clauses.yaml")
	if r.Mutable {
		t.Fatal("doctrine layer must be immutable")
	}
}

func TestWorkbooksMutabilityTrue(t *testing.T) {
	r := ClassifyRuntimeLayer("03_workbooks/output.csv")
	if !r.Mutable {
		t.Fatal("workbooks layer must be mutable")
	}
}

func TestRuntimeMutabilityFalse(t *testing.T) {
	r := ClassifyRuntimeLayer("02_parcours/sinistre.yaml")
	if r.Mutable {
		t.Fatal("runtime layer must be immutable")
	}
}

// --- Full layout table-driven ---

func TestRuntimeFullLayout(t *testing.T) {
	cases := []struct {
		path     string
		layer    RuntimeLayer
		priority string
		role     SourceRole
		mutable  bool
	}{
		{"00_meta/index.yaml", LayerMeta, "primary", RoleLawbook, false},
		{"01_rbok/referentiel/garanties.yaml", LayerDoctrine, "primary", RoleLawbook, false},
		{"01_referentiel/normes/norme.md", LayerDoctrine, "primary", RoleLawbook, false},
		{"02_parcours/habitation/souscription.yaml", LayerRuntime, "primary", RoleLawbook, false},
		{"02_domaines/auto/franchise.yaml", LayerRuntime, "primary", RoleLawbook, false},
		{"03_workbooks/export/rapport.xlsx", LayerWorkbooks, "derived", RoleDerived, true},
		{"03_generated/output.json", LayerWorkbooks, "derived", RoleDerived, true},
		{"98_schemas/warranty.schema.json", LayerSchemas, "reference", RoleSchema, false},
		{"99_RBOK_initial_pdf/CG-2026.pdf", LayerReference, "reference", RoleReference, false},
		{"scripts/deploy.sh", LayerUnknown, "out_of_scope", RoleOutOfScope, false},
		{".DS_Store", LayerUnknown, "out_of_scope", RoleOutOfScope, false},
		{"01_rbok/generated/data.json", LayerWorkbooks, "derived", RoleDerived, true},
		{"01_rbok/terms.de.yaml", LayerWorkbooks, "derived", RoleDerived, true},
		{"04_other/notes.md", LayerUnknown, "secondary", RoleLawbook, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			r := ClassifyRuntimeLayer(tc.path)
			if r.Layer != tc.layer {
				t.Errorf("layer: got %q, want %q (reason: %s)", r.Layer, tc.layer, r.Reason)
			}
			if r.Priority != tc.priority {
				t.Errorf("priority: got %q, want %q", r.Priority, tc.priority)
			}
			if r.Role != tc.role {
				t.Errorf("role: got %q, want %q", r.Role, tc.role)
			}
			if r.Mutable != tc.mutable {
				t.Errorf("mutable: got %v, want %v", r.Mutable, tc.mutable)
			}
		})
	}
}

// --- Layer constant values ---

func TestLayerConstants(t *testing.T) {
	if LayerDoctrine != "doctrine" || LayerRuntime != "runtime" ||
		LayerWorkbooks != "workbooks" || LayerMeta != "meta" ||
		LayerSchemas != "schemas" || LayerReference != "reference" ||
		LayerUnknown != "unknown" {
		t.Fatal("layer constant mismatch")
	}
}

// --- helpers ---

func assertLayer(t *testing.T, r RuntimeLayerClassification, layer RuntimeLayer, priority string, role SourceRole, mutable bool) {
	t.Helper()
	if r.Layer != layer {
		t.Fatalf("expected layer %q, got %q (reason: %s)", layer, r.Layer, r.Reason)
	}
	if r.Priority != priority {
		t.Fatalf("expected priority %q, got %q", priority, r.Priority)
	}
	if r.Role != role {
		t.Fatalf("expected role %q, got %q", role, r.Role)
	}
	if r.Mutable != mutable {
		t.Fatalf("expected mutable=%v, got %v", mutable, r.Mutable)
	}
}

func assertRuntimeOutOfScope(t *testing.T, r RuntimeLayerClassification) {
	t.Helper()
	if r.Priority != "out_of_scope" || r.Role != RoleOutOfScope {
		t.Fatalf("expected out_of_scope, got priority=%q role=%q", r.Priority, r.Role)
	}
}

func assertUse(t *testing.T, r RuntimeLayerClassification, use string) {
	t.Helper()
	for _, u := range r.AllowedUses {
		if u == use {
			return
		}
	}
	t.Fatalf("expected allowed_use %q in %v", use, r.AllowedUses)
}
