package fidelity

import (
	"strings"
)

// SemanticRole classifies the purpose of a structured path.
type SemanticRole string

const (
	RoleConfig   SemanticRole = "config"
	RoleData     SemanticRole = "data"
	RoleMetadata SemanticRole = "metadata"
	RolePolicy   SemanticRole = "policy"
	RoleSchema   SemanticRole = "schema"
	RoleIdentity SemanticRole = "identity"
	RoleUnknown  SemanticRole = "unknown"
)

// PathMapping maps a structured path to its semantic role.
type PathMapping struct {
	Path       string       `json:"path"`
	Role       SemanticRole `json:"role"`
	Confidence string       `json:"confidence"` // high, medium, low
	Reason     string       `json:"reason"`
}

// MappingRule defines a pattern-to-role association.
type MappingRule struct {
	Pattern    string       // suffix, prefix, or contains match
	MatchMode  string       // "suffix", "prefix", "contains", "exact"
	Role       SemanticRole
	Confidence string
	Reason     string
}

// DefaultRules returns the standard semantic mapping rules.
func DefaultRules() []MappingRule {
	return []MappingRule{
		// Identity
		{Pattern: "/id", MatchMode: "suffix", Role: RoleIdentity, Confidence: "high", Reason: "identifier field"},
		{Pattern: "/name", MatchMode: "suffix", Role: RoleIdentity, Confidence: "medium", Reason: "name field"},
		{Pattern: "/external_id", MatchMode: "suffix", Role: RoleIdentity, Confidence: "high", Reason: "external identifier"},
		{Pattern: "/uuid", MatchMode: "suffix", Role: RoleIdentity, Confidence: "high", Reason: "UUID field"},

		// Metadata
		{Pattern: "/version", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "version field"},
		{Pattern: "/schema_version", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "schema version"},
		{Pattern: "/created_at", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "creation timestamp"},
		{Pattern: "/updated_at", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "update timestamp"},
		{Pattern: "/generated_at", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "generation timestamp"},
		{Pattern: "/owner", MatchMode: "suffix", Role: RoleMetadata, Confidence: "high", Reason: "ownership field"},
		{Pattern: "/status", MatchMode: "suffix", Role: RoleMetadata, Confidence: "medium", Reason: "status field"},
		{Pattern: "/metadata", MatchMode: "contains", Role: RoleMetadata, Confidence: "high", Reason: "metadata container"},
		{Pattern: "/labels", MatchMode: "suffix", Role: RoleMetadata, Confidence: "medium", Reason: "labels/tags"},
		{Pattern: "/annotations", MatchMode: "suffix", Role: RoleMetadata, Confidence: "medium", Reason: "annotations"},

		// Config
		{Pattern: "/config", MatchMode: "contains", Role: RoleConfig, Confidence: "high", Reason: "config container"},
		{Pattern: "/settings", MatchMode: "contains", Role: RoleConfig, Confidence: "high", Reason: "settings container"},
		{Pattern: "/options", MatchMode: "suffix", Role: RoleConfig, Confidence: "medium", Reason: "options field"},
		{Pattern: "/timeout", MatchMode: "suffix", Role: RoleConfig, Confidence: "high", Reason: "timeout setting"},
		{Pattern: "/threshold", MatchMode: "suffix", Role: RoleConfig, Confidence: "high", Reason: "threshold setting"},
		{Pattern: "/max_", MatchMode: "contains", Role: RoleConfig, Confidence: "medium", Reason: "max limit"},
		{Pattern: "/min_", MatchMode: "contains", Role: RoleConfig, Confidence: "medium", Reason: "min limit"},
		{Pattern: "/enabled", MatchMode: "suffix", Role: RoleConfig, Confidence: "high", Reason: "feature toggle"},
		{Pattern: "/disabled", MatchMode: "suffix", Role: RoleConfig, Confidence: "high", Reason: "feature toggle"},

		// Policy
		{Pattern: "/policy", MatchMode: "contains", Role: RolePolicy, Confidence: "high", Reason: "policy container"},
		{Pattern: "/rules", MatchMode: "suffix", Role: RolePolicy, Confidence: "medium", Reason: "rules field"},
		{Pattern: "/required", MatchMode: "suffix", Role: RolePolicy, Confidence: "medium", Reason: "requirement flag"},
		{Pattern: "/allowed", MatchMode: "suffix", Role: RolePolicy, Confidence: "medium", Reason: "allowlist"},
		{Pattern: "/denied", MatchMode: "suffix", Role: RolePolicy, Confidence: "medium", Reason: "denylist"},
		{Pattern: "/permissions", MatchMode: "suffix", Role: RolePolicy, Confidence: "high", Reason: "permissions field"},
		{Pattern: "/constraints", MatchMode: "suffix", Role: RolePolicy, Confidence: "high", Reason: "constraints"},
		{Pattern: "/gate", MatchMode: "contains", Role: RolePolicy, Confidence: "medium", Reason: "gate control"},

		// Schema
		{Pattern: "/schema", MatchMode: "contains", Role: RoleSchema, Confidence: "high", Reason: "schema definition"},
		{Pattern: "/type", MatchMode: "suffix", Role: RoleSchema, Confidence: "medium", Reason: "type field"},
		{Pattern: "/properties", MatchMode: "suffix", Role: RoleSchema, Confidence: "high", Reason: "schema properties"},
		{Pattern: "/definitions", MatchMode: "contains", Role: RoleSchema, Confidence: "high", Reason: "schema definitions"},
		{Pattern: "/$defs", MatchMode: "contains", Role: RoleSchema, Confidence: "high", Reason: "JSON Schema $defs"},

		// Data
		{Pattern: "/content", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "content field"},
		{Pattern: "/text", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "text field"},
		{Pattern: "/value", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "value field"},
		{Pattern: "/items", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "items array"},
		{Pattern: "/entries", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "entries array"},
		{Pattern: "/nodes", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "nodes array"},
		{Pattern: "/data", MatchMode: "contains", Role: RoleData, Confidence: "high", Reason: "data container"},
		{Pattern: "/results", MatchMode: "suffix", Role: RoleData, Confidence: "medium", Reason: "results field"},
	}
}

