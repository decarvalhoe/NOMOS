package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// DocClass classifies a document's authenticity.
type DocClass string

const (
	ClassOriginal   DocClass = "original"
	ClassSurrogate  DocClass = "surrogate"
	ClassDerivedDoc DocClass = "derived"
	ClassUnverified DocClass = "unverified"
)

// DocStatus tracks verification state.
type DocStatus string

const (
	StatusVerified   DocStatus = "verified"
	StatusSurrogate  DocStatus = "surrogate"
	StatusDerivedDoc DocStatus = "derived"
	StatusUnverified DocStatus = "unverified"
	StatusBlocked    DocStatus = "blocked"
)

// RegisterEntry is a single document in the licensed document register.
type RegisterEntry struct {
	ID          string    `json:"id"           yaml:"id"`
	Title       string    `json:"title"        yaml:"title"`
	Publisher   string    `json:"publisher"    yaml:"publisher"`
	SourceURL   string    `json:"source_url"   yaml:"source_url"`
	Class       DocClass  `json:"class"        yaml:"class"`
	Status      DocStatus `json:"status"       yaml:"status"`
	Hash        string    `json:"hash"         yaml:"hash"`
	ObtainedAt  string    `json:"obtained_at"  yaml:"obtained_at"`
	License     string    `json:"license"      yaml:"license"`
	OriginalRef string    `json:"original_ref" yaml:"original_ref"`
	Retention   string    `json:"retention"    yaml:"retention"`
	Notes       string    `json:"notes"        yaml:"notes"`
}

// AllowedUses returns what the document class permits.
func (e RegisterEntry) AllowedUses() []string {
	switch e.Class {
	case ClassOriginal:
		return []string{"structured_contract", "citation_external", "golden_case", "vector_index"}
	case ClassSurrogate:
		return []string{"structured_contract", "citation_internal", "vector_index"}
	case ClassDerivedDoc:
		return []string{"citation_internal"}
	default:
		return nil
	}
}

// LicensedDocRegister is the full register.
type LicensedDocRegister struct {
	SchemaVersion string          `json:"schema_version" yaml:"schema_version"`
	Documents     []RegisterEntry `json:"documents"      yaml:"documents"`
}

// ClassifyResult is the outcome of classifying a document.
type ClassifyResult struct {
	EntryID     string   `json:"entry_id"`
	Class       DocClass `json:"class"`
	Status      DocStatus `json:"status"`
	HashMatch   bool     `json:"hash_match"`
	AllowedUses []string `json:"allowed_uses"`
	Blocked     bool     `json:"blocked"`
	Reason      string   `json:"reason"`
}

// RefPolicyFinding is a single policy violation.
type RefPolicyFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Blocking bool   `json:"blocking"`
	DocID    string `json:"doc_id"`
	Message  string `json:"message"`
}

// RefPolicyResult holds the policy evaluation output.
type RefPolicyResult struct {
	TotalDocs    int                `json:"total_docs"`
	Original     int                `json:"original"`
	Surrogate    int                `json:"surrogate"`
	Derived      int                `json:"derived"`
	Unverified   int                `json:"unverified"`
	Blocked      int                `json:"blocked"`
	Findings     []RefPolicyFinding `json:"findings,omitempty"`
}

// LoadRegister reads a licensed document register from YAML.
func LoadRegister(path string) (LicensedDocRegister, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LicensedDocRegister{}, err
	}
	return ParseRegister(data)
}

// ParseRegister parses register YAML bytes.
func ParseRegister(data []byte) (LicensedDocRegister, error) {
	var reg LicensedDocRegister
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return LicensedDocRegister{}, err
	}
	return reg, nil
}

// ClassifyDocument checks a document's content hash against the register.
func ClassifyDocument(contentHash string, register LicensedDocRegister) ClassifyResult {
	for _, entry := range register.Documents {
		if entry.Hash == contentHash {
			return ClassifyResult{
				EntryID:     entry.ID,
				Class:       entry.Class,
				Status:      entry.Status,
				HashMatch:   true,
				AllowedUses: entry.AllowedUses(),
				Blocked:     entry.Status == StatusBlocked,
				Reason:      fmt.Sprintf("matched register entry %s (%s)", entry.ID, entry.Class),
			}
		}
	}
	return ClassifyResult{
		Class:     ClassUnverified,
		Status:    StatusUnverified,
		HashMatch: false,
		Blocked:   true,
		Reason:    "no matching entry in licensed document register",
	}
}

// ClassifyByID looks up a document by register ID.
func ClassifyByID(id string, register LicensedDocRegister) ClassifyResult {
	for _, entry := range register.Documents {
		if entry.ID == id {
			return ClassifyResult{
				EntryID:     entry.ID,
				Class:       entry.Class,
				Status:      entry.Status,
				HashMatch:   true,
				AllowedUses: entry.AllowedUses(),
				Blocked:     entry.Status == StatusBlocked,
				Reason:      fmt.Sprintf("register entry %s (%s)", entry.ID, entry.Class),
			}
		}
	}
	return ClassifyResult{
		Class:   ClassUnverified,
		Status:  StatusUnverified,
		Blocked: true,
		Reason:  fmt.Sprintf("ID %q not found in register", id),
	}
}

// EvaluateRegister checks all entries for policy compliance.
func EvaluateRegister(register LicensedDocRegister) RefPolicyResult {
	result := RefPolicyResult{TotalDocs: len(register.Documents)}
	var findings []RefPolicyFinding

	for _, entry := range register.Documents {
		switch entry.Class {
		case ClassOriginal:
			result.Original++
		case ClassSurrogate:
			result.Surrogate++
		case ClassDerivedDoc:
			result.Derived++
		default:
			result.Unverified++
		}

		if entry.Status == StatusBlocked {
			result.Blocked++
		}

		if entry.Hash == "" {
			findings = append(findings, RefPolicyFinding{
				Code: "REF_NO_HASH", Severity: "critical", Blocking: true,
				DocID:   entry.ID,
				Message: fmt.Sprintf("document %q has no content hash", entry.ID),
			})
		}

		if entry.Title == "" {
			findings = append(findings, RefPolicyFinding{
				Code: "REF_NO_TITLE", Severity: "medium", Blocking: false,
				DocID:   entry.ID,
				Message: fmt.Sprintf("document %q has no title", entry.ID),
			})
		}

		if entry.Class == ClassDerivedDoc && entry.OriginalRef == "" {
			findings = append(findings, RefPolicyFinding{
				Code: "REF_DERIVED_NO_ORIGINAL", Severity: "high", Blocking: true,
				DocID:   entry.ID,
				Message: fmt.Sprintf("derived document %q has no original_ref", entry.ID),
			})
		}

		if entry.Status == StatusUnverified {
			findings = append(findings, RefPolicyFinding{
				Code: "REF_UNVERIFIED", Severity: "high", Blocking: true,
				DocID:   entry.ID,
				Message: fmt.Sprintf("document %q has unverified status", entry.ID),
			})
		}
	}

	result.Findings = findings
	return result
}

// HashFile computes SHA-256 of a file's content.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// WriteRefPolicyJSON serializes the result.
func WriteRefPolicyJSON(w io.Writer, result RefPolicyResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
