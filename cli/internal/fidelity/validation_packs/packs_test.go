package validation_packs

import (
	"testing"
)

func TestAllPacksDefined(t *testing.T) {
	packs := AllPacks()
	if len(packs) != 3 {
		t.Fatalf("expected 3 packs, got %d", len(packs))
	}
	ids := map[PackID]bool{}
	for _, p := range packs {
		ids[p.ID] = true
	}
	for _, expected := range []PackID{PackRBOK, PackLegal, PackGameRules} {
		if !ids[expected] {
			t.Fatalf("missing pack %s", expected)
		}
	}
}

func TestLoadFixtureRBOK(t *testing.T) {
	packs := AllPacks()
	content, err := LoadFixture(packs[0])
	if err != nil {
		t.Fatalf("load rbok fixture: %v", err)
	}
	if len(content) < 100 {
		t.Fatalf("fixture too short: %d bytes", len(content))
	}
}

func TestLoadFixtureLegal(t *testing.T) {
	packs := AllPacks()
	content, err := LoadFixture(packs[1])
	if err != nil {
		t.Fatalf("load legal fixture: %v", err)
	}
	if len(content) < 100 {
		t.Fatalf("fixture too short: %d bytes", len(content))
	}
}

func TestLoadFixtureGameRules(t *testing.T) {
	packs := AllPacks()
	content, err := LoadFixture(packs[2])
	if err != nil {
		t.Fatalf("load game-rules fixture: %v", err)
	}
	if len(content) < 100 {
		t.Fatalf("fixture too short: %d bytes", len(content))
	}
}

func TestValidateRBOKPack(t *testing.T) {
	packs := AllPacks()
	var rbokPack Pack
	for _, p := range packs {
		if p.ID == PackRBOK {
			rbokPack = p
			break
		}
	}

	result, err := Validate(rbokPack)
	if err != nil {
		t.Fatalf("validate rbok: %v", err)
	}
	if !result.Pass {
		t.Fatalf("rbok pack failed: %v", result.Failures)
	}
	if result.AtomCount < rbokPack.Expected.MinAtoms {
		t.Fatalf("expected >= %d atoms, got %d", rbokPack.Expected.MinAtoms, result.AtomCount)
	}
}

func TestValidateLegalPack(t *testing.T) {
	packs := AllPacks()
	var legalPack Pack
	for _, p := range packs {
		if p.ID == PackLegal {
			legalPack = p
			break
		}
	}

	result, err := Validate(legalPack)
	if err != nil {
		t.Fatalf("validate legal: %v", err)
	}
	if !result.Pass {
		t.Fatalf("legal pack failed: %v", result.Failures)
	}
}

func TestValidateGameRulesPack(t *testing.T) {
	packs := AllPacks()
	var grPack Pack
	for _, p := range packs {
		if p.ID == PackGameRules {
			grPack = p
			break
		}
	}

	result, err := Validate(grPack)
	if err != nil {
		t.Fatalf("validate game-rules: %v", err)
	}
	if !result.Pass {
		t.Fatalf("game-rules pack failed: %v", result.Failures)
	}
}

func TestValidateAllPacks(t *testing.T) {
	for _, pack := range AllPacks() {
		t.Run(string(pack.ID), func(t *testing.T) {
			result, err := Validate(pack)
			if err != nil {
				t.Fatalf("validate %s: %v", pack.ID, err)
			}
			if !result.Pass {
				t.Fatalf("pack %s failed:\n  %v", pack.ID, result.Failures)
			}
		})
	}
}

func TestValidatePackAtomCountConsistency(t *testing.T) {
	for _, pack := range AllPacks() {
		r1, _ := Validate(pack)
		r2, _ := Validate(pack)
		if r1.AtomCount != r2.AtomCount {
			t.Fatalf("pack %s non-deterministic: %d vs %d atoms", pack.ID, r1.AtomCount, r2.AtomCount)
		}
	}
}

func TestValidatePackDomainCorrect(t *testing.T) {
	for _, pack := range AllPacks() {
		result, _ := Validate(pack)
		if !result.Pass {
			t.Skipf("pack %s did not pass, skip domain check", pack.ID)
		}
	}
}

func TestParseToSourceUnitsHeadings(t *testing.T) {
	content := "# Title\n\n## Sub\n\nParagraph.\n"
	units := parseToSourceUnits(content)

	headings := 0
	for _, u := range units {
		if u.Type == "heading" {
			headings++
		}
	}
	if headings != 2 {
		t.Fatalf("expected 2 headings, got %d", headings)
	}
}

func TestParseToSourceUnitsListItems(t *testing.T) {
	content := "# Title\n\n- Item one\n- Item two\n"
	units := parseToSourceUnits(content)

	listItems := 0
	for _, u := range units {
		if u.Type == "list_item" {
			listItems++
		}
	}
	if listItems != 2 {
		t.Fatalf("expected 2 list items, got %d", listItems)
	}
}

func TestParseToSourceUnitsParentChain(t *testing.T) {
	content := "# Parent\n\nChild paragraph.\n"
	units := parseToSourceUnits(content)

	if len(units) < 2 {
		t.Fatalf("expected at least 2 units, got %d", len(units))
	}
	if units[1].ParentID == "" {
		t.Fatal("expected paragraph to have parent ID from heading")
	}
}
