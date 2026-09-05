package attestation

// #645 — the issuance boundary fails closed: production is refused before any
// process runs, unknown hosts are refused, the issuer's word is checked, a
// refusal leaves no bundle behind, and the produced bundle is verified
// independently. Stub processes stand in for the external binary; the real
// issuance runs in tools/sigstore-verifier's tests and the CI gate.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEndpointPolicy(t *testing.T) {
	cases := []struct {
		url   string
		allow []string
		code  string
	}{
		{"https://fulcio.sigstore.dev", nil, CodeSigstoreProductionForbidden},
		{"https://rekor.sigstore.dev/api/v1", nil, CodeSigstoreProductionForbidden},
		{"https://fulcio.sigstage.dev", nil, CodeSigstoreProductionForbidden},
		{"https://FULCIO.SIGSTORE.DEV", nil, CodeSigstoreProductionForbidden},
		{"https://fulcio.sigstore.dev", []string{"fulcio.sigstore.dev"}, CodeSigstoreProductionForbidden}, // no allow-list overrides production
		{"https://fulcio.example.com", nil, CodeSigstoreEndpointNotAllowed},
		{"http://10.0.0.5:8080", nil, CodeSigstoreEndpointNotAllowed},
		{"ftp://localhost", nil, CodeSigstoreEndpointNotAllowed},
		{"not a url", nil, CodeSigstoreEndpointNotAllowed},
		{"http://127.0.0.1:4321", nil, ""},
		{"http://[::1]:4321", nil, ""},
		{"http://localhost:4321", nil, ""},
		{"https://fulcio.ci.nomos.invalid", nil, ""},
		{"https://fulcio.staging.test", nil, ""},
		{"https://fulcio.example.com", []string{"fulcio.example.com"}, ""},
	}
	for _, tc := range cases {
		err := CheckEndpointPolicy(tc.url, tc.allow)
		if tc.code == "" {
			if err != nil {
				t.Errorf("%s: unexpected refusal %v", tc.url, err)
			}
			continue
		}
		wantSigstoreCode(t, err, tc.code)
	}
}

func issueRequestFixture(t *testing.T) (SigstoreIssueRequest, string) {
	t.Helper()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "artifact.txt")
	os.WriteFile(artifact, []byte("payload\n"), 0o644)
	root := filepath.Join(dir, "trusted_root.json")
	os.WriteFile(root, []byte("{}"), 0o644)
	return SigstoreIssueRequest{
		ArtifactPath: artifact, OutBundlePath: filepath.Join(dir, "issued.json"), FulcioURL: "http://127.0.0.1:1", RekorURL: "http://127.0.0.1:2",
		IDToken: "eyJ.fixture.", TrustedRootPath: root, Identity: SigstoreIdentity{SAN: goodSAN, Issuer: goodIssuer},
	}, digestOf([]byte("payload\n"))
}

// stubIssuer writes a bundle file (when told) and prints a canned response.
func stubIssuer(t *testing.T, exit int, response string, writeBundle bool) []string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "issuer.sh")
	script := "#!/bin/sh\nreq=$(cat)\n"
	if writeBundle {
		script += `out=$(printf '%s' "$req" | sed -n 's/.*"out_bundle_path":"\([^"]*\)".*/\1/p')` + "\nprintf '{\"fake\":true}' > \"$out\"\n"
	}
	if response != "" {
		script += "cat <<'JSON'\n" + response + "\nJSON\n"
	}
	script += "exit " + itoa(exit) + "\n"
	os.WriteFile(p, []byte(script), 0o755)
	return []string{p}
}

func goodIssueResponse(req SigstoreIssueRequest, digest string) string {
	return `{"schema_version":"nomos.sigstore-issue.response.v1","verifier":{"name":"stub","version":"0","library":"stub","library_version":"0"},"non_production":true,"issued":true,"bundle_path":"` + req.OutBundlePath + `","artifact_digest":"` + digest + `","endpoints":{"fulcio":"` + req.FulcioURL + `","rekor":"` + req.RekorURL + `"},"certificate":{"subject_alternative_name":"` + goodSAN + `","issuer":"` + goodIssuer + `"},"claim_boundary":"stub"}`
}

func TestIssueHappyPathWithIndependentVerification(t *testing.T) {
	req, digest := issueRequestFixture(t)
	issuer := ExternalSigstoreIssuer{Command: stubIssuer(t, 0, goodIssueResponse(req, digest), true)}
	verifier := ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(digest))}
	rec, err := IssueSigstoreBundle(issuer, verifier, req, time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !rec.NonProduction || rec.ArtifactDigest != digest || rec.SignerSAN != goodSAN || !rec.IndependentCheck.Verified {
		t.Fatalf("record: %+v", rec)
	}
	if rec.Request.IDToken == req.IDToken || !strings.HasPrefix(rec.Request.IDToken, "<redacted:") {
		t.Fatalf("the id token must not be written into the record, got %q", rec.Request.IDToken)
	}
	if rec.IndependentCheck.Request.Require.SignedCertificateTimestamps != 0 {
		t.Fatal("the injected environment runs no CT log; the independent check must record SCT=0, not hide it")
	}
	if _, err := os.Stat(req.OutBundlePath); err != nil {
		t.Fatal("bundle must exist after a successful issuance")
	}
	if !strings.Contains(rec.ClaimBoundary, "No production service was contacted") {
		t.Fatalf("claim boundary must travel, got %q", rec.ClaimBoundary)
	}
}

