package corpus

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CorpusDiff represents a set of changed files in the corpus.
type CorpusDiff struct {
	Added    []string `json:"added,omitempty"`
	Modified []string `json:"modified,omitempty"`
	Removed  []string `json:"removed,omitempty"`
}

// AllPaths returns all changed paths (added + modified + removed).
func (d CorpusDiff) AllPaths() []string {
	seen := map[string]struct{}{}
	var all []string
	for _, p := range d.Added {
		if _, ok := seen[p]; !ok {
			all = append(all, p)
			seen[p] = struct{}{}
		}
	}
	for _, p := range d.Modified {
		if _, ok := seen[p]; !ok {
			all = append(all, p)
			seen[p] = struct{}{}
		}
	}
	for _, p := range d.Removed {
		if _, ok := seen[p]; !ok {
			all = append(all, p)
			seen[p] = struct{}{}
		}
	}
	return all
}

// ImpactSkeleton is the result of impact analysis: which units and
// chunks are affected by corpus changes.
type ImpactSkeleton struct {
	AffectedUnits  []AffectedUnit  `json:"affected_units"`
	AffectedChunks []AffectedChunk `json:"affected_chunks,omitempty"`
	Summary        ImpactSummary   `json:"summary"`
}

// AffectedUnit describes a matrix unit impacted by a change.
type AffectedUnit struct {
	UnitID      string   `json:"unit_id"`
	Name        string   `json:"name,omitempty"`
	Criticality string   `json:"criticality,omitempty"`
	Reason      string   `json:"reason"`
	ChangedRefs []string `json:"changed_refs"`
}

// AffectedChunk describes a knowledge-base chunk impacted by a change.
type AffectedChunk struct {
	Collection string `json:"collection"`
	Filter     string `json:"filter"`
	UnitID     string `json:"unit_id"`
	Reason     string `json:"reason"`
}

// ImpactSummary provides counts for the impact skeleton.
type ImpactSummary struct {
	TotalChanged   int `json:"total_changed"`
	UnitsAffected  int `json:"units_affected"`
	ChunksAffected int `json:"chunks_affected"`
	CriticalUnits  int `json:"critical_units"`
}

// MatrixUnit is a minimal representation of a canonical matrix unit
// for impact analysis purposes.
type MatrixUnit struct {
	UnitID      string       `yaml:"unit_id" json:"unit_id"`
	Name        string       `yaml:"name" json:"name"`
	Criticality string       `yaml:"criticality" json:"criticality"`
	SourceRefs  []SourceRef  `yaml:"source_refs" json:"source_refs"`
	Contract    *ContractRef `yaml:"canonical_contract" json:"canonical_contract,omitempty"`
	SchemaRefs  []string     `yaml:"schema_refs" json:"schema_refs,omitempty"`
	DBRefs      []DBRef      `yaml:"db_refs" json:"db_refs,omitempty"`
	VectorRefs  []VectorRef  `yaml:"vector_refs" json:"vector_refs,omitempty"`
	CoreRefs    []CoreRef    `yaml:"core_refs" json:"core_refs,omitempty"`
	APIRefs     []APIRef     `yaml:"api_refs" json:"api_refs,omitempty"`
	UIRefs      []UIRef      `yaml:"ui_refs" json:"ui_refs,omitempty"`
	TestRefs    []string     `yaml:"test_refs" json:"test_refs,omitempty"`
}

type SourceRef struct {
	SourceID string `yaml:"source_id" json:"source_id"`
	Locator  string `yaml:"locator" json:"locator,omitempty"`
}

type ContractRef struct {
	Path     string `yaml:"path" json:"path"`
	ObjectID string `yaml:"object_id" json:"object_id,omitempty"`
}

type DBRef struct {
	Table string `yaml:"table" json:"table"`
	Key   string `yaml:"key" json:"key,omitempty"`
}

type VectorRef struct {
	Collection string `yaml:"collection" json:"collection"`
	Filter     string `yaml:"filter" json:"filter,omitempty"`
}

type CoreRef struct {
	Module string `yaml:"module" json:"module"`
	Symbol string `yaml:"symbol" json:"symbol,omitempty"`
}

type APIRef struct {
	Method string `yaml:"method" json:"method,omitempty"`
	Path   string `yaml:"path" json:"path"`
}

type UIRef struct {
	App  string `yaml:"app" json:"app,omitempty"`
	Path string `yaml:"path" json:"path"`
}

