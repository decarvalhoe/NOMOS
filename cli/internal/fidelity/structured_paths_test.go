package fidelity

import (
	"testing"
)

func TestMapPathIdentity(t *testing.T) {
	rules := DefaultRules()
	cases := []struct {
		path string
		role SemanticRole
	}{
		{"/project/id", RoleIdentity},
		{"/node/external_id", RoleIdentity},
		{"/record/uuid", RoleIdentity},
	}
	for _, tc := range cases {
		m := MapPath(tc.path, rules)
		if m.Role != tc.role {
			t.Fatalf("path %s: expected %s, got %s", tc.path, tc.role, m.Role)
		}
	}
}

func TestMapPathMetadata(t *testing.T) {
	rules := DefaultRules()
	cases := []string{
		"/schema_version",
		"/project/version",
		"/record/created_at",
		"/document/updated_at",
		"/feed/owner",
		"/extra/metadata/key",
	}
	for _, path := range cases {
		m := MapPath(path, rules)
		if m.Role != RoleMetadata {
			t.Fatalf("path %s: expected metadata, got %s (%s)", path, m.Role, m.Reason)
		}
	}
}

func TestMapPathConfig(t *testing.T) {
	rules := DefaultRules()
	cases := []string{
		"/app/config/db",
		"/settings/timeout",
		"/gate/threshold",
		"/feature/enabled",
		"/limits/max_retries",
	}
	for _, path := range cases {
		m := MapPath(path, rules)
		if m.Role != RoleConfig {
			t.Fatalf("path %s: expected config, got %s (%s)", path, m.Role, m.Reason)
		}
	}
}

func TestMapPathPolicy(t *testing.T) {
	rules := DefaultRules()
	cases := []string{
		"/approval/policy/min",
		"/access/permissions",
		"/validation/constraints",
		"/admission/gate/mode",
	}
	for _, path := range cases {
		m := MapPath(path, rules)
		if m.Role != RolePolicy {
			t.Fatalf("path %s: expected policy, got %s (%s)", path, m.Role, m.Reason)
		}
	}
}

func TestMapPathSchema(t *testing.T) {
	rules := DefaultRules()
	cases := []string{
		"/document/schema/format",
		"/definitions/Node",
		"/$defs",
		"/object/properties",
	}
	for _, path := range cases {
		m := MapPath(path, rules)
		if m.Role != RoleSchema {
			t.Fatalf("path %s: expected schema, got %s (%s)", path, m.Role, m.Reason)
		}
	}
}

func TestMapPathData(t *testing.T) {
	rules := DefaultRules()
	cases := []string{
		"/document/content",
		"/feed/nodes",
		"/response/data/items",
		"/report/results",
	}
	for _, path := range cases {
		m := MapPath(path, rules)
		if m.Role != RoleData {
			t.Fatalf("path %s: expected data, got %s (%s)", path, m.Role, m.Reason)
		}
	}
}

func TestMapPathUnknown(t *testing.T) {
	rules := DefaultRules()
	m := MapPath("/completely/random/path", rules)
	if m.Role != RoleUnknown {
		t.Fatalf("expected unknown, got %s", m.Role)
	}
	if m.Confidence != "low" {
		t.Fatalf("expected low confidence for unknown, got %s", m.Confidence)
	}
}

func TestMapPathYAMLDotNotation(t *testing.T) {
	rules := DefaultRules()
	m := MapPath("project.version", rules)
	if m.Role != RoleMetadata {
		t.Fatalf("expected metadata for dot notation version, got %s", m.Role)
	}
}

func TestMapPathArrayIndicesStripped(t *testing.T) {
	rules := DefaultRules()
	m := MapPath("/items/0/id", rules)
	if m.Role != RoleIdentity {
		t.Fatalf("expected identity (array index stripped), got %s (%s)", m.Role, m.Reason)
	}
}

func TestMapPathsMultiple(t *testing.T) {
	rules := DefaultRules()
	paths := []string{
		"/project/id",
		"/config/timeout",
		"/random/thing",
	}
	mappings := MapPaths(paths, rules)

	if len(mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(mappings))
	}
	if mappings[0].Role != RoleIdentity {
		t.Fatalf("expected identity first, got %s", mappings[0].Role)
	}
	if mappings[1].Role != RoleConfig {
		t.Fatalf("expected config second, got %s", mappings[1].Role)
	}
	if mappings[2].Role != RoleUnknown {
		t.Fatalf("expected unknown third, got %s", mappings[2].Role)
	}
}

func TestMapPathConfidenceHigh(t *testing.T) {
	rules := DefaultRules()
	m := MapPath("/record/schema_version", rules)
	if m.Confidence != "high" {
		t.Fatalf("expected high confidence for schema_version, got %s", m.Confidence)
	}
}

func TestMapPathCustomRules(t *testing.T) {
	rules := []MappingRule{
		{Pattern: "/score", MatchMode: "suffix", Role: RoleData, Confidence: "high", Reason: "score field"},
		{Pattern: "/game/", MatchMode: "prefix", Role: RoleData, Confidence: "medium", Reason: "game prefix"},
	}

	m1 := MapPath("/player/score", rules)
	if m1.Role != RoleData {
		t.Fatalf("expected data for /player/score, got %s", m1.Role)
	}

	m2 := MapPath("/game/board", rules)
	if m2.Role != RoleData {
		t.Fatalf("expected data for /game/board, got %s", m2.Role)
	}
}

func TestMapPathCaseInsensitive(t *testing.T) {
	rules := DefaultRules()
	m := MapPath("/Project/VERSION", rules)
	if m.Role != RoleMetadata {
		t.Fatalf("expected metadata (case insensitive), got %s", m.Role)
	}
}

func TestDefaultRulesCount(t *testing.T) {
	rules := DefaultRules()
	if len(rules) < 30 {
		t.Fatalf("expected >= 30 default rules, got %d", len(rules))
	}
}
