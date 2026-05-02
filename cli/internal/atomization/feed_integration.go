package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// FeedIntegrationConfig controls how atoms are projected into feeds.
type FeedIntegrationConfig struct {
	Domain     string
	Owner      string
	SourcePath string
	SourceHash string
	Version    string
	// MinReviewState is the minimum review state for inclusion.
	// Default (empty) requires ReviewApproved.
	MinReviewState ReviewState
}

func (c FeedIntegrationConfig) minState() ReviewState {
	if c.MinReviewState != "" {
		return c.MinReviewState
	}
	return ReviewApproved
}

// LawbookFeedEntry matches the corpus.LawbookNode structure
// without importing the corpus package (avoids circular deps).
type LawbookFeedEntry struct {
	NodeID       string         `json:"node_id"`
	DocumentID   string         `json:"document_id"`
	NodeType     string         `json:"node_type"`
	CanonicalRef string         `json:"canonical_ref"`
	DisplayRef   string         `json:"display_ref"`
	Depth        int            `json:"depth"`
	OrdinalPath  string         `json:"ordinal_path"`
	SourcePath   string         `json:"source_path"`
	SourceHash   string         `json:"source_hash"`
	Status       string         `json:"status"`
	Priority     string         `json:"priority"`
	Domain       string         `json:"domain"`
	Title        string         `json:"title,omitempty"`
	Text         string         `json:"text,omitempty"`
	ParentID     string         `json:"parent_id,omitempty"`
	ReviewState  string         `json:"review_state"`
	AtomID       string         `json:"atom_id"`
}

// LawbookFeedResult is the output of atom-to-lawbook-feed projection.
type LawbookFeedResult struct {
	SchemaVersion  string             `json:"schema_version"`
	FeedID         string             `json:"feed_id"`
	Domain         string             `json:"domain"`
	GeneratedAt    string             `json:"generated_at"`
	SourcePath     string             `json:"source_path"`
	SourceHash     string             `json:"source_hash"`
	TotalAtoms     int                `json:"total_atoms"`
	CertifiedAtoms int                `json:"certified_atoms"`
	RejectedAtoms  int                `json:"rejected_atoms"`
	Entries        []LawbookFeedEntry `json:"entries"`
}

// EngineImportResult is the output of atom-to-engine projection.
type EngineImportResult struct {
	ContractVersion string            `json:"contract_version"`
	GeneratedAt     string            `json:"generated_at"`
	Domain          string            `json:"domain"`
	TotalAtoms      int               `json:"total_atoms"`
	CertifiedAtoms  int               `json:"certified_atoms"`
	Nodes           []NodeProjection  `json:"nodes"`
}

// ProjectAtomsToLawbookFeed converts certified atoms into lawbook feed entries.
func ProjectAtomsToLawbookFeed(atoms []Atom, config FeedIntegrationConfig) LawbookFeedResult {
	now := time.Now().UTC()
	certified, rejected := filterCertified(atoms, config.minState())

	docID := makeFeedDocID(config.SourcePath)
	entries := make([]LawbookFeedEntry, 0, len(certified))

	for i, atom := range certified {
		entries = append(entries, LawbookFeedEntry{
			NodeID:       atomToNodeID(atom),
			DocumentID:   docID,
			NodeType:     mapAtomTypeToNodeType(atom.Type),
			CanonicalRef: atom.CanonicalRef,
			DisplayRef:   atomDisplayRef(atom),
			Depth:        atom.Depth,
			OrdinalPath:  fmt.Sprintf("%d", i+1),
			SourcePath:   config.SourcePath,
			SourceHash:   atom.ContentHash,
			Status:       "active",
			Priority:     atomPriority(atom),
			Domain:       firstNonEmpty(atom.Domain, config.Domain),
			Title:        atom.Title,
			Text:         atom.Text,
			ParentID:     atom.ParentID,
			ReviewState:  string(atom.ReviewState),
			AtomID:       atom.ID,
		})
	}

	feedID := makeFeedID(config.Domain, config.SourcePath)

	return LawbookFeedResult{
		SchemaVersion:  "0.1.0",
		FeedID:         feedID,
		Domain:         config.Domain,
		GeneratedAt:    now.Format(time.RFC3339),
		SourcePath:     config.SourcePath,
		SourceHash:     config.SourceHash,
		TotalAtoms:     len(atoms),
		CertifiedAtoms: len(certified),
		RejectedAtoms:  len(rejected),
		Entries:        entries,
	}
}

