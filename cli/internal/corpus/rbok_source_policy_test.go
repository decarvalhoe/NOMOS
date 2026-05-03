package corpus

import "testing"

func TestClassifyPrimary00Meta(t *testing.T) {
	r := ClassifyRBOKSource("00_meta/glossary.yaml")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, "active", r.Status)
	policyAssertEqual(t, RoleLawbook, r.Role)
	assertContains(t, r.AllowedUses, "structured_contract")
	assertContains(t, r.AllowedUses, "vector_index")
	assertContains(t, r.AllowedUses, "golden_case")
}

func TestClassifyPrimary01Referentiel(t *testing.T) {
	r := ClassifyRBOKSource("01_referentiel/source-manifest.yaml")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, RoleLawbook, r.Role)
}

func TestClassifyPrimary02Domaines(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/assurance-habitation/garanties.yaml")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, RoleLawbook, r.Role)
}

func TestClassifyReference99RBOK(t *testing.T) {
	r := ClassifyRBOKSource("99_RBOK_initial_pdf/contract-home-2026.pdf")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleReference, r.Role)
	assertContains(t, r.AllowedUses, "human_review_only")
	assertContains(t, r.AllowedUses, "citation_internal")
}

func TestClassifySchema98Schemas(t *testing.T) {
	r := ClassifyRBOKSource("98_schemas/warranty.schema.json")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleSchema, r.Role)
	assertContains(t, r.AllowedUses, "structured_contract")
	assertContains(t, r.AllowedUses, "citation_internal")
}

func TestClassifyDerivedGermanTranslation(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/habitation/garanties.de.yaml")
	policyAssertEqual(t, "derived", r.Priority)
	policyAssertEqual(t, RoleDerived, r.Role)
	assertContains(t, r.AllowedUses, "citation_internal")
}

func TestClassifyDerivedGenerated(t *testing.T) {
	r := ClassifyRBOKSource("01_referentiel/generated/output.json")
	policyAssertEqual(t, "derived", r.Priority)
	policyAssertEqual(t, RoleDerived, r.Role)
}

func TestClassifyDerivedTestdata(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/testdata/fixture.yaml")
	policyAssertEqual(t, "derived", r.Priority)
	policyAssertEqual(t, RoleDerived, r.Role)
}

func TestClassifyDerivedFixtures(t *testing.T) {
	r := ClassifyRBOKSource("01_referentiel/fixtures/sample.json")
	policyAssertEqual(t, "derived", r.Priority)
}

func TestClassifyOutOfScopeScript(t *testing.T) {
	r := ClassifyRBOKSource("scripts/deploy.sh")
	policyAssertEqual(t, "out_of_scope", r.Priority)
	policyAssertEqual(t, "out_of_scope", r.Status)
	policyAssertEqual(t, RoleOutOfScope, r.Role)
	if len(r.AllowedUses) != 0 {
		t.Fatalf("expected no allowed_uses, got %v", r.AllowedUses)
	}
}

func TestClassifyOutOfScopeDSStore(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/.DS_Store")
	policyAssertEqual(t, "out_of_scope", r.Priority)
	policyAssertEqual(t, RoleOutOfScope, r.Role)
}

