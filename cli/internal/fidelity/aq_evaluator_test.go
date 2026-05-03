package fidelity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Claim scanning ---

func TestScanClaimsCertified(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("This system is certified for production."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimCertified {
		t.Fatalf("expected 1 certified, got %v", claims)
	}
	if claims[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", claims[0].Line)
	}
}

func TestScanClaimsCompliance(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("The platform is compliant with standards."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimCompliance {
		t.Fatalf("expected compliance, got %v", claims)
	}
}

func TestScanClaimsRegulated(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("Provides regulated-grade evidence."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimRegulated {
		t.Fatalf("expected regulated, got %v", claims)
	}
}

func TestScanClaimsValidation(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("System has been validated."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimValidation {
		t.Fatalf("expected validation, got %v", claims)
	}
}

func TestScanClaimsQuality(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("Quality management system established."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimQuality {
		t.Fatalf("expected quality, got %v", claims)
	}
}

func TestScanClaimsPart11(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("Part 11 compliant records."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimRegulated {
		t.Fatalf("expected regulated, got %v", claims)
	}
}

func TestScanClaimsGxP(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("GxP compliant infrastructure."), "t.md")
	if len(claims) != 1 || claims[0].Kind != ClaimRegulated {
		t.Fatalf("expected regulated, got %v", claims)
	}
}

func TestScanClaimsNone(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("Plain text with no special language.\nJust docs."), "t.md")
	if len(claims) != 0 {
		t.Fatalf("expected 0, got %d", len(claims))
	}
}

func TestScanClaimsMultipleLines(t *testing.T) {
	input := "Line 1\nThis is certified.\nAlso compliant.\nEnd."
	claims, _ := ScanClaimsFromReader(strings.NewReader(input), "t.md")
	if len(claims) != 2 {
		t.Fatalf("expected 2, got %d", len(claims))
	}
	if claims[0].Line != 2 || claims[1].Line != 3 {
		t.Fatalf("wrong lines: %d, %d", claims[0].Line, claims[1].Line)
	}
}

func TestScanClaimsOnePerLine(t *testing.T) {
	claims, _ := ScanClaimsFromReader(strings.NewReader("Certified and also fully validated."), "t.md")
	if len(claims) != 1 {
		t.Fatalf("expected 1 (first match wins per line), got %d", len(claims))
	}
}

func TestScanClaimsFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("# Title\nThis is certified.\n"), 0o644)
	claims, err := ScanClaims(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1, got %d", len(claims))
	}
}

func TestScanClaimsInDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("Certified system."), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.yaml"), []byte("status: validated"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("Certified but ignored."), 0o644)
	claims, err := ScanClaimsInDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 2 {
		t.Fatalf("expected 2 (md+yaml), got %d", len(claims))
	}
}

func TestScanClaimsMissingFile(t *testing.T) {
	_, err := ScanClaims("/nonexistent.md")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Evidence levels ---

func TestEvidenceNQ0(t *testing.T) {
	assertLevel(t, AQEvidenceLevel{}, AQNQ0)
}

func TestEvidenceNQ1(t *testing.T) {
	assertLevel(t, AQEvidenceLevel{HasDocumentation: true}, AQNQ1)
}

func TestEvidenceNQ2(t *testing.T) {
	assertLevel(t, AQEvidenceLevel{HasDocumentation: true, HasCIGate: true, HasTests: true}, AQNQ2)
}

func TestEvidenceNQ3(t *testing.T) {
	assertLevel(t, AQEvidenceLevel{
		HasDocumentation: true, HasCIGate: true, HasTests: true,
		HasReview: true, HasAuditTrail: true, HasApproval: true,
	}, AQNQ3)
}

func TestEvidencePartialNQ2(t *testing.T) {
	// tests + docs but no CI = NQ-1
	assertLevel(t, AQEvidenceLevel{HasDocumentation: true, HasTests: true}, AQNQ1)
}

// --- Claim evaluation ---

func TestEvalAllSupported(t *testing.T) {
	claims := []DetectedClaim{
		{Text: "The tool is tested.", Kind: ClaimQuality},
		{Text: "Documentation is verified.", Kind: ClaimValidation},
	}
	ev := AQEvidenceLevel{HasDocumentation: true, HasTests: true, HasCIGate: true}
	r := EvaluateClaims(claims, ev)
	if r.Supported != 2 || r.Blocking != 0 {
		t.Fatalf("expected 2/0, got %d/%d", r.Supported, r.Blocking)
	}
	if r.CurrentLevel != AQNQ2 {
		t.Fatalf("expected NQ-2, got %q", r.CurrentLevel)
	}
}

func TestEvalOverclaim(t *testing.T) {
	claims := []DetectedClaim{{Text: "System is certified.", Kind: ClaimCertified}}
	ev := AQEvidenceLevel{HasDocumentation: true}
	r := EvaluateClaims(claims, ev)
	if r.Unsupported != 1 || r.Blocking != 1 {
		t.Fatalf("expected 1/1, got %d/%d", r.Unsupported, r.Blocking)
	}
	if r.Claims[0].Supported {
		t.Fatal("expected not supported")
	}
	if !strings.Contains(r.Claims[0].Reason, "overclaim") {
		t.Fatalf("expected overclaim, got %q", r.Claims[0].Reason)
	}
}

func TestEvalMixed(t *testing.T) {
	claims := []DetectedClaim{
		{Text: "This is tested.", Kind: ClaimQuality},
		{Text: "Regulated-grade compliance.", Kind: ClaimRegulated},
	}
	ev := AQEvidenceLevel{HasDocumentation: true, HasTests: true, HasCIGate: true}
	r := EvaluateClaims(claims, ev)
	if r.Supported != 1 || r.Unsupported != 1 {
		t.Fatalf("expected 1+1, got %d+%d", r.Supported, r.Unsupported)
	}
}

func TestEvalEmpty(t *testing.T) {
	r := EvaluateClaims(nil, AQEvidenceLevel{})
	if r.TotalClaims != 0 || r.Blocking != 0 {
		t.Fatal("expected clean")
	}
}

func TestEvalRequiredLevelCertified(t *testing.T) {
	c := DetectedClaim{Text: "This is certified."}
	if matchRequiredLevel(c) != AQNQ3 {
		t.Fatal("certified should require NQ-3")
	}
}

func TestEvalRequiredLevelTested(t *testing.T) {
	c := DetectedClaim{Text: "This is tested."}
	if matchRequiredLevel(c) != AQNQ1 {
		t.Fatal("tested should require NQ-1")
	}
}

// --- JSON ---

func TestAQResultJSON(t *testing.T) {
	claims := []DetectedClaim{{Text: "Tested.", Kind: ClaimQuality, SourcePath: "d.md", Line: 5}}
	r := EvaluateClaims(claims, AQEvidenceLevel{HasDocumentation: true})
	var buf bytes.Buffer
	if err := WriteAQResultJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var decoded AQEvaluationResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TotalClaims != 1 {
		t.Fatal("roundtrip mismatch")
	}
}

// --- Constants ---

func TestAQLevelConstants(t *testing.T) {
	if AQNQ0 != "NQ-0" || AQNQ1 != "NQ-1" || AQNQ2 != "NQ-2" || AQNQ3 != "NQ-3" {
		t.Fatal("level constants wrong")
	}
}

func TestClaimKindConstants(t *testing.T) {
	if ClaimQuality != "quality" || ClaimCompliance != "compliance" ||
		ClaimValidation != "validation" || ClaimCertified != "certified" ||
		ClaimRegulated != "regulated" {
		t.Fatal("kind constants wrong")
	}
}

// --- helper ---

func assertLevel(t *testing.T, e AQEvidenceLevel, expected AQLevel) {
	t.Helper()
	if got := e.MaxSupportedLevel(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