// CanonicalMatrix is the top-level structure for a matrix YAML file.
type CanonicalMatrix struct {
	SchemaVersion string       `yaml:"schema_version"`
	Units         []MatrixUnit `yaml:"units"`
}

// ParseMatrix parses a canonical matrix from YAML bytes.
func ParseMatrix(data []byte) (CanonicalMatrix, error) {
	var m CanonicalMatrix
	if err := yaml.Unmarshal(data, &m); err != nil {
		return CanonicalMatrix{}, fmt.Errorf("parsing matrix: %w", err)
	}
	return m, nil
}

// ComputeImpact determines which units and chunks are affected by a corpus diff.
func ComputeImpact(diff CorpusDiff, matrix CanonicalMatrix) ImpactSkeleton {
	changedPaths := diff.AllPaths()
	if len(changedPaths) == 0 {
		return ImpactSkeleton{
			AffectedUnits:  []AffectedUnit{},
			AffectedChunks: []AffectedChunk{},
			Summary:        ImpactSummary{},
		}
	}

	var affectedUnits []AffectedUnit
	var affectedChunks []AffectedChunk
	criticalCount := 0

	for _, unit := range matrix.Units {
		refs := matchedRefs(unit, changedPaths)
		if len(refs) == 0 {
			continue
		}

		reason := fmt.Sprintf("Changed files match %d reference(s) in unit %s.", len(refs), unit.UnitID)
		affectedUnits = append(affectedUnits, AffectedUnit{
			UnitID:      unit.UnitID,
			Name:        unit.Name,
			Criticality: unit.Criticality,
			Reason:      reason,
			ChangedRefs: refs,
		})

		if unit.Criticality == "high" || unit.Criticality == "critical" {
			criticalCount++
		}

		for _, vref := range unit.VectorRefs {
			affectedChunks = append(affectedChunks, AffectedChunk{
				Collection: vref.Collection,
				Filter:     vref.Filter,
				UnitID:     unit.UnitID,
				Reason:     fmt.Sprintf("Unit %s is affected; chunks in %s may be stale.", unit.UnitID, vref.Collection),
			})
		}
	}

	sort.Slice(affectedUnits, func(i, j int) bool {
		return affectedUnits[i].UnitID < affectedUnits[j].UnitID
	})
	sort.Slice(affectedChunks, func(i, j int) bool {
		if affectedChunks[i].UnitID != affectedChunks[j].UnitID {
			return affectedChunks[i].UnitID < affectedChunks[j].UnitID
		}
		return affectedChunks[i].Collection < affectedChunks[j].Collection
	})

	return ImpactSkeleton{
		AffectedUnits:  affectedUnits,
		AffectedChunks: affectedChunks,
		Summary: ImpactSummary{
			TotalChanged:   len(changedPaths),
			UnitsAffected:  len(affectedUnits),
			ChunksAffected: len(affectedChunks),
			CriticalUnits:  criticalCount,
		},
	}
}

// matchedRefs returns which reference paths in a unit match the changed paths.
func matchedRefs(unit MatrixUnit, changedPaths []string) []string {
	unitPaths := collectUnitPaths(unit)
	var matched []string
	seen := map[string]struct{}{}

	for _, changed := range changedPaths {
		for _, ref := range unitPaths {
			if pathMatches(changed, ref) {
				if _, ok := seen[changed]; !ok {
					matched = append(matched, changed)
					seen[changed] = struct{}{}
				}
			}
		}
	}
	sort.Strings(matched)
	return matched
}

// collectUnitPaths gathers all file paths referenced by a unit.
func collectUnitPaths(unit MatrixUnit) []string {
	var paths []string
	if unit.Contract != nil && unit.Contract.Path != "" {
		paths = append(paths, unit.Contract.Path)
	}
	paths = append(paths, unit.SchemaRefs...)
	for _, ref := range unit.CoreRefs {
		paths = append(paths, ref.Module)
	}
	paths = append(paths, unit.TestRefs...)
	return paths
}

// pathMatches checks if a changed path is equal to or a child of a ref path.
func pathMatches(changed, ref string) bool {
	changed = strings.TrimPrefix(changed, "/")
	ref = strings.TrimPrefix(ref, "/")
	if changed == ref {
		return true
	}
	// Check if changed is under ref (ref is a directory prefix).
	if strings.HasSuffix(ref, "/") && strings.HasPrefix(changed, ref) {
		return true
	}
	// Check if ref is a prefix directory of changed.
	if strings.HasPrefix(changed, ref+"/") {
		return true
	}
	return false
}
