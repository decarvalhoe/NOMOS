package corpus

import (
	"testing"
)

var testMatrix = CanonicalMatrix{
	SchemaVersion: "0.1.0",
	Units: []MatrixUnit{
		{
			UnitID:      "UNIT-WATER-DAMAGE",
			Name:        "Water damage warranty",
			Criticality: "high",
			SourceRefs: []SourceRef{
				{SourceID: "SRC-CONTRACT-2026", Locator: "p.12"},
			},
			Contract: &ContractRef{
				Path:     "data/canonical/warranties.yaml",
				ObjectID: "WATER-DAMAGE",
			},
			SchemaRefs: []string{"schemas/warranty.schema.json"},
			CoreRefs:   []CoreRef{{Module: "core/warranties/water_damage.go", Symbol: "Evaluate"}},
			TestRefs:   []string{"tests/golden/water-damage.yaml", "tests/core/water_damage_test.go"},
			VectorRefs: []VectorRef{{Collection: "canonical_chunks", Filter: "unit_id = UNIT-WATER-DAMAGE"}},
		},
		{
			UnitID:      "UNIT-ROOF-EXCLUSION",
			Name:        "Roof infiltration exclusion",
			Criticality: "medium",
			Contract: &ContractRef{
				Path:     "data/canonical/exclusions.yaml",
				ObjectID: "ROOF-EXCLUSION",
			},
			CoreRefs: []CoreRef{{Module: "core/exclusions/roof.go"}},
			TestRefs: []string{"tests/core/roof_test.go"},
		},
		{
			UnitID:      "UNIT-DEDUCTIBLE",
			Name:        "Deductible calculation",
			Criticality: "low",
			CoreRefs:    []CoreRef{{Module: "core/deductibles/calc.go"}},
			TestRefs:    []string{"tests/core/deductible_test.go"},
		},
	},
}

