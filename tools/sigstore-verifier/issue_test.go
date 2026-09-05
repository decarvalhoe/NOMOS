package main

// #645 — issuance against in-process fixture services: the happy path issues
// and verifies; every refusal is named; production and unknown hosts are
// refused before any request; a stopped service leaves no bundle behind.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RBOKproject/Nomos/tools/sigstore-verifier/fixtureservices"
)

func startFixture(t *testing.T) (*fixtureservices.Services, string, string) {
	t.Helper()
	svc, err := fixtureservices.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	dir := t.TempDir()
	if _, err := svc.WriteMaterial(dir, "fixture-signer@nomos.invalid"); err != nil {
		t.Fatal(err)
	}
	return svc, filepath.Join(dir, "trusted_root.json"), filepath.Join(dir, "id_token")
}

func issueFixtureRequest(t *testing.T, svc *fixtureservices.Services, root, tokenPath string) IssueRequest {
	t.Helper()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "artifact.txt")
	os.WriteFile(artifact, []byte("issued against the fixture\n"), 0o644)
	token, _ := os.ReadFile(tokenPath)
	return IssueRequest{
		SchemaVersion: IssueRequestSchema, ArtifactPath: artifact, OutBundlePath: filepath.Join(dir, "issued.sigstore.json"),
		FulcioURL: svc.FulcioURL, RekorURL: svc.RekorURL, IDToken: string(token), TrustedRootPath: root,
		Identity: Identity{SAN: "fixture-signer@nomos.invalid", Issuer: fixtureservices.FixtureIssuer},
	}
}

func runIssue(t *testing.T, req IssueRequest) (IssueResponse, int) {
	t.Helper()
	raw, _ := json.Marshal(req)
	var stdout, stderr bytes.Buffer
	code := run(bytes.NewReader(raw), &stdout, &stderr)
	var resp IssueResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("issue response is not JSON (exit %d): %v\n%s\n%s", code, err, stdout.String(), stderr.String())
	}
	return resp, code
}

func TestIssueAgainstInjectedServicesThenVerify(t *testing.T) {
	svc, root, token := startFixture(t)
	req := issueFixtureRequest(t, svc, root, token)
	resp, code := runIssue(t, req)
	if code != 0 || !resp.Issued {
		t.Fatalf("issuance refused: exit %d %+v", code, resp.Refusal)
	}
	if !resp.NonProduction || resp.BundlePath != req.OutBundlePath || resp.Certificate == nil || resp.Certificate.SubjectAlternativeName != "fixture-signer@nomos.invalid" {
		t.Fatalf("response: %+v", resp)
	}
	if resp.Verification == nil || !resp.Verification.Verified || len(resp.Verification.TlogEntries) != 1 || !resp.Verification.TlogEntries[0].HasInclusionPromise {
		t.Fatalf("issued bundle must be verified with a transparency-log promise: %+v", resp.Verification)
	}
	if _, err := os.Stat(req.OutBundlePath); err != nil {
		t.Fatal("bundle file must exist")
	}
	served := strings.Join(svc.Served(), " ")
	if !strings.Contains(served, "fulcio:signingCert") || !strings.Contains(served, "rekor:createLogEntry") {
		t.Fatalf("both fixture services must have been used, served=%q", served)
	}
	// Independent verification through the verify mode with the fixture root.
	v, vcode := runRequest(t, Request{
		SchemaVersion: RequestSchema, BundlePath: req.OutBundlePath, TrustedRootPath: root, ArtifactPath: req.ArtifactPath,
		CertificateIdentity: req.Identity, Require: Require{TlogEntries: 1, ObserverTimestamps: 1, NoCTLog: true},
	})
	if vcode != 0 || !v.Verified {
		t.Fatalf("independent verify refused: %+v", v.Refusal)
	}
	// Without the explicit no-CT-log decision the default SCT threshold applies and the fixture bundle is refused.
	v2, vcode2 := runRequest(t, Request{
		SchemaVersion: RequestSchema, BundlePath: req.OutBundlePath, TrustedRootPath: root, ArtifactPath: req.ArtifactPath,
		CertificateIdentity: req.Identity, Require: Require{TlogEntries: 1, ObserverTimestamps: 1},
	})
	if vcode2 != 1 || v2.Verified || v2.Refusal == nil || v2.Refusal.Code != "CERTIFICATE_INVALID" {
		t.Fatalf("an omitted no_ct_log must keep the SCT requirement: %+v", v2.Refusal)
	}
	// Tamper the issued bundle: the signature byte.
	raw, _ := os.ReadFile(req.OutBundlePath)
	var b map[string]any
	json.Unmarshal(raw, &b)
	ms := b["messageSignature"].(map[string]any)
	ms["signature"] = flipBase64(ms["signature"].(string))
	tampered := filepath.Join(t.TempDir(), "tampered.json")
	out, _ := json.Marshal(b)
	os.WriteFile(tampered, out, 0o644)
	v3, vcode3 := runRequest(t, Request{
		SchemaVersion: RequestSchema, BundlePath: tampered, TrustedRootPath: root, ArtifactPath: req.ArtifactPath,
		CertificateIdentity: req.Identity, Require: Require{TlogEntries: 1, ObserverTimestamps: 1, NoCTLog: true},
	})
	if vcode3 != 1 || v3.Verified {
		t.Fatal("tampered issued bundle verified")
	}
	// The fixture's own trust material does not verify the public-good fixture bundle, and vice versa.
	v4, _ := runRequest(t, Request{
		SchemaVersion: RequestSchema, BundlePath: req.OutBundlePath, TrustedRootPath: fixtureRoot, ArtifactPath: req.ArtifactPath,
		CertificateIdentity: req.Identity, Require: Require{TlogEntries: 1, ObserverTimestamps: 1, NoCTLog: true},
	})
	if v4.Verified {
		t.Fatal("a fixture-issued bundle must not verify against the public-good root")
	}
}

