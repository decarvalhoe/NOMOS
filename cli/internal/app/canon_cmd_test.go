package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VRC-11 (#557) — the `canon validate` CLI: passes a valid promotion, fails
// closed (exit 1) on a user_promoted atom claiming certified.

const validPromotion = `source:
  source_id: SRC-1
  access_policy: customer_source
  silo_id: SILO-1
shared_catalog:
  exported_source_ids: []
silo_catalog:
  source_ids: [SRC-1]
certificates:
  - cert_id: CERT-1
    revoked: false
atoms:
  - atom_id: AT-1
    review_state: approved
    metadata:
      facets:
        provenance: user_promoted
        trust_tier: indicative
        confidentiality: internal
      canon_promotion:
        certificate_id: CERT-1
        source_id: SRC-1
        silo_id: SILO-1
`

func writeBundle(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "promotion.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCanonValidate_PassesValidPromotion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"canon", "validate", "--bundle", writeBundle(t, validPromotion)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("valid promotion must pass, got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "pass"`) {
		t.Fatalf("expected pass status: %s", stdout.String())
	}
}

func TestCanonValidate_FailsClosedOnCertifiedClaim(t *testing.T) {
	bad := strings.Replace(validPromotion, "trust_tier: indicative", "trust_tier: certified", 1)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"canon", "validate", "--bundle", writeBundle(t, bad)}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("a certified user_promoted atom must exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "user_promoted_cannot_be_certified") {
		t.Fatalf("expected the certified-claim finding: %s", stdout.String())
	}
}

func TestCanonValidate_RequiresBundle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"canon", "validate"}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing --bundle must exit 2, got %d", code)
	}
}
