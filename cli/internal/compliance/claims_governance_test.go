package compliance

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestMD(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// --- ScanPublicClaims ---

func TestScanPublicClaims_DetectsComplianceClaim(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "This product is fully compliant with GDPR.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	if claims[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", claims[0].Line)
	}
}

func TestScanPublicClaims_DetectsCertificationClaim(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "docs.md", "The system is certified for production use.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
}

func TestScanPublicClaims_DetectsRegulatoryCitation(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "security.md", "Our platform follows SOC 2 Type II controls.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
}

func TestScanPublicClaims_DetectsGuaranteeClaim(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "sla.md", "We guarantee 99.9% uptime availability.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) == 0 {
		t.Fatal("expected at least 1 claim for guarantee+SLA")
	}
}

func TestScanPublicClaims_ExcludesFutureStatements(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "This product does not claim compliance with any standard.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims (excluded), got %d", len(claims))
	}
}

func TestScanPublicClaims_ExcludesHeadings(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "# Compliance Overview\nSome text.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims (heading excluded), got %d", len(claims))
	}
}

func TestScanPublicClaims_ExcludesPlannedLanguage(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "roadmap.md", "Compliance will be achieved in the next release.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims (future language), got %d", len(claims))
	}
}

func TestScanPublicClaims_MultipleClaimsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "README.md", "We are fully compliant.\nOur data is validated.\n")
	writeTestMD(t, dir, "security.md", "Platform is ISO 27001 certified.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 3 {
		t.Fatalf("expected 3 claims, got %d", len(claims))
	}
}

func TestScanPublicClaims_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims, got %d", len(claims))
	}
}

func TestScanPublicClaims_NonMDIgnored(t *testing.T) {
	dir := t.TempDir()
	writeTestMD(t, dir, "code.go", "// This is compliant with the spec.\n")

	claims, err := ScanPublicClaims([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected 0 claims (non-md), got %d", len(claims))
	}
}

func TestScanPublicClaims_SingleFile(t *testing.T) {
	dir := t.TempDir()
	p := writeTestMD(t, dir, "claim.md", "The product ensures data integrity.\n")

	claims, err := ScanPublicClaims([]string{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
}

// --- GovernClaims ---

func TestGovernClaims_FullyEvidenced(t *testing.T) {
	claims := []PublicClaim{{Text: "We are compliant.", SourcePath: "README.md", Line: 1}}
	registry := map[string]ClaimEvidence{
		"README.md:1": {
			IntendedUse:      "Product compliance statement",
			Owner:            "compliance-team",
			RiskClass:        "high",
			ExternalRef:      "ISO-27001",
			ControlReq:       "CTRL-001",
			ImplementRef:     "cli/internal/compliance/",
			VerificationRef:  "tests/compliance_test.go",
			EvidenceArtifact: "reports/compliance.json",
			ReleaseGate:      "regulated-evidence-pack",
		},
	}

	result := GovernClaims(claims, registry)
	if result.GateVerdict != "pass" {
		t.Fatalf("expected pass, got %s", result.GateVerdict)
	}
	if result.Qualified != 1 {
		t.Fatalf("expected 1 qualified, got %d", result.Qualified)
	}
	if result.NotQualified != 0 {
		t.Fatalf("expected 0 not_qualified, got %d", result.NotQualified)
	}
	if result.GovernedClaims[0].Status != StatusVerified {
		t.Fatalf("expected verified, got %s", result.GovernedClaims[0].Status)
	}
}

func TestGovernClaims_NoEvidence(t *testing.T) {
	claims := []PublicClaim{{Text: "We guarantee SLA.", SourcePath: "sla.md", Line: 5}}
	registry := map[string]ClaimEvidence{}

	result := GovernClaims(claims, registry)
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail, got %s", result.GateVerdict)
	}
	if result.NotQualified != 1 {
		t.Fatalf("expected 1 not_qualified, got %d", result.NotQualified)
	}
	gc := result.GovernedClaims[0]
	if gc.Status != StatusNotQualified {
		t.Fatalf("expected not_qualified, got %s", gc.Status)
	}
	if len(gc.MissingFields) != 9 {
		t.Fatalf("expected 9 missing fields, got %d", len(gc.MissingFields))
	}
}

func TestGovernClaims_PartialEvidence(t *testing.T) {
	claims := []PublicClaim{{Text: "Certified product.", SourcePath: "docs.md", Line: 3}}
	registry := map[string]ClaimEvidence{
		"docs.md:3": {
			IntendedUse: "Marketing claim",
			Owner:       "product-team",
			// Missing: risk_classification, external_ref, etc.
		},
	}

	result := GovernClaims(claims, registry)
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail for partial evidence, got %s", result.GateVerdict)
	}
	gc := result.GovernedClaims[0]
	if gc.Status != StatusNotQualified {
		t.Fatalf("expected not_qualified, got %s", gc.Status)
	}
	if len(gc.MissingFields) != 7 {
		t.Fatalf("expected 7 missing fields, got %d: %v", len(gc.MissingFields), gc.MissingFields)
	}
}

func TestGovernClaims_MixedClaims(t *testing.T) {
	claims := []PublicClaim{
		{Text: "We are compliant.", SourcePath: "a.md", Line: 1},
		{Text: "Platform is certified.", SourcePath: "b.md", Line: 1},
	}
	registry := map[string]ClaimEvidence{
		"a.md:1": {
			IntendedUse: "x", Owner: "o", RiskClass: "h",
			ExternalRef: "e", ControlReq: "c", ImplementRef: "i",
			VerificationRef: "v", EvidenceArtifact: "a", ReleaseGate: "g",
		},
		// b.md:1 has no evidence
	}

	result := GovernClaims(claims, registry)
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail (1 ungoverned), got %s", result.GateVerdict)
	}
	if result.Qualified != 1 || result.NotQualified != 1 {
		t.Fatalf("expected 1 qualified + 1 not_qualified, got %d/%d", result.Qualified, result.NotQualified)
	}
}