func TestComputeImpactSingleUnit(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{"data/canonical/warranties.yaml"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 1 {
		t.Fatalf("expected 1 affected unit, got %d", skeleton.Summary.UnitsAffected)
	}
	if skeleton.AffectedUnits[0].UnitID != "UNIT-WATER-DAMAGE" {
		t.Fatalf("expected UNIT-WATER-DAMAGE, got %s", skeleton.AffectedUnits[0].UnitID)
	}
	if skeleton.Summary.CriticalUnits != 1 {
		t.Fatalf("expected 1 critical unit, got %d", skeleton.Summary.CriticalUnits)
	}
}

func TestComputeImpactMultipleUnits(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{"core/warranties/water_damage.go", "core/exclusions/roof.go"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 2 {
		t.Fatalf("expected 2 affected units, got %d", skeleton.Summary.UnitsAffected)
	}
	ids := map[string]bool{}
	for _, u := range skeleton.AffectedUnits {
		ids[u.UnitID] = true
	}
	if !ids["UNIT-WATER-DAMAGE"] || !ids["UNIT-ROOF-EXCLUSION"] {
		t.Fatalf("expected both units affected, got %v", skeleton.AffectedUnits)
	}
}

func TestComputeImpactChunks(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{"data/canonical/warranties.yaml"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.ChunksAffected != 1 {
		t.Fatalf("expected 1 affected chunk, got %d", skeleton.Summary.ChunksAffected)
	}
	chunk := skeleton.AffectedChunks[0]
	if chunk.Collection != "canonical_chunks" {
		t.Fatalf("expected canonical_chunks, got %s", chunk.Collection)
	}
	if chunk.UnitID != "UNIT-WATER-DAMAGE" {
		t.Fatalf("expected chunk linked to UNIT-WATER-DAMAGE, got %s", chunk.UnitID)
	}
}

func TestComputeImpactNoMatch(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{"unrelated/file.txt"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 0 {
		t.Fatalf("expected 0 affected units, got %d", skeleton.Summary.UnitsAffected)
	}
	if len(skeleton.AffectedUnits) != 0 {
		t.Fatalf("expected empty affected units, got %v", skeleton.AffectedUnits)
	}
}

func TestComputeImpactEmptyDiff(t *testing.T) {
	diff := CorpusDiff{}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.TotalChanged != 0 {
		t.Fatalf("expected 0 total changed, got %d", skeleton.Summary.TotalChanged)
	}
	if skeleton.Summary.UnitsAffected != 0 {
		t.Fatalf("expected 0 units affected, got %d", skeleton.Summary.UnitsAffected)
	}
}

func TestComputeImpactTestRefMatch(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{"tests/core/water_damage_test.go"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 1 {
		t.Fatalf("expected 1 affected unit from test ref, got %d", skeleton.Summary.UnitsAffected)
	}
	if skeleton.AffectedUnits[0].UnitID != "UNIT-WATER-DAMAGE" {
		t.Fatalf("expected UNIT-WATER-DAMAGE, got %s", skeleton.AffectedUnits[0].UnitID)
	}
}

func TestComputeImpactSchemaRefMatch(t *testing.T) {
	diff := CorpusDiff{
		Added: []string{"schemas/warranty.schema.json"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 1 {
		t.Fatalf("expected 1 affected unit from schema ref, got %d", skeleton.Summary.UnitsAffected)
	}
}

func TestComputeImpactRemovedFile(t *testing.T) {
	diff := CorpusDiff{
		Removed: []string{"core/deductibles/calc.go"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 1 {
		t.Fatalf("expected 1 affected unit, got %d", skeleton.Summary.UnitsAffected)
	}
	if skeleton.AffectedUnits[0].UnitID != "UNIT-DEDUCTIBLE" {
		t.Fatalf("expected UNIT-DEDUCTIBLE, got %s", skeleton.AffectedUnits[0].UnitID)
	}
}

func TestComputeImpactChangedRefsPopulated(t *testing.T) {
	diff := CorpusDiff{
		Modified: []string{
			"data/canonical/warranties.yaml",
			"core/warranties/water_damage.go",
		},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.UnitsAffected != 1 {
		t.Fatalf("expected 1 unit, got %d", skeleton.Summary.UnitsAffected)
	}
	unit := skeleton.AffectedUnits[0]
	if len(unit.ChangedRefs) != 2 {
		t.Fatalf("expected 2 changed refs, got %d: %v", len(unit.ChangedRefs), unit.ChangedRefs)
	}
}

func TestComputeImpactSummaryTotalChanged(t *testing.T) {
	diff := CorpusDiff{
		Added:    []string{"new.go"},
		Modified: []string{"core/warranties/water_damage.go"},
		Removed:  []string{"old.go"},
	}

	skeleton := ComputeImpact(diff, testMatrix)

	if skeleton.Summary.TotalChanged != 3 {
		t.Fatalf("expected 3 total changed, got %d", skeleton.Summary.TotalChanged)
	}
}

func TestParseMatrix(t *testing.T) {
	data := []byte(`
schema_version: "0.1.0"
units:
  - unit_id: UNIT-001
    name: Test unit
    criticality: high
    canonical_contract:
      path: data/contracts/test.yaml
      object_id: OBJ-001
    core_refs:
      - module: src/test.go
        symbol: TestFunc
    test_refs:
      - tests/test_test.go
`)
	m, err := ParseMatrix(data)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(m.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(m.Units))
	}
	if m.Units[0].UnitID != "UNIT-001" {
		t.Fatalf("expected UNIT-001, got %s", m.Units[0].UnitID)
	}
	if m.Units[0].Contract.Path != "data/contracts/test.yaml" {
		t.Fatalf("expected contract path, got %s", m.Units[0].Contract.Path)
	}
}

func TestParseMatrixInvalid(t *testing.T) {
	_, err := ParseMatrix([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestCorpusDiffAllPaths(t *testing.T) {
	diff := CorpusDiff{
		Added:    []string{"a.go"},
		Modified: []string{"b.go", "a.go"}, // duplicate with added
		Removed:  []string{"c.go"},
	}

	paths := diff.AllPaths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 unique paths, got %d: %v", len(paths), paths)
	}
}
