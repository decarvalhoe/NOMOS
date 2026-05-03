package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// TaxonLevel identifies the level of a taxon in the hierarchy.
type TaxonLevel string

const (
	LevelDomain  TaxonLevel = "domain"
	LevelSubject TaxonLevel = "subject"
	LevelConcept TaxonLevel = "concept"
)

// Taxon is a single node in the taxonomy tree.
type Taxon struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Level      TaxonLevel `json:"level"`
	ParentID   string     `json:"parent_id,omitempty"`
	Children   []string   `json:"children,omitempty"`
	Terms      []string   `json:"terms,omitempty"`
	TermCount  int        `json:"term_count"`
	Definition string     `json:"definition,omitempty"`
}

// Taxonomy is a hierarchical classification of domains, subjects, and concepts.
type Taxonomy struct {
	roots  []string
	byID   map[string]*Taxon
	byTerm map[string][]string // lowercase term → taxon IDs
}

// NewTaxonomy creates an empty taxonomy.
func NewTaxonomy() *Taxonomy {
	return &Taxonomy{
		byID:   map[string]*Taxon{},
		byTerm: map[string][]string{},
	}
}

// AddDomain adds a top-level domain taxon.
func (t *Taxonomy) AddDomain(id, label, definition string) error {
	return t.add(id, label, LevelDomain, "", definition)
}

// AddSubject adds a subject under a domain.
func (t *Taxonomy) AddSubject(id, label, parentID, definition string) error {
	if _, ok := t.byID[parentID]; !ok {
		return fmt.Errorf("parent %q not found", parentID)
	}
	return t.add(id, label, LevelSubject, parentID, definition)
}

// AddConcept adds a concept under a subject.
func (t *Taxonomy) AddConcept(id, label, parentID, definition string) error {
	if _, ok := t.byID[parentID]; !ok {
		return fmt.Errorf("parent %q not found", parentID)
	}
	return t.add(id, label, LevelConcept, parentID, definition)
}

func (t *Taxonomy) add(id, label string, level TaxonLevel, parentID, definition string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("taxon id is empty")
	}
	if _, exists := t.byID[id]; exists {
		return fmt.Errorf("taxon %q already exists", id)
	}

	taxon := &Taxon{
		ID:         id,
		Label:      label,
		Level:      level,
		ParentID:   parentID,
		Definition: definition,
	}
	t.byID[id] = taxon

	if parentID == "" {
		t.roots = append(t.roots, id)
	} else if parent, ok := t.byID[parentID]; ok {
		parent.Children = append(parent.Children, id)
	}
	return nil
}

// LinkTerm associates a lexicon term with a taxon.
func (t *Taxonomy) LinkTerm(taxonID, term string) error {
	taxon, ok := t.byID[taxonID]
	if !ok {
		return fmt.Errorf("taxon %q not found", taxonID)
	}
	lower := strings.ToLower(strings.TrimSpace(term))
	if lower == "" {
		return fmt.Errorf("term is empty")
	}
	// Avoid duplicates.
	for _, existing := range taxon.Terms {
		if strings.ToLower(existing) == lower {
			return nil
		}
	}
	taxon.Terms = append(taxon.Terms, term)
	taxon.TermCount = len(taxon.Terms)
	t.byTerm[lower] = append(t.byTerm[lower], taxonID)
	return nil
}

// Get returns a taxon by ID.
func (t *Taxonomy) Get(id string) (*Taxon, bool) {
	taxon, ok := t.byID[id]
	return taxon, ok
}

// Roots returns the top-level domain taxon IDs.
func (t *Taxonomy) Roots() []string {
	return t.roots
}

// Classify looks up which taxons a term belongs to.
func (t *Taxonomy) Classify(term string) []string {
	lower := strings.ToLower(strings.TrimSpace(term))
	return t.byTerm[lower]
}

// Ancestors returns the chain from root to the given taxon (exclusive).
func (t *Taxonomy) Ancestors(id string) []string {
	var chain []string
	seen := map[string]bool{}
	cur := id
	for {
		taxon, ok := t.byID[cur]
		if !ok || taxon.ParentID == "" || seen[cur] {
			break
		}
		seen[cur] = true
		chain = append([]string{taxon.ParentID}, chain...)
		cur = taxon.ParentID
	}
	return chain
}

// Descendants returns all descendant taxon IDs (BFS).
func (t *Taxonomy) Descendants(id string) []string {
	var result []string
	queue := []string{id}
	seen := map[string]bool{id: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		taxon, ok := t.byID[cur]
		if !ok {
			continue
		}
		for _, child := range taxon.Children {
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
				queue = append(queue, child)
			}
		}
	}
	return result
}

// AllTerms returns all terms linked to a taxon and its descendants.
func (t *Taxonomy) AllTerms(id string) []string {
	ids := append([]string{id}, t.Descendants(id)...)
	termSet := map[string]bool{}
	var terms []string
	for _, tid := range ids {
		taxon, ok := t.byID[tid]
		if !ok {
			continue
		}
		for _, term := range taxon.Terms {
			lower := strings.ToLower(term)
			if !termSet[lower] {
				termSet[lower] = true
				terms = append(terms, term)
			}
		}
	}
	sort.Strings(terms)
	return terms
}

// Size returns the total number of taxons.
func (t *Taxonomy) Size() int {
	return len(t.byID)
}

// Flat returns all taxons as a sorted slice.
func (t *Taxonomy) Flat() []Taxon {
	result := make([]Taxon, 0, len(t.byID))
	for _, taxon := range t.byID {
		result = append(result, *taxon)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// BuildFromLexicon constructs a taxonomy from a lexicon by grouping
// terms by their Domain field into a domain→concept hierarchy.
func BuildFromLexicon(lex *Lexicon) *Taxonomy {
	tax := NewTaxonomy()
	if lex == nil {
		return tax
	}

	domainSet := map[string]bool{}
	terms := lex.AllTerms()

	// First pass: create domains.
	for _, term := range terms {
		domain := term.Domain
		if domain == "" {
			domain = "general"
		}
		domainID := "dom-" + slugID(domain)
		if !domainSet[domainID] {
			tax.AddDomain(domainID, domain, "")
			domainSet[domainID] = true
		}
	}

	// Second pass: create concepts and link terms.
	for _, term := range terms {
		domain := term.Domain
		if domain == "" {
			domain = "general"
		}
		domainID := "dom-" + slugID(domain)
		conceptID := "cpt-" + slugID(term.Canonical)
		if _, exists := tax.byID[conceptID]; !exists {
			tax.AddConcept(conceptID, term.Canonical, domainID, term.Definition)
		}
		tax.LinkTerm(conceptID, term.Canonical)
		for _, syn := range term.Synonyms {
			tax.LinkTerm(conceptID, syn)
		}
	}

	return tax
}

func slugID(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
