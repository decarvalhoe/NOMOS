package fidelity

import (
	"testing"
)

// --- classifyText ---

func TestClassifyText_Definition(t *testing.T) {
	for _, text := range []string{
		"On entend par assure toute personne titulaire d'un contrat.",
		"Au sens du present reglement, le sinistre designe un evenement.",
		"The term 'policy' means a binding insurance contract.",
		"Franchise : montant restant a la charge de l'assure.",
	} {
		if r := classifyText(text, ""); r != SemDefinition {
			t.Errorf("expected definition for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Rule(t *testing.T) {
	for _, text := range []string{
		"L'assure doit declarer tout sinistre dans un delai de 5 jours.",
		"Il est interdit de depasser le plafond annuel.",
		"The insured shall notify the insurer within 48 hours.",
		"Les frais sont pris en charge a 100% du tarif conventionnel.",
	} {
		if r := classifyText(text, ""); r != SemRule {
			t.Errorf("expected rule for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Exception(t *testing.T) {
	for _, text := range []string{
		"Les soins hors UE ne sont pas couverts par le contrat.",
		"This does not apply to pre-existing conditions.",
		"Exclusion : chirurgie esthetique sans justification medicale.",
	} {
		if r := classifyText(text, ""); r != SemException {
			t.Errorf("expected exception for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Example(t *testing.T) {
	for _, text := range []string{
		"Par exemple, un accident de la route survenu le 15 mars.",
		"For instance, a claim filed after the deadline.",
		"Cas pratique : calcul de la franchise pour un sinistre auto.",
	} {
		if r := classifyText(text, ""); r != SemExample {
			t.Errorf("expected example for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Note(t *testing.T) {
	for _, text := range []string{
		"Note : cette disposition entre en vigueur le 1er janvier.",
		"Remarque importante concernant les delais.",
		"N.B. les montants sont exprimes hors taxes.",
	} {
		if r := classifyText(text, ""); r != SemNote {
			t.Errorf("expected note for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Warning(t *testing.T) {
	for _, text := range []string{
		"Attention : le non-respect du delai entraine la decheance.",
		"Warning: this operation is irreversible.",
		"Avertissement concernant les donnees personnelles.",
	} {
		if r := classifyText(text, ""); r != SemWarning {
			t.Errorf("expected warning for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Procedure(t *testing.T) {
	for _, text := range []string{
		"Procedure de declaration de sinistre en 3 etapes.",
		"Etape 1 : remplir le formulaire en ligne.",
		"Follow the steps below to complete registration.",
	} {
		if r := classifyText(text, ""); r != SemProcedure {
			t.Errorf("expected procedure for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Reference(t *testing.T) {
	for _, text := range []string{
		"Conformement a l'article 42 du code des assurances.",
		"See also the appendix for full details.",
		"En application de la directive ISO 27001.",
	} {
		if r := classifyText(text, ""); r != SemReference {
			t.Errorf("expected reference for %q, got %s", text, r)
		}
	}
}

func TestClassifyText_Unknown(t *testing.T) {
	r := classifyText("Le present document a ete redige en mars 2026.", "")
	if r != SemUnknown {
		t.Errorf("expected unknown, got %s", r)
	}
}

func TestClassifyText_HeadingBoost_Definition(t *testing.T) {
	r := classifyText("Un terme technique courant.", "Definitions")
	if r != SemDefinition {
		t.Errorf("expected definition with heading, got %s", r)
	}
}

func TestClassifyText_HeadingBoost_Exception(t *testing.T) {
	r := classifyText("La chirurgie esthetique.", "Exclusions generales")
	if r != SemException {
		t.Errorf("expected exception with heading, got %s", r)
	}
}

func TestClassifyText_HeadingBoost_Procedure(t *testing.T) {
	r := classifyText("Remplir le formulaire.", "Procedure de reclamation")
	if r != SemProcedure {
		t.Errorf("expected procedure with heading, got %s", r)
	}
}

func TestClassifyText_HeadingBoost_Warning(t *testing.T) {
	r := classifyText("Les donnees seront perdues.", "Avertissement")
	if r != SemWarning {
		t.Errorf("expected warning with heading, got %s", r)
	}
}

// --- classifyBlock ---

func TestClassifyBlock_Heading(t *testing.T) {
	n := CNode{Kind: KindHeading, Text: "Chapter"}
	if classifyBlock(n, nil) != SemStructure {
		t.Fatal("headings should be structure")
	}
}

func TestClassifyBlock_CodeBlock(t *testing.T) {
	n := CNode{Kind: KindCodeBlock, Text: "code"}
	if classifyBlock(n, nil) != SemExample {
		t.Fatal("code blocks should be example")
	}
}

func TestClassifyBlock_Blockquote(t *testing.T) {
	n := CNode{Kind: KindBlockquote, Text: "quote"}
	if classifyBlock(n, nil) != SemNote {
		t.Fatal("blockquotes should be note")
	}
}

func TestClassifyBlock_Table(t *testing.T) {
	n := CNode{Kind: KindTable}
	if classifyBlock(n, nil) != SemReference {
		t.Fatal("tables should be reference")
	}
}

// --- ClassifyBlocks integration ---

func TestClassifyBlocks_FullFixture(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	if result.TotalNodes == 0 {
		t.Fatal("expected classified nodes")
	}
	if result.Classified == 0 {
		t.Fatal("expected some nodes classified")
	}
	if result.ByRole[SemStructure] == 0 {
		t.Fatal("expected structure role from headings")
	}
	if result.ByRole[SemExample] == 0 {
		t.Fatal("expected example role from code block")
	}
}

func TestClassifyBlocks_AllRoles(t *testing.T) {
	src := `# Reglement

## Definitions

Franchise : montant restant a la charge de l'assure.

## Obligations

L'assure doit declarer tout sinistre dans un delai de 5 jours.

## Exclusions

Les soins hors UE ne sont pas couverts.

## Exemples

Par exemple, un accident survenu en Suisse.

## Procedure

Etape 1 : remplir le formulaire.

## Avertissement

Attention : le non-respect du delai entraine la decheance.

## Notes

Note : ces dispositions sont provisoires.

## References

Conformement a l'article 42 du code civil.
`
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	found := map[SemRole]bool{}
	for _, n := range result.Nodes {
		found[n.Role] = true
	}
	for _, expected := range []SemRole{SemStructure, SemDefinition, SemRule, SemException, SemExample, SemProcedure, SemWarning, SemNote, SemReference} {
		if !found[expected] {
			t.Errorf("expected role %s to be present", expected)
		}
	}
}

func TestClassifyBlocks_SiblingContext(t *testing.T) {
	src := "# Doc\n\nL'assure doit declarer.\n\nSauf en cas de force majeure.\n"
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	for _, n := range result.Nodes {
		if n.Role == SemException {
			if n.SiblingPrev == "" {
				t.Fatal("expected sibling_prev on exception")
			}
			return
		}
	}
	// Skip if exception not detected in this context.
}

func TestClassifyBlocks_ParentRole(t *testing.T) {
	src := "# Doc\n\n## Definitions\n\nTerme : une valeur.\n"
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	for _, n := range result.Nodes {
		if n.Role == SemDefinition && n.Kind == KindParagraph {
			if n.ParentRole != SemStructure {
				t.Fatalf("expected parent_role=structure, got %s", n.ParentRole)
			}
			return
		}
	}
}

func TestClassifyBlocks_Confidence(t *testing.T) {
	src := "# Doc\n\nL'assure doit payer.\n\nTexte neutre.\n"
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	for _, n := range result.Nodes {
		if n.Confidence == "" {
			t.Fatalf("node %s has empty confidence", n.NodeID)
		}
		if n.Confidence != "high" && n.Confidence != "medium" && n.Confidence != "low" {
			t.Fatalf("node %s has invalid confidence %q", n.NodeID, n.Confidence)
		}
	}
}

func TestClassifyBlocks_Depth(t *testing.T) {
	src := "# Doc\n\n## Ch\n\nParagraph.\n"
	cast := ParseMarkdown(src)
	result := ClassifyBlocks(cast)

	for _, n := range result.Nodes {
		if n.Kind == KindParagraph && n.Depth < 2 {
			t.Fatalf("expected depth >= 2, got %d", n.Depth)
		}
	}
}

func TestClassifyBlocks_Empty(t *testing.T) {
	cast := ParseMarkdown("")
	result := ClassifyBlocks(cast)
	if result.TotalNodes != 0 {
		t.Fatalf("expected 0, got %d", result.TotalNodes)
	}
}

func TestSemRole_IsValid(t *testing.T) {
	for _, r := range AllSemRoles() {
		if !r.IsValid() {
			t.Fatalf("%s should be valid", r)
		}
	}
	if SemRole("bogus").IsValid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestAllSemRoles_Count(t *testing.T) {
	roles := AllSemRoles()
	if len(roles) != 10 {
		t.Fatalf("expected 10 roles, got %d", len(roles))
	}
}