// ProjectAtomsToEngineImport converts certified atoms into engine import nodes.
func ProjectAtomsToEngineImport(atoms []Atom, config FeedIntegrationConfig) EngineImportResult {
	now := time.Now().UTC()
	certified, _ := filterCertified(atoms, config.minState())

	nodes := make([]NodeProjection, 0, len(certified))
	docID := makeFeedDocID(config.SourcePath)

	for _, atom := range certified {
		nodes = append(nodes, NodeProjection{
			ExternalID:         atom.ID,
			DocumentExternalID: docID,
			ParentExternalID:   atom.ParentID,
			NodeType:           mapAtomTypeToNodeType(atom.Type),
			DisplayRef:         atomDisplayRef(atom),
			CanonicalRef:       atom.CanonicalRef,
			Depth:              atom.Depth,
			StructureOnly:      false,
			Priority:           atomPriorityInt(atom),
			Status:             "active",
			SourcePath:         config.SourcePath,
			SourceHash:         atom.ContentHash,
			Content:            atom.Text,
		})
	}

	return EngineImportResult{
		ContractVersion: EngineProfileVersion,
		GeneratedAt:     now.Format(time.RFC3339),
		Domain:          config.Domain,
		TotalAtoms:      len(atoms),
		CertifiedAtoms:  len(certified),
		Nodes:           nodes,
	}
}

// IsCertified returns true if the atom meets the minimum review state.
func IsCertified(atom Atom, minState ReviewState) bool {
	return reviewRank(atom.ReviewState) >= reviewRank(minState)
}

func filterCertified(atoms []Atom, minState ReviewState) (certified, rejected []Atom) {
	for _, a := range atoms {
		if IsCertified(a, minState) {
			certified = append(certified, a)
		} else {
			rejected = append(rejected, a)
		}
	}
	return
}

func reviewRank(state ReviewState) int {
	switch state {
	case ReviewRejected:
		return 0
	case ReviewDraft:
		return 1
	case ReviewPending:
		return 2
	case ReviewAmended:
		return 3
	case ReviewApproved:
		return 4
	default:
		return -1
	}
}

func atomToNodeID(atom Atom) string {
	h := sha256.Sum256([]byte(atom.ID + ":" + atom.CanonicalRef))
	return "N-" + strings.ToUpper(hex.EncodeToString(h[:8]))
}

func mapAtomTypeToNodeType(t AtomType) string {
	switch t {
	case AtomRule:
		return "article"
	case AtomClause:
		return "article"
	case AtomDefinition:
		return "definition"
	case AtomListItem:
		return "article"
	case AtomTable:
		return "annex"
	case AtomCodeBlock:
		return "annex"
	case AtomMeta:
		return "definition"
	default:
		return "article"
	}
}

func atomDisplayRef(atom Atom) string {
	if atom.Title != "" {
		return fmt.Sprintf("%s: %s", atom.Type, atom.Title)
	}
	return fmt.Sprintf("%s@%s", atom.Type, atom.SourceSpan.String())
}

func atomPriority(atom Atom) string {
	switch atom.Type {
	case AtomRule, AtomClause:
		return "high"
	case AtomDefinition:
		return "medium"
	default:
		return "low"
	}
}

func atomPriorityInt(atom Atom) int {
	switch atom.Type {
	case AtomRule, AtomClause:
		return 80
	case AtomDefinition:
		return 60
	case AtomListItem:
		return 40
	default:
		return 20
	}
}

func makeFeedDocID(sourcePath string) string {
	h := sha256.Sum256([]byte(sourcePath))
	return "feed-doc-" + hex.EncodeToString(h[:8])
}

func makeFeedID(domain, sourcePath string) string {
	h := sha256.Sum256([]byte(domain + ":" + sourcePath))
	return "feed-" + hex.EncodeToString(h[:8])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
