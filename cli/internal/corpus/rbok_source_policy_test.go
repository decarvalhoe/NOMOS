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

func TestClassifyReference98Schemas(t *testing.T) {
	r := ClassifyRBOKSource("98_schemas/warranty.schema.json")
	policyAssertEqual(t, "reference", r.Priority)
	policyAssertEqual(t, RoleReference, r.Role)
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
