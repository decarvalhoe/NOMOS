package atomization

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// GoldAnnotation is a reference annotation for validating atomization output.
type GoldAnnotation struct {
	ID            string          `json:"id"`
	Corpus        string          `json:"corpus"`
	SourceFile    string          `json:"source_file"`
	SourceSpan    GoldSourceSpan  `json:"source_span"`
	ExpectedAtoms []ExpectedAtom  `json:"expected_atoms"`
	ExpectedRefs  []ExpectedRef   `json:"expected_refs"`
	Notes         string          `json:"notes,omitempty"`
}

// GoldSourceSpan identifies the annotated region in the source.
type GoldSourceSpan struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Locator   string `json:"locator,omitempty"`
	Text      string `json:"text,omitempty"`
}

// ExpectedAtom describes an atom that the pipeline should extract.
type ExpectedAtom struct {
	AtomID   string `json:"atom_id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Priority string `json:"priority,omitempty"`
	Content  string `json:"content,omitempty"`
}

// ExpectedRef describes a reference the pipeline should detect.
type ExpectedRef struct {
	RefType  string `json:"ref_type"`
	TargetID string `json:"target_id"`
	FromAtom string `json:"from_atom"`
}

// GoldCorpus is a collection of annotations for a specific corpus type.
type GoldCorpus struct {
	SchemaVersion string           `json:"schema_version"`
	Corpus        string           `json:"corpus"`
	Description   string           `json:"description"`
	Annotations   []GoldAnnotation `json:"annotations"`
}

// RegressionResult is the output of running regression against gold annotations.
type RegressionResult struct {
	Corpus       string             `json:"corpus"`
	TotalGold    int                `json:"total_gold"`
	Matched      int                `json:"matched"`
	Missing      int                `json:"missing"`
	Extra        int                `json:"extra"`
	Score        float64            `json:"score"`
	Details      []RegressionDetail `json:"details"`
}

// RegressionDetail is a per-annotation comparison result.
type RegressionDetail struct {
	AnnotationID  string   `json:"annotation_id"`
	Status        string   `json:"status"` // "matched", "partial", "missing"
	MatchedAtoms  []string `json:"matched_atoms,omitempty"`
	MissingAtoms  []string `json:"missing_atoms,omitempty"`
	ExtraAtoms    []string `json:"extra_atoms,omitempty"`
	MatchedRefs   []string `json:"matched_refs,omitempty"`
	MissingRefs   []string `json:"missing_refs,omitempty"`
}

// ProducedAtom represents an atom produced by the pipeline for comparison.
type ProducedAtom struct {
	AtomID string
	Kind   string
	Title  string
}

// ProducedRef represents a reference produced by the pipeline for comparison.
type ProducedRef struct {
	RefType  string
	TargetID string
	FromAtom string
}

// LoadGoldCorpus reads a gold annotation file.
func LoadGoldCorpus(path string) (GoldCorpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoldCorpus{}, fmt.Errorf("reading gold corpus: %w", err)
	}
	return ParseGoldCorpus(data)
}

// ParseGoldCorpus parses gold annotation JSON.
func ParseGoldCorpus(data []byte) (GoldCorpus, error) {
	var corpus GoldCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return GoldCorpus{}, fmt.Errorf("parsing gold corpus: %w", err)
	}
	return corpus, nil
}

// RunRegression compares produced atoms/refs against gold annotations.
func RunRegression(gold GoldCorpus, produced []ProducedAtom, producedRefs []ProducedRef) RegressionResult {
	result := RegressionResult{
		Corpus:    gold.Corpus,
		TotalGold: len(gold.Annotations),
	}

	producedByID := make(map[string]ProducedAtom, len(produced))
	for _, a := range produced {
		producedByID[a.AtomID] = a
	}

	producedRefSet := make(map[string]bool, len(producedRefs))
	for _, r := range producedRefs {
		producedRefSet[refKey(r.FromAtom, r.RefType, r.TargetID)] = true
	}

	allExpectedAtomIDs := map[string]bool{}
	allMatchedAtomIDs := map[string]bool{}

	for _, ann := range gold.Annotations {
		detail := RegressionDetail{AnnotationID: ann.ID}

		for _, expected := range ann.ExpectedAtoms {
			allExpectedAtomIDs[expected.AtomID] = true
			if p, ok := producedByID[expected.AtomID]; ok {
				if matchesAtom(expected, p) {
					detail.MatchedAtoms = append(detail.MatchedAtoms, expected.AtomID)
					allMatchedAtomIDs[expected.AtomID] = true
				} else {
					detail.MissingAtoms = append(detail.MissingAtoms, expected.AtomID)
				}
			} else {
				detail.MissingAtoms = append(detail.MissingAtoms, expected.AtomID)
			}
		}

		for _, expected := range ann.ExpectedRefs {
			key := refKey(expected.FromAtom, expected.RefType, expected.TargetID)
			if producedRefSet[key] {
				detail.MatchedRefs = append(detail.MatchedRefs, key)
			} else {
				detail.MissingRefs = append(detail.MissingRefs, key)
			}
		}

		switch {
		case len(detail.MissingAtoms) == 0 && len(detail.MissingRefs) == 0:
			detail.Status = "matched"
			result.Matched++
		case len(detail.MatchedAtoms) > 0 || len(detail.MatchedRefs) > 0:
			detail.Status = "partial"
			result.Matched++ // count partial as half in score
		default:
			detail.Status = "missing"
			result.Missing++
		}

		result.Details = append(result.Details, detail)
	}

	// Count extra atoms not in any gold annotation.
	for id := range producedByID {
		if !allExpectedAtomIDs[id] {
			result.Extra++
		}
	}

	if result.TotalGold > 0 {
		result.Score = float64(result.Matched) / float64(result.TotalGold)
	}

	return result
}

// ValidateGoldCorpus checks a gold corpus for structural validity.
func ValidateGoldCorpus(corpus GoldCorpus) []string {
	var errs []string

	if strings.TrimSpace(corpus.Corpus) == "" {
		errs = append(errs, "corpus name is required")
	}
	if len(corpus.Annotations) == 0 {
		errs = append(errs, "at least one annotation is required")
	}

	ids := map[string]bool{}
	for i, ann := range corpus.Annotations {
		if strings.TrimSpace(ann.ID) == "" {
			errs = append(errs, fmt.Sprintf("annotations[%d].id is required", i))
		} else if ids[ann.ID] {
			errs = append(errs, fmt.Sprintf("annotations[%d].id %q is duplicated", i, ann.ID))
		} else {
			ids[ann.ID] = true
		}
		if strings.TrimSpace(ann.SourceFile) == "" {
			errs = append(errs, fmt.Sprintf("annotations[%d].source_file is required", i))
		}
		if len(ann.ExpectedAtoms) == 0 {
			errs = append(errs, fmt.Sprintf("annotations[%d].expected_atoms must not be empty", i))
		}
		for j, atom := range ann.ExpectedAtoms {
			if strings.TrimSpace(atom.AtomID) == "" {
				errs = append(errs, fmt.Sprintf("annotations[%d].expected_atoms[%d].atom_id is required", i, j))
			}
			if strings.TrimSpace(atom.Kind) == "" {
				errs = append(errs, fmt.Sprintf("annotations[%d].expected_atoms[%d].kind is required", i, j))
			}
		}
	}

	return errs
}

func matchesAtom(expected ExpectedAtom, produced ProducedAtom) bool {
	if expected.Kind != "" && !strings.EqualFold(expected.Kind, produced.Kind) {
		return false
	}
	return true
}

func refKey(fromAtom, refType, targetID string) string {
	return fromAtom + "|" + refType + "|" + targetID
}

// SortDetails sorts regression details by annotation ID.
func SortDetails(details []RegressionDetail) {
	sort.Slice(details, func(i, j int) bool {
		return details[i].AnnotationID < details[j].AnnotationID
	})
}