func TestIssueRefusals(t *testing.T) {
	svc, root, token := startFixture(t)
	base := issueFixtureRequest(t, svc, root, token)
	cases := []struct {
		name string
		mut  func(r *IssueRequest)
		code string
	}{
		{"production fulcio", func(r *IssueRequest) { r.FulcioURL = "https://fulcio.sigstore.dev" }, "PRODUCTION_FORBIDDEN"},
		{"production rekor", func(r *IssueRequest) { r.RekorURL = "https://rekor.sigstore.dev" }, "PRODUCTION_FORBIDDEN"},
		{"staging is a public instance", func(r *IssueRequest) { r.FulcioURL = "https://fulcio.sigstage.dev" }, "PRODUCTION_FORBIDDEN"},
		{"allow-list cannot unlock production", func(r *IssueRequest) {
			r.FulcioURL = "https://fulcio.sigstore.dev"
			r.AllowHosts = []string{"fulcio.sigstore.dev"}
		}, "PRODUCTION_FORBIDDEN"},
		{"unlisted host", func(r *IssueRequest) { r.FulcioURL = "https://fulcio.corp.example.com" }, "ENDPOINT_NOT_ALLOWED"},
		{"non-loopback ip", func(r *IssueRequest) { r.RekorURL = "http://10.1.2.3:3000" }, "ENDPOINT_NOT_ALLOWED"},
		{"no token", func(r *IssueRequest) { r.IDToken = "" }, "REQUEST_INCOMPLETE"},
		{"no identity", func(r *IssueRequest) { r.Identity = Identity{} }, "REQUEST_INCOMPLETE"},
		{"wrong expected identity", func(r *IssueRequest) { r.Identity.SAN = "someone-else@nomos.invalid" }, "ISSUED_BUNDLE_UNVERIFIABLE"},
		{"wrong expected issuer", func(r *IssueRequest) { r.Identity.Issuer = "https://accounts.google.com" }, "ISSUED_BUNDLE_UNVERIFIABLE"},
		{"token for another issuer is refused by the fixture fulcio", func(r *IssueRequest) {
			r.IDToken = strings.Replace(fixtureservices.FixtureIDToken("x@nomos.invalid", time.Now().Add(time.Hour)), base64.RawURLEncoding.EncodeToString([]byte(`{"`)), base64.RawURLEncoding.EncodeToString([]byte(`{"`)), 1)
			// forge a payload with a different issuer
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"https://evil.invalid","sub":"x@nomos.invalid","email":"x@nomos.invalid"}`))
			r.IDToken = "eyJhbGciOiJub25lIn0." + payload + "."
		}, "CERTIFICATE_ISSUANCE_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.OutBundlePath = filepath.Join(t.TempDir(), "issued.json")
			tc.mut(&req)
			resp, code := runIssue(t, req)
			if code != 1 || resp.Issued {
				t.Fatalf("accepted: exit %d", code)
			}
			if resp.Refusal == nil || resp.Refusal.Code != tc.code {
				t.Fatalf("want %s, got %+v", tc.code, resp.Refusal)
			}
			if _, err := os.Stat(req.OutBundlePath); !os.IsNotExist(err) {
				t.Fatal("a refused issuance must leave no bundle")
			}
		})
	}
}

func TestIssueWithServicesDownLeavesNoBundle(t *testing.T) {
	svc, root, token := startFixture(t)
	req := issueFixtureRequest(t, svc, root, token)
	_ = svc.Close()
	resp, code := runIssue(t, req)
	if code != 1 || resp.Issued || resp.Refusal == nil {
		t.Fatalf("issuance must be refused with the services down: %d %+v", code, resp)
	}
	if resp.Refusal.Code != "SERVICE_UNAVAILABLE" && resp.Refusal.Code != "CERTIFICATE_ISSUANCE_FAILED" {
		t.Fatalf("refusal code %s (%s)", resp.Refusal.Code, resp.Refusal.Message)
	}
	if _, err := os.Stat(req.OutBundlePath); !os.IsNotExist(err) {
		t.Fatal("no partial bundle may survive a service outage")
	}
}
