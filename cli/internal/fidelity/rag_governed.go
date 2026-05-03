package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// AuthorityLevel classifies how authoritative a chunk is.
type AuthorityLevel string

const (
	AuthorityCertified   AuthorityLevel = "certified"   // approved atom, full provenance
	AuthorityProvisional AuthorityLevel = "provisional"  // draft/pending, source present
	AuthorityDerived     AuthorityLevel = "derived"      // generated content
	AuthorityUncertified AuthorityLevel = "uncertified"  // missing provenance
)

// SafetyGate is a pre-embedding check result.
type SafetyGate struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// ProvenanceLink traces a chunk back to its source.
type ProvenanceLink struct {
	SourceFile string `json:"source_file"`
	SourceLine int    `json:"source_line"`
	SourceHash string `json:"source_hash"`
	AtomID     string `json:"atom_id"`
	ParentID   string `json:"parent_id,omitempty"`
	Profile    string `json:"profile"`
}

// GovernanceTag carries governance metadata for a chunk.
type GovernanceTag struct {
	Owner           string `json:"owner,omitempty"`
	Status          string `json:"status,omitempty"`
	Confidentiality string `json:"confidentiality,omitempty"`
	ReviewState     string `json:"review_state"`
	Version         string `json:"version,omitempty"`
}

// GovernedChunk is a RAG chunk enriched with governance, authority, and provenance.
type GovernedChunk struct {
	ChunkID      string         `json:"chunk_id"`
	CanonicalRef string         `json:"canonical_ref"`
	Content      string         `json:"content"`
	ContentHash  string         `json:"content_hash"`
	TokenCount   int            `json:"token_count"`
	Domain       string         `json:"domain"`
	Depth        int            `json:"depth"`
	Authority    AuthorityLevel `json:"authority"`
	Governance   GovernanceTag  `json:"governance"`
	Provenance   ProvenanceLink `json:"provenance"`
	SafetyGates  []SafetyGate   `json:"safety_gates"`
	Embeddable   bool           `json:"embeddable"`
	RejectReason string         `json:"reject_reason,omitempty"`
}

// GovernedProjectionResult holds the full projection output.
type GovernedProjectionResult struct {
	TotalAtoms   int             `json:"total_atoms"`
	Embeddable   int             `json:"embeddable"`
	Rejected     int             `json:"rejected"`
	ChainHash    string          `json:"chain_hash"`
	Chunks       []GovernedChunk `json:"chunks"`
}

// GovernedProjectionConfig controls the governed RAG projection.
type GovernedProjectionConfig struct {
	MaxTokens       int
	MinTokens       int
	DefaultDomain   string
	DefaultOwner    string
	DefaultVersion  string
	Profile         string
	// RequireApproved blocks non-approved atoms from embedding
	RequireApproved bool
	// RequireHash blocks atoms without content hash
	RequireHash bool
	// BlockConfidential blocks restricted/secret content
	BlockConfidential bool
}

// DefaultGovernedConfig returns production defaults.
func DefaultGovernedConfig() GovernedProjectionConfig {
	return GovernedProjectionConfig{
		MaxTokens:         512,
		MinTokens:         10,
		DefaultDomain:     "unknown",
		Profile:           "default",
		RequireApproved:   false,
		RequireHash:       true,
		BlockConfidential: true,
	}
}

