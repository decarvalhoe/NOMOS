package compliance

import (
	"path/filepath"
	"testing"
)

func testRegistry() ForbiddenClaimsRegistry {
	return ForbiddenClaimsRegistry{
		ForbiddenClaims: []ForbiddenClaim{
			{ID: "FC-001", Pattern: "compliant", Aliases: []string{"compliance", "comply"}, Reason: "no evidence"},
			{ID: "FC-002", Pattern: "certified", Aliases: []string{"certification"}, Reason: "no cert"},
			{ID: "FC-003", Pattern: "enterprise-grade", Aliases: []string{"enterprise grade"}, Reason: "not demonstrated"},
			{ID: "FC-004", Pattern: "production-ready", Aliases: []string{"production ready"}, Reason: "no release evidence"},
			{ID: "FC-005", Pattern: "guaranteed", Aliases: []string{"guarantee"}, Reason: "no SLA"},
		},
		AllowedAlternatives: []AllowedAlternative{
			{InsteadOf: "compliant", Use: "designed to support compliance workflows"},
			{InsteadOf: "certified", Use: "undergoing validation"},
		},
	}
}

func TestScanForbiddenClaims_DetectsCompliant(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "Our platform is fully compliant with industry standards.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail, got %s", result.GateVerdict)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].ClaimID != "FC-001" {
		t.Fatalf("expected FC-001, got %s", result.Violations[0].ClaimID)
	}
	if result.Violations[0].Alternative != "designed to support compliance workflows" {
		t.Fatalf("expected alternative text, got %q", result.Violations[0].Alternative)
	}
}

func TestScanForbiddenClaims_DetectsCertified(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "about.md", "The system is certified for production use.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) < 1 {
		t.Fatal("expected at least 1 violation for certified")
	}
}

func TestScanForbiddenClaims_DetectsEnterpriseGrade(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "features.md", "Nomos provides enterprise-grade security.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail for enterprise-grade, got %s", result.GateVerdict)
	}
}

func TestScanForbiddenClaims_DetectsProductionReady(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "deploy.md", "This release is production-ready.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail for production-ready, got %s", result.GateVerdict)
	}
}

func TestScanForbiddenClaims_DetectsAlias(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "info.md", "We comply with all applicable regulations.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation for alias 'comply', got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_ExcludesNegation(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "This product does not claim compliance with any standard.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations (negation excluded), got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_ExcludesHeadings(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "# Compliance Overview\nSome safe text.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations (heading excluded), got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_ExcludesForbiddenListItself(t *testing.T) {
	dir := t.TempDir()
	// Lines describing what is forbidden should be excluded.
	writeTestMD(t, dir, "policy.md", "The following claims are forbidden: compliant, certified.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations (forbidden-list excluded), got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_ExcludesTableRows(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "matrix.md", "| Status | compliant | not yet |\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations (table row excluded), got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_ExcludesAlternatives(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "guide.md", "Instead of compliant, use: designed to support compliance workflows.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations (alternative excluded), got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_MultipleViolations(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md",
		"We are fully compliant.\nOur product is certified.\nEnterprise-grade solution.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) < 3 {
		t.Fatalf("expected at least 3 violations, got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_CleanDoc(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md",
		"Nomos is a canonical-first verification tool.\nIt produces structured reports.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GateVerdict != "pass" {
		t.Fatalf("expected pass for clean doc, got %s", result.GateVerdict)
	}
}

func TestScanForbiddenClaims_IgnoresNonMD(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "code.go", "// This is compliant with the spec.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("expected 0 violations for .go file, got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_SingleFile(t *testing.T) {
	dir := t.TempDir()
	p := writeTestMD(t, dir, "claim.md", "We guarantee 99.9% uptime.\n")

	result, err := ScanForbiddenClaims([]string{p}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalFiles != 1 {
		t.Fatalf("expected 1 file, got %d", result.TotalFiles)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(result.Violations))
	}
}

func TestScanForbiddenClaims_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GateVerdict != "pass" {
		t.Fatalf("expected pass for empty dir, got %s", result.GateVerdict)
	}
}

func TestLoadForbiddenClaims_RealFile(t *testing.T) {
	p := filepath.Join("..", "..", "..", "docs", "regulated", "customer-integration", "forbidden-claims.yaml")
	reg, err := LoadForbiddenClaims(p)
	if err != nil {
		t.Skipf("real forbidden-claims not available: %v", err)
	}
	if len(reg.ForbiddenClaims) < 10 {
		t.Fatalf("expected at least 10 forbidden claims, got %d", len(reg.ForbiddenClaims))
	}
	if len(reg.AllowedAlternatives) < 4 {
		t.Fatalf("expected at least 4 alternatives, got %d", len(reg.AllowedAlternatives))
	}
}

func TestLoadForbiddenClaims_NotFound(t *testing.T) {
	_, err := LoadForbiddenClaims("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestViolationHasLineText(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "x.md", "This product is production-ready for all teams.\n")

	result, err := ScanForbiddenClaims([]string{dir}, testRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) == 0 {
		t.Fatal("expected violation")
	}
	if result.Violations[0].LineText == "" {
		t.Fatal("expected non-empty line text")
	}
	if result.Violations[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", result.Violations[0].Line)
	}
}