func TestClassifyOutOfScopeShellFile(t *testing.T) {
	r := ClassifyRBOKSource("00_meta/setup.sh")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifyOutOfScopeLockFile(t *testing.T) {
	r := ClassifyRBOKSource("package-lock.json")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifyOutOfScopeGitDir(t *testing.T) {
	r := ClassifyRBOKSource(".git/config")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifyOutOfScopeNodeModules(t *testing.T) {
	r := ClassifyRBOKSource("node_modules/react/index.js")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifySecondaryDefault(t *testing.T) {
	r := ClassifyRBOKSource("03_other/notes.md")
	policyAssertEqual(t, "secondary", r.Priority)
	policyAssertEqual(t, "active", r.Status)
	policyAssertEqual(t, RoleLawbook, r.Role)
	assertContains(t, r.AllowedUses, "structured_contract")
}

func TestClassifyDerivedDeYaml(t *testing.T) {
	r := ClassifyRBOKSource("00_meta/terms.de.yaml")
	policyAssertEqual(t, "derived", r.Priority)
}

func TestClassifyDerivedDeMd(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/guide.de.md")
	policyAssertEqual(t, "derived", r.Priority)
}

func TestClassifyDerivedGenFile(t *testing.T) {
	r := ClassifyRBOKSource("01_referentiel/model_gen.go")
	policyAssertEqual(t, "derived", r.Priority)
}

func TestClassifyPrimarySubdirectory(t *testing.T) {
	r := ClassifyRBOKSource("00_meta/sub/deep/file.yaml")
	policyAssertEqual(t, "primary", r.Priority)
}

func TestClassifyReferenceInitialPDF(t *testing.T) {
	r := ClassifyRBOKSource("99_initial_sources/contract_initial.pdf")
	policyAssertEqual(t, "reference", r.Priority)
}

func TestClassifyOutOfScopeThumbsDb(t *testing.T) {
	r := ClassifyRBOKSource("02_domaines/Thumbs.db")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifyOutOfScopeBatFile(t *testing.T) {
	r := ClassifyRBOKSource("run.bat")
	policyAssertEqual(t, "out_of_scope", r.Priority)
}

func TestClassifyDeterminism(t *testing.T) {
	paths := []string{
		"00_meta/glossary.yaml",
		"99_RBOK_initial_pdf/doc.pdf",
		"scripts/build.sh",
		"02_domaines/terms.de.yaml",
		"03_other/readme.md",
	}
	for _, p := range paths {
		r1 := ClassifyRBOKSource(p)
		r2 := ClassifyRBOKSource(p)
		policyAssertEqual(t, r1.Priority, r2.Priority)
		policyAssertEqual(t, r1.Status, r2.Status)
		policyAssertEqual(t, r1.Role, r2.Role)
		policyAssertEqual(t, r1.Reason, r2.Reason)
	}
}

func TestClassifyWindowsBackslash(t *testing.T) {
	r := ClassifyRBOKSource("00_meta\\glossary.yaml")
	policyAssertEqual(t, "primary", r.Priority)
}

func TestClassifyRealisonsBusinessCanonicalCore(t *testing.T) {
	r := ClassifyRBOKSource("01_rbok/00_meta/RBOK_structure_v1.md")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, "active", r.Status)
	policyAssertEqual(t, RoleLawbook, r.Role)
	policyAssertEqual(t, "canonical_corpus", r.SourceClass)
	policyAssertEqual(t, "canonical_core", r.CorpusLayer)
	policyAssertEqual(t, "primary", r.Authority)
	assertContains(t, r.AllowedUses, "vector_index")
}

func TestClassifyRealisonsBusinessRuntimeBinding(t *testing.T) {
	r := ClassifyRBOKSource("01_rbok/03_parcours/PAR_ACC_ADMIN.yaml")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, RoleLawbook, r.Role)
	policyAssertEqual(t, "runtime_binding", r.SourceClass)
	policyAssertEqual(t, "runtime_binding", r.CorpusLayer)
	policyAssertEqual(t, "primary", r.Authority)
	assertContains(t, r.AllowedUses, "runtime_binding")
}

func TestClassifyRealisonsBusinessSupportingLayers(t *testing.T) {
	cases := []struct {
		path        string
		role        SourceRole
		sourceClass string
		layer       string
		authority   string
	}{
		{"02_organisation/equipe.md", RoleSupporting, "supporting_context", "organisation", "supporting"},
		{"03_catalogue_services/catalogue.md", RoleSupporting, "supporting_context", "service_catalog", "supporting"},
		{"04_marketing/cas-client.md", RoleEvidence, "experience_evidence", "market_context", "evidence"},
		{"05_pilotage/kpi.md", RoleOperational, "operational_context", "pilotage", "operational"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			r := ClassifyRBOKSource(tc.path)
			policyAssertEqual(t, "secondary", r.Priority)
			policyAssertEqual(t, "active", r.Status)
			policyAssertEqual(t, tc.role, r.Role)
			policyAssertEqual(t, tc.sourceClass, r.SourceClass)
			policyAssertEqual(t, tc.layer, r.CorpusLayer)
			policyAssertEqual(t, tc.authority, r.Authority)
			assertContains(t, r.AllowedUses, "citation_internal")
		})
	}
}

func TestClassifyRealisonsBusinessArchiveExcludedByDefault(t *testing.T) {
	r := ClassifyRBOKSource("99_archive/old/RBOK_structure_v0.md")
	policyAssertEqual(t, "archive", r.Priority)
	policyAssertEqual(t, "out_of_scope", r.Status)
	policyAssertEqual(t, RoleArchive, r.Role)
	policyAssertEqual(t, "archive", r.SourceClass)
	policyAssertEqual(t, "archive", r.CorpusLayer)
	policyAssertEqual(t, "none", r.Authority)
	if len(r.AllowedUses) != 0 {
		t.Fatalf("expected no allowed uses for archive by default, got %v", r.AllowedUses)
	}
}

// --- 03_parcours as primary ---

func TestClassifyPrimary03Parcours(t *testing.T) {
	r := ClassifyRBOKSource("03_parcours/assurance-habitation/parcours.yaml")
	policyAssertEqual(t, "primary", r.Priority)
	policyAssertEqual(t, RoleLawbook, r.Role)
	assertContains(t, r.AllowedUses, "structured_contract")
	assertContains(t, r.AllowedUses, "vector_index")
	assertContains(t, r.AllowedUses, "golden_case")
}

func TestClassifyPrimary03ParcoursSubdir(t *testing.T) {
	r := ClassifyRBOKSource("03_parcours/sinistres/etapes/declaration.yaml")
	policyAssertEqual(t, "primary", r.Priority)
}

// --- 98_schemas distinct from 99_RBOK ---

func TestClassifySchema98SchemaCUE(t *testing.T) {
	r := ClassifyRBOKSource("98_schemas/source-manifest.cue")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleSchema, r.Role)
}

func TestClassifySchema98SchemaSubdir(t *testing.T) {
	r := ClassifyRBOKSource("98_schemas/canonical/matrix.schema.json")
	policyAssertEqual(t, RoleSchema, r.Role)
}

// --- 99_RBOK_initial_pdf ---

func TestClassifyReference99RBOKInitialPdfExact(t *testing.T) {
	r := ClassifyRBOKSource("99_RBOK_initial_pdf/conditions-generales-habitation-2026.pdf")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleReference, r.Role)
	assertContains(t, r.AllowedUses, "human_review_only")
}