// ProjectGoverned converts PortableAtoms into governed RAG chunks.
func ProjectGoverned(atoms []PortableAtom, governance map[string]GovernanceTag, config GovernedProjectionConfig) GovernedProjectionResult {
	if config.MaxTokens <= 0 {
		config.MaxTokens = 512
	}
	if config.MinTokens <= 0 {
		config.MinTokens = 10
	}

	result := GovernedProjectionResult{TotalAtoms: len(atoms)}
	h := sha256.New()

	for _, atom := range atoms {
		gov := governance[atom.ID]
		if gov.ReviewState == "" {
			gov.ReviewState = "draft"
		}
		if gov.Owner == "" {
			gov.Owner = config.DefaultOwner
		}
		if gov.Version == "" {
			gov.Version = config.DefaultVersion
		}

		chunk := GovernedChunk{
			ChunkID:      fmt.Sprintf("rag-%s", atom.ID),
			CanonicalRef: atom.CanonicalRef,
			Content:      atom.Text,
			ContentHash:  atom.ContentHash,
			TokenCount:   estimateTokens(atom.Text),
			Domain:       coalesce(atom.Domain, config.DefaultDomain),
			Depth:        atom.Depth,
			Authority:    classifyAuthority(atom, gov),
			Governance:   gov,
			Provenance: ProvenanceLink{
				SourceFile: "",
				SourceLine: atom.SourceLine,
				SourceHash: atom.ContentHash,
				AtomID:     atom.ID,
				ParentID:   atom.ParentID,
				Profile:    coalesce(atom.Profile, config.Profile),
			},
		}

		gates := runSafetyGates(chunk, config)
		chunk.SafetyGates = gates

		allPassed := true
		var rejectReasons []string
		for _, g := range gates {
			if !g.Passed {
				allPassed = false
				rejectReasons = append(rejectReasons, g.Reason)
			}
		}

		chunk.Embeddable = allPassed
		if !allPassed {
			chunk.RejectReason = strings.Join(rejectReasons, "; ")
			result.Rejected++
		} else {
			result.Embeddable++
		}

		h.Write([]byte(chunk.ContentHash))
		result.Chunks = append(result.Chunks, chunk)
	}

	result.ChainHash = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return result
}

// WriteGovernedJSON serializes the result.
func WriteGovernedJSON(w io.Writer, result GovernedProjectionResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func classifyAuthority(atom PortableAtom, gov GovernanceTag) AuthorityLevel {
	if atom.ContentHash == "" {
		return AuthorityUncertified
	}
	switch gov.ReviewState {
	case "approved":
		return AuthorityCertified
	case "draft", "pending", "amended":
		return AuthorityProvisional
	case "rejected":
		return AuthorityDerived
	}
	return AuthorityProvisional
}

func runSafetyGates(chunk GovernedChunk, config GovernedProjectionConfig) []SafetyGate {
	var gates []SafetyGate

	// Gate 1: non-empty content
	gates = append(gates, SafetyGate{
		Name:   "content_present",
		Passed: strings.TrimSpace(chunk.Content) != "",
		Reason: ternary(strings.TrimSpace(chunk.Content) == "", "empty content", ""),
	})

	// Gate 2: content hash present
	if config.RequireHash {
		gates = append(gates, SafetyGate{
			Name:   "hash_present",
			Passed: chunk.ContentHash != "",
			Reason: ternary(chunk.ContentHash == "", "missing content hash", ""),
		})
	}

	// Gate 3: token count within bounds
	withinBounds := chunk.TokenCount >= config.MinTokens && chunk.TokenCount <= config.MaxTokens
	gates = append(gates, SafetyGate{
		Name:   "token_bounds",
		Passed: withinBounds,
		Reason: ternary(!withinBounds,
			fmt.Sprintf("token count %d outside [%d, %d]", chunk.TokenCount, config.MinTokens, config.MaxTokens),
			""),
	})

	// Gate 4: approval required
	if config.RequireApproved {
		approved := chunk.Governance.ReviewState == "approved"
		gates = append(gates, SafetyGate{
			Name:   "approved_only",
			Passed: approved,
			Reason: ternary(!approved, fmt.Sprintf("review_state is %q, not approved", chunk.Governance.ReviewState), ""),
		})
	}

	// Gate 5: confidentiality
	if config.BlockConfidential {
		conf := strings.ToLower(chunk.Governance.Confidentiality)
		blocked := conf == "restricted" || conf == "secret"
		gates = append(gates, SafetyGate{
			Name:   "confidentiality_clear",
			Passed: !blocked,
			Reason: ternary(blocked, fmt.Sprintf("confidentiality %q blocks embedding", conf), ""),
		})
	}

	return gates
}

func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	// Rough estimate: 1 token ≈ 0.75 words for English/French
	tokens := int(float64(words) * 1.33)
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ternary(cond bool, ifTrue, ifFalse string) string {
	if cond {
		return ifTrue
	}
	return ifFalse
}
