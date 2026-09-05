package compliance

// NRT-016 (#660) — every rule of the exchange verifier has a test that breaks
// exactly it; the committed fixtures are verified against the real tree.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoRootFromPkg = "../../.."

func loadFixture(t *testing.T, name string) PraxisEvidenceExchange {
	t.Helper()
	ex, err := LoadPraxisExchange(filepath.Join(repoRootFromPkg, "specs", "examples", name))
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func wantPraxis(t *testing.T, err error, code, frag string) {
	t.Helper()
	var pe *PraxisError
	if !errors.As(err, &pe) {
		t.Fatalf("want %s, got %v", code, err)
	}
	if pe.Code != code {
		t.Fatalf("want %s, got %s (%s)", code, pe.Code, pe.Message)
	}
	if !strings.Contains(err.Error(), frag) {
		t.Fatalf("message must name %q, got %q", frag, err.Error())
	}
}

func TestPraxisValidFixtureVerifiesAgainstTree(t *testing.T) {
	ex := loadFixture(t, "nomos-praxis-evidence.valid.yaml")
	rep, err := VerifyPraxisExchange(ex, repoRootFromPkg)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.HashesChecked || rep.Artifacts != 2 || rep.VerifiedArtifacts != 0 || rep.Scenarios != 2 || rep.Findings != 1 || rep.Capa != 1 {
		t.Fatalf("report: %+v", rep)
	}
	if !strings.HasPrefix(rep.ExchangeDigest, "sha256:") || !strings.Contains(rep.ClaimBoundary, "does not activate Praxis") {
		t.Fatalf("digest/claim boundary: %+v", rep)
	}
}

func TestPraxisNegativeFixtures(t *testing.T) {
	cases := map[string][2]string{
		"nomos-praxis-evidence.invalid-regulated-unverified.yaml": {CodePraxisRelianceUnsupported, "0 of 1"},
		"nomos-praxis-evidence.invalid-praxis-producer.yaml":      {CodePraxisAuthority, "producer=\"praxis\""},
	}
	for name, want := range cases {
		_, err := VerifyPraxisExchange(loadFixture(t, name), "")
		wantPraxis(t, err, want[0], want[1])
	}
}

func TestPraxisUnknownFieldIsRefusedAtLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	raw, _ := os.ReadFile(filepath.Join(repoRootFromPkg, "specs", "examples", "nomos-praxis-evidence.valid.yaml"))
	os.WriteFile(p, append(raw, []byte("\nblock_id: B-INTERNAL\n")...), 0o644)
	_, err := LoadPraxisExchange(p)
	wantPraxis(t, err, CodePraxisShape, "block_id")
}