func TestGovernClaims_ZeroClaims(t *testing.T) {
	result := GovernClaims(nil, nil)
	if result.GateVerdict != "pass" {
		t.Fatalf("expected pass for zero claims, got %s", result.GateVerdict)
	}
	if result.TotalClaims != 0 {
		t.Fatalf("expected 0 total claims, got %d", result.TotalClaims)
	}
}

// --- checkMissingFields ---

func TestCheckMissingFields_AllPresent(t *testing.T) {
	e := ClaimEvidence{
		IntendedUse: "x", Owner: "o", RiskClass: "h",
		ExternalRef: "e", ControlReq: "c", ImplementRef: "i",
		VerificationRef: "v", EvidenceArtifact: "a", ReleaseGate: "g",
	}
	missing := checkMissingFields(e)
	if len(missing) != 0 {
		t.Fatalf("expected 0 missing, got %v", missing)
	}
}

func TestCheckMissingFields_AllMissing(t *testing.T) {
	missing := checkMissingFields(ClaimEvidence{})
	if len(missing) != 9 {
		t.Fatalf("expected 9 missing fields, got %d", len(missing))
	}
}

// --- Integration ---

func TestScanAndGovern_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	p := writeTestMD(t, dir, "README.md",
		"# Product\n\nThis product ensures data integrity at all times.\n\nSome non-claim text here.\n")

	claims, err := ScanPublicClaims([]string{p})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}

	result := GovernClaims(claims, map[string]ClaimEvidence{})
	if result.GateVerdict != "fail" {
		t.Fatalf("expected fail for ungoverned claim, got %s", result.GateVerdict)
	}
	if result.GovernedClaims[0].Status != StatusNotQualified {
		t.Fatalf("expected not_qualified")
	}
}