func TestIssueFailsClosedAndLeavesNoBundle(t *testing.T) {
	req, digest := issueRequestFixture(t)
	other := "sha256:" + strings.Repeat("0", 64)
	okVerifier := ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(digest))}
	cases := []struct {
		name     string
		issuer   []string
		verifier SigstoreVerifier
		mut      func(r *SigstoreIssueRequest)
		code     string
		frag     string
	}{
		{"production fulcio refused before any process", stubIssuer(t, 0, goodIssueResponse(req, digest), true), okVerifier,
			func(r *SigstoreIssueRequest) { r.FulcioURL = "https://fulcio.sigstore.dev" }, CodeSigstoreProductionForbidden, "#638"},
		{"production rekor refused", stubIssuer(t, 0, goodIssueResponse(req, digest), true), okVerifier,
			func(r *SigstoreIssueRequest) { r.RekorURL = "https://rekor.sigstore.dev" }, CodeSigstoreProductionForbidden, "public instance"},
		{"unknown host refused", stubIssuer(t, 0, goodIssueResponse(req, digest), true), okVerifier,
			func(r *SigstoreIssueRequest) { r.FulcioURL = "https://fulcio.corp.example.com" }, CodeSigstoreEndpointNotAllowed, "allow-list"},
		{"no id token", stubIssuer(t, 0, goodIssueResponse(req, digest), true), okVerifier,
			func(r *SigstoreIssueRequest) { r.IDToken = "" }, CodeSigstoreIssueRefused, "never fetches"},
		{"issuer binary absent", []string{filepath.Join(t.TempDir(), "missing")}, okVerifier, nil, CodeSigstoreVerifierUnavailable, "no verdict"},
		{"issuer refuses (service down)", stubIssuer(t, 1, strings.Replace(strings.Replace(goodIssueResponse(req, digest), `"issued":true`, `"issued":false,"refusal":{"code":"SERVICE_UNAVAILABLE","message":"dial tcp: connection refused"}`, 1), `"bundle_path":"`+req.OutBundlePath+`",`, "", 1), false), okVerifier,
			nil, CodeSigstoreIssueRefused, "SERVICE_UNAVAILABLE"},
		{"issued=true but exit 1", stubIssuer(t, 1, goodIssueResponse(req, digest), true), okVerifier, nil, CodeSigstoreIssueBadResponse, "contradictory"},
		{"issued but not marked non-production", stubIssuer(t, 0, strings.Replace(goodIssueResponse(req, digest), `"non_production":true`, `"non_production":false`, 1), true), okVerifier,
			nil, CodeSigstoreProductionForbidden, "non_production"},
		{"issuer lies about the artifact", stubIssuer(t, 0, goodIssueResponse(req, other), true), okVerifier, nil, CodeSigstoreDigestDisagreement, "NOMOS computed"},
		{"issuer reports another signer", stubIssuer(t, 0, strings.Replace(goodIssueResponse(req, digest), goodSAN, "https://evil.invalid/x", 1), true), okVerifier,
			nil, CodeSigstoreIdentityDisagreement, "signer SAN"},
		{"issuer wrote no bundle", stubIssuer(t, 0, goodIssueResponse(req, digest), false), okVerifier, nil, CodeSigstoreIssueBadResponse, "unreadable"},
		{"independent verification refuses the bundle", stubIssuer(t, 0, goodIssueResponse(req, digest), true),
			ExternalSigstoreVerifier{Command: stubVerifier(t, 1, `{"schema_version":"nomos.sigstore-verify.response.v1","mode":"offline","verified":false,"refusal":{"code":"INCLUSION_INVALID","message":"bad proof"},"claim_boundary":""}`)},
			nil, CodeSigstoreIssueRefused, "independent verification"},
		{"independent verifier disagrees on digest", stubIssuer(t, 0, goodIssueResponse(req, digest), true),
			ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(other))}, nil, CodeSigstoreIssueRefused, "not the same artifact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := req
			if tc.mut != nil {
				tc.mut(&r)
			}
			_, err := IssueSigstoreBundle(ExternalSigstoreIssuer{Command: tc.issuer}, tc.verifier, r, time.Now())
			wantSigstoreCode(t, err, tc.code)
			if !strings.Contains(err.Error(), tc.frag) {
				t.Fatalf("message must name %q, got %q", tc.frag, err.Error())
			}
			if _, statErr := os.Stat(r.OutBundlePath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatal("a refused issuance must leave no bundle behind")
			}
		})
	}
}