func TestPraxisRules(t *testing.T) {
	base := loadFixture(t, "nomos-praxis-evidence.valid.yaml")
	cases := []struct {
		name string
		mut  func(ex *PraxisEvidenceExchange)
		code string
		frag string
	}{
		{"schema", func(ex *PraxisEvidenceExchange) { ex.SchemaVersion = "v0" }, CodePraxisSchema, "schema_version"},
		{"bad exchange id", func(ex *PraxisEvidenceExchange) { ex.ExchangeID = "" }, CodePraxisShape, "exchange_id"},
		{"bad timestamp", func(ex *PraxisEvidenceExchange) { ex.GeneratedAt = "2026-09-05 12:00" }, CodePraxisShape, "generated_at"},
		{"consumer is not praxis", func(ex *PraxisEvidenceExchange) { ex.Consumer.Product = "nomos" }, CodePraxisAuthority, "consumer"},
		{"no artifacts", func(ex *PraxisEvidenceExchange) { ex.NomosArtifacts = nil }, CodePraxisShape, "without Nomos artifacts"},
		{"thin claim boundary", func(ex *PraxisEvidenceExchange) { ex.ClaimBoundary = "ok" }, CodePraxisShape, "claim_boundary"},
		{"duplicate artifact id", func(ex *PraxisEvidenceExchange) { ex.NomosArtifacts[1].ArtifactID = ex.NomosArtifacts[0].ArtifactID }, CodePraxisShape, "duplicated"},
		{"unknown artifact kind", func(ex *PraxisEvidenceExchange) { ex.NomosArtifacts[0].Kind = "praxis_report" }, CodePraxisShape, "kind"},
		{"bad artifact sha", func(ex *PraxisEvidenceExchange) { ex.NomosArtifacts[0].Sha256 = "sha256:abc" }, CodePraxisShape, "sha256"},
		{"verified without record", func(ex *PraxisEvidenceExchange) {
			ex.NomosArtifacts[0].Verification = PraxisVerification{State: "verified"}
		}, CodePraxisShape, "verification record"},
		{"unknown verification state", func(ex *PraxisEvidenceExchange) { ex.NomosArtifacts[0].Verification.State = "trusted" }, CodePraxisShape, "verification.state"},
		{"scenario bad result", func(ex *PraxisEvidenceExchange) { ex.PraxisScenarios[0].Result = "ok" }, CodePraxisShape, "result"},
		{"scenario duplicate", func(ex *PraxisEvidenceExchange) { ex.PraxisScenarios[1].ScenarioID = ex.PraxisScenarios[0].ScenarioID }, CodePraxisShape, "duplicated"},
		{"finding dangling scenario", func(ex *PraxisEvidenceExchange) { ex.RuntimeFindings[0].ScenarioID = "PRX-SCN-999" }, CodePraxisReference, "PRX-SCN-999"},
		{"finding dangling capa", func(ex *PraxisEvidenceExchange) { ex.RuntimeFindings[0].CapaID = "PRX-CAPA-999" }, CodePraxisReference, "PRX-CAPA-999"},
		{"capa without findings", func(ex *PraxisEvidenceExchange) { ex.Capa[0].FindingIDs = nil }, CodePraxisShape, "no cause"},
		{"capa dangling finding", func(ex *PraxisEvidenceExchange) { ex.Capa[0].FindingIDs = []string{"PRX-FND-999"} }, CodePraxisReference, "PRX-FND-999"},
		{"closed capa without closed_at", func(ex *PraxisEvidenceExchange) { ex.Capa[0].Status = "closed" }, CodePraxisShape, "closed_at"},
		{"unknown reliance", func(ex *PraxisEvidenceExchange) { ex.Reliance = "trusted" }, CodePraxisShape, "reliance"},
		{"not-qualified with a verdict", func(ex *PraxisEvidenceExchange) { ex.ActivationVerdictPath = "x" }, CodePraxisShape, "no activation verdict"},
		{"regulated with unverified artifact", func(ex *PraxisEvidenceExchange) { ex.Reliance = "regulated_evidence" }, CodePraxisRelianceUnsupported, "every Nomos artifact verified"},
		{"regulated verified but no verdict", func(ex *PraxisEvidenceExchange) {
			ex.Reliance = "regulated_evidence"
			for i := range ex.NomosArtifacts {
				ex.NomosArtifacts[i].Verification = PraxisVerification{State: "verified", RecordPath: "r.json", RecordSha256: "sha256:" + strings.Repeat("0", 64)}
			}
		}, CodePraxisRelianceUnsupported, "activation verdict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := loadFixture(t, "nomos-praxis-evidence.valid.yaml")
			_ = base
			tc.mut(&ex)
			_, err := VerifyPraxisExchange(ex, "")
			wantPraxis(t, err, tc.code, tc.frag)
		})
	}
}

func TestPraxisTreeHashes(t *testing.T) {
	root := t.TempDir()
	ex := loadFixture(t, "nomos-praxis-evidence.valid.yaml")
	for _, a := range ex.NomosArtifacts {
		src, _ := os.ReadFile(filepath.Join(repoRootFromPkg, filepath.FromSlash(a.Path)))
		dst := filepath.Join(root, filepath.FromSlash(a.Path))
		os.MkdirAll(filepath.Dir(dst), 0o755)
		os.WriteFile(dst, src, 0o644)
	}
	if _, err := VerifyPraxisExchange(ex, root); err != nil {
		t.Fatal(err)
	}
	// Artifact changed after the exchange was written.
	p := filepath.Join(root, filepath.FromSlash(ex.NomosArtifacts[0].Path))
	os.WriteFile(p, []byte("changed\n"), 0o644)
	_, err := VerifyPraxisExchange(ex, root)
	wantPraxis(t, err, CodePraxisArtifactHash, ex.NomosArtifacts[0].ArtifactID)
	// Artifact missing.
	os.Remove(p)
	_, err = VerifyPraxisExchange(ex, root)
	wantPraxis(t, err, CodePraxisArtifactMissing, ex.NomosArtifacts[0].ArtifactID)
	// A verified artifact whose record does not match its recorded hash.
	os.WriteFile(p, []byte("changed\n"), 0o644)
	ex.NomosArtifacts[0].Sha256 = "sha256:" + sha256Hex([]byte("changed\n"))[7:]
	ex.NomosArtifacts[0].Verification = PraxisVerification{State: "verified", RecordPath: "record.json", RecordSha256: "sha256:" + strings.Repeat("1", 64)}
	os.WriteFile(filepath.Join(root, "record.json"), []byte("{}"), 0o644)
	_, err = VerifyPraxisExchange(ex, root)
	wantPraxis(t, err, CodePraxisRecordHash, "record.json")
}