// MapPaths classifies a list of structured paths using the provided rules.
func MapPaths(paths []string, rules []MappingRule) []PathMapping {
	var mappings []PathMapping
	for _, path := range paths {
		mapping := classifyPath(path, rules)
		mappings = append(mappings, mapping)
	}
	return mappings
}

// MapPath classifies a single structured path.
func MapPath(path string, rules []MappingRule) PathMapping {
	return classifyPath(path, rules)
}

func classifyPath(path string, rules []MappingRule) PathMapping {
	normalized := normalizePath(path)

	for _, rule := range rules {
		if matchRule(normalized, rule) {
			return PathMapping{
				Path:       path,
				Role:       rule.Role,
				Confidence: rule.Confidence,
				Reason:     rule.Reason,
			}
		}
	}

	return PathMapping{
		Path:       path,
		Role:       RoleUnknown,
		Confidence: "low",
		Reason:     "no matching rule",
	}
}

func matchRule(path string, rule MappingRule) bool {
	pattern := strings.ToLower(rule.Pattern)
	lower := strings.ToLower(path)

	switch rule.MatchMode {
	case "exact":
		return lower == pattern
	case "prefix":
		return strings.HasPrefix(lower, pattern)
	case "suffix":
		return strings.HasSuffix(lower, pattern)
	case "contains":
		return strings.Contains(lower, pattern)
	default:
		return strings.Contains(lower, pattern)
	}
}

func normalizePath(path string) string {
	// Normalize YAML dot notation to JSON pointer style.
	if !strings.HasPrefix(path, "/") && strings.Contains(path, ".") {
		path = "/" + strings.ReplaceAll(path, ".", "/")
	}
	// Ensure leading slash for JSON pointers.
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Strip array indices for pattern matching: /items/0/name → /items/name
	parts := strings.Split(path, "/")
	var cleaned []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		if isNumeric(part) {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return "/" + strings.Join(cleaned, "/")
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