func TestClassifyReference99InitialSubdir(t *testing.T) {
	r := ClassifyRBOKSource("99_initial_sources/regulations/code-assurances.pdf")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleReference, r.Role)
}

// --- Real layout integration test ---

func TestClassifyRealRBOKLayout(t *testing.T) {
	cases := []struct {
		path     string
		priority string
		role     SourceRole
	}{
		{"00_meta/glossaire.yaml", "primary", RoleLawbook},
		{"00_meta/index/tables-reference.yaml", "primary", RoleLawbook},
		{"01_referentiel/source-manifest.yaml", "primary", RoleLawbook},
		{"01_referentiel/normes/norme-construction.md", "primary", RoleLawbook},
		{"02_domaines/assurance-habitation/garanties.yaml", "primary", RoleLawbook},
		{"02_domaines/assurance-auto/franchise.yaml", "primary", RoleLawbook},
		{"03_parcours/assurance-habitation/souscription.yaml", "primary", RoleLawbook},
		{"03_parcours/assurance-habitation/sinistre.yaml", "primary", RoleLawbook},
		{"98_schemas/source-manifest.cue", "reference", RoleSchema},
		{"98_schemas/canonical-matrix.cue", "reference", RoleSchema},
		{"99_RBOK_initial_pdf/CG-habitation-2026.pdf", "reference", RoleReference},
		{"99_RBOK_initial_pdf/code-assurances-extract.pdf", "reference", RoleReference},
		{"scripts/build.sh", "out_of_scope", RoleOutOfScope},
		{".DS_Store", "out_of_scope", RoleOutOfScope},
		{"02_domaines/habitation/garanties.de.yaml", "derived", RoleDerived},
		{"04_archives/old-notes.md", "secondary", RoleLawbook},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			r := ClassifyRBOKSource(tc.path)
			if r.Priority != tc.priority {
				t.Errorf("priority: expected %q, got %q (reason: %s)", tc.priority, r.Priority, r.Reason)
			}
			if r.Role != tc.role {
				t.Errorf("role: expected %q, got %q (reason: %s)", tc.role, r.Role, r.Reason)
			}
		})
	}
}

// --- helper ---

func policyAssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func assertContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Fatalf("expected %v to contain %q", slice, want)
}
