package attestation

// #637 — the boundary fails closed. Stub verifiers (shell scripts) exercise
// every way the process side can lie or be absent; the real verifier is
// exercised end to end by tools/sigstore-verifier's own tests and the CI gate.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const goodSAN = "https://example.invalid/ci/release.yml@refs/heads/main"
const goodIssuer = "https://oidc.example.invalid"

func stubVerifier(t *testing.T, exit int, stdout string) []string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "verifier.sh")
	script := "#!/bin/sh\ncat >/dev/null\n"
	if stdout != "" {
		script += "cat <<'JSON'\n" + stdout + "\nJSON\n"
	}
	script += "exit " + itoa(exit) + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{p}
}

func itoa(i int) string { return strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+i))) }

func artifactFixture(t *testing.T) (path, digest string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "artifact.txt")
	data := []byte("artifact bytes\n")
	os.WriteFile(path, data, 0o644)
	sum := sha256.Sum256(data)
	return path, "sha256:" + hex.EncodeToString(sum[:])
}

func goodResponse(digest string) string {
	return `{"schema_version":"nomos.sigstore-verify.response.v1","verifier":{"name":"stub","version":"0","library":"stub-lib","library_version":"0"},"mode":"offline","verified":true,"media_type":"application/vnd.dev.sigstore.bundle+json;version=0.1","artifact_digest":"` + digest + `","certificate":{"subject_alternative_name":"` + goodSAN + `","issuer":"` + goodIssuer + `"},"timestamps":[{"type":"Tlog","uri":"https://rekor.example.invalid","timestamp":"2026-09-05T00:00:00Z"}],"tlog_entries":[{"log_index":1,"log_key_id":"ab","integrated_time":"2026-09-05T00:00:00Z","has_inclusion_proof":true,"has_inclusion_promise":false}],"claim_boundary":"stub"}`
}

func request(artifact string) SigstoreRequest {
	return SigstoreRequest{BundlePath: "b.json", TrustedRootPath: "r.json", ArtifactPath: artifact,
		CertificateIdentity: SigstoreIdentity{SAN: goodSAN, Issuer: goodIssuer}, Require: SigstoreRequire{TlogEntries: 1}}
}

func wantSigstoreCode(t *testing.T, err error, code string) {
	t.Helper()
	var se *SigstoreError
	if !errors.As(err, &se) {
		t.Fatalf("want %s, got %v", code, err)
	}
	if se.Code != code {
		t.Fatalf("want %s, got %s (%s)", code, se.Code, se.Message)
	}
}

func TestSigstoreHappyPathRecordBindsResponse(t *testing.T) {
	artifact, digest := artifactFixture(t)
	v := ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(digest))}
	rec, raw, err := VerifySigstoreBundle(v, request(artifact), time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Verified || rec.ArtifactDigest != digest || rec.SignerSAN != goodSAN || rec.SignerIssuer != goodIssuer || rec.TlogEntries != 1 {
		t.Fatalf("record: %+v", rec)
	}
	if rec.Request.SchemaVersion != SigstoreRequestSchema {
		t.Fatal("request schema not stamped")
	}
	if err := VerifySigstoreRecordResponse(rec, raw); err != nil {
		t.Fatalf("record must bind the exact response bytes: %v", err)
	}
	if err := VerifySigstoreRecordResponse(rec, append([]byte{}, raw[:len(raw)-1]...)); err == nil {
		t.Fatal("altered response bytes must not match the record")
	}
	if !strings.Contains(rec.ClaimBoundary, "did not sign") {
		t.Fatalf("claim boundary must say what was not done, got %q", rec.ClaimBoundary)
	}
}

func TestSigstoreBoundaryFailsClosed(t *testing.T) {
	artifact, digest := artifactFixture(t)
	other := "sha256:" + strings.Repeat("0", 64)
	cases := []struct {
		name string
		cmd  []string
		req  SigstoreRequest
		code string
		frag string
	}{
		{"verifier binary absent", []string{filepath.Join(t.TempDir(), "missing")}, request(artifact), CodeSigstoreVerifierUnavailable, "no verdict"},
		{"verifier exits 1 with refusal", stubVerifier(t, 1, `{"schema_version":"nomos.sigstore-verify.response.v1","mode":"offline","verified":false,"refusal":{"code":"IDENTITY_MISMATCH","message":"san differs"},"claim_boundary":""}`), request(artifact), CodeSigstoreRefused, "IDENTITY_MISMATCH"},
		{"verifier exits 1 silently", stubVerifier(t, 1, ""), request(artifact), CodeSigstoreVerifierFailed, "without a response"},
		{"verifier writes garbage", stubVerifier(t, 0, "not json at all"), request(artifact), CodeSigstoreBadResponse, "not nomos.sigstore-verify.response.v1"},
		{"verifier speaks another schema", stubVerifier(t, 0, strings.Replace(goodResponse(digest), "response.v1", "response.v9", 1)), request(artifact), CodeSigstoreBadResponse, "response.v9"},
		{"verified=true but exit 1", stubVerifier(t, 1, goodResponse(digest)), request(artifact), CodeSigstoreBadResponse, "contradictory"},
		{"verified=true but for another artifact", stubVerifier(t, 0, goodResponse(other)), request(artifact), CodeSigstoreDigestDisagreement, "not the same artifact"},
		{"verified=true but another signer", stubVerifier(t, 0, strings.Replace(goodResponse(digest), goodSAN, "https://evil.invalid/x", 1)), request(artifact), CodeSigstoreIdentityDisagreement, "signer SAN"},
		{"verified=true but another issuer", stubVerifier(t, 0, strings.Replace(goodResponse(digest), goodIssuer, "https://other-issuer.invalid", 1)), request(artifact), CodeSigstoreIdentityDisagreement, "signer issuer"},
		{"verified=true but no certificate", stubVerifier(t, 0, strings.Replace(goodResponse(digest), `"certificate":{"subject_alternative_name":"`+goodSAN+`","issuer":"`+goodIssuer+`"},`, "", 1)), request(artifact), CodeSigstoreIdentityDisagreement, "no signing certificate"},
		{"verified=true but online mode", stubVerifier(t, 0, strings.Replace(goodResponse(digest), `"mode":"offline"`, `"mode":"online"`, 1)), request(artifact), CodeSigstoreBadResponse, "only offline"},
		{"verified=true but no tlog entries", stubVerifier(t, 0, strings.Replace(goodResponse(digest), `"tlog_entries":[{"log_index":1,"log_key_id":"ab","integrated_time":"2026-09-05T00:00:00Z","has_inclusion_proof":true,"has_inclusion_promise":false}],`, "", 1)), request(artifact), CodeSigstoreRefused, "transparency-log"},
		{"verified=false without refusal", stubVerifier(t, 1, strings.Replace(goodResponse(digest), `"verified":true`, `"verified":false`, 1)), request(artifact), CodeSigstoreRefused, "without a refusal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := VerifySigstoreBundle(ExternalSigstoreVerifier{Command: tc.cmd}, tc.req, time.Now())
			wantSigstoreCode(t, err, tc.code)
			if !strings.Contains(err.Error(), tc.frag) {
				t.Fatalf("message must name %q, got %q", tc.frag, err.Error())
			}
		})
	}
}

func TestSigstoreRequestShapeIsEnforcedBeforeAnyProcess(t *testing.T) {
	artifact, _ := artifactFixture(t)
	never := []string{filepath.Join(t.TempDir(), "must-not-run")}
	cases := []struct {
		name string
		mut  func(r *SigstoreRequest)
		frag string
	}{
		{"no identity", func(r *SigstoreRequest) { r.CertificateIdentity.SAN = "" }, "signer identity"},
		{"no issuer", func(r *SigstoreRequest) { r.CertificateIdentity.Issuer = "" }, "issuer"},
		{"both artifact and digest", func(r *SigstoreRequest) { r.ArtifactDigest = "sha256:" + strings.Repeat("a", 64) }, "exactly one"},
		{"neither artifact nor digest", func(r *SigstoreRequest) { r.ArtifactPath = "" }, "exactly one"},
		{"malformed digest", func(r *SigstoreRequest) { r.ArtifactPath = ""; r.ArtifactDigest = "md5:abcd" }, "sha256|sha384|sha512"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := request(artifact)
			tc.mut(&req)
			_, _, err := VerifySigstoreBundle(ExternalSigstoreVerifier{Command: never}, req, time.Now())
			wantSigstoreCode(t, err, CodeSigstoreRefused)
			if !strings.Contains(err.Error(), tc.frag) {
				t.Fatalf("message must name %q, got %q", tc.frag, err.Error())
			}
		})
	}
}

func TestSigstoreIdentityRegexes(t *testing.T) {
	artifact, digest := artifactFixture(t)
	v := ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(digest))}
	req := request(artifact)
	req.CertificateIdentity = SigstoreIdentity{SANRegex: `^https://example\.invalid/ci/.*@refs/heads/main$`, IssuerRegex: `^https://oidc\.example\.invalid$`}
	if _, _, err := VerifySigstoreBundle(v, req, time.Now()); err != nil {
		t.Fatalf("matching regexes must pass: %v", err)
	}
	req.CertificateIdentity = SigstoreIdentity{SANRegex: `^https://example\.invalid/ci/.*@refs/tags/`, IssuerRegex: `.*`}
	_, _, err := VerifySigstoreBundle(v, req, time.Now())
	wantSigstoreCode(t, err, CodeSigstoreIdentityDisagreement)
}

func TestSigstoreDigestSuppliedNotFile(t *testing.T) {
	sha512 := "sha512:" + strings.Repeat("ab", 64)
	v := ExternalSigstoreVerifier{Command: stubVerifier(t, 0, goodResponse(sha512))}
	req := request("")
	req.ArtifactDigest = strings.ToUpper(sha512[:6]) + sha512[6:] // algorithm case-insensitive
	rec, _, err := VerifySigstoreBundle(v, req, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ArtifactDigest != sha512 {
		t.Fatalf("digest normalised, got %s", rec.ArtifactDigest)
	}
}

func TestResolveSigstoreVerifierFailsClosedWhenNothingIsAvailable(t *testing.T) {
	t.Setenv(SigstoreVerifierEnv, "")
	t.Setenv("PATH", t.TempDir())
	_, err := ResolveSigstoreVerifier("")
	wantSigstoreCode(t, err, CodeSigstoreVerifierUnavailable)
	if !strings.Contains(err.Error(), "No verdict") {
		t.Fatalf("absence must say no verdict, got %q", err.Error())
	}
	cmd, err := ResolveSigstoreVerifier("/x/y verifier --flag")
	if err != nil || len(cmd) != 3 {
		t.Fatalf("explicit command splits into fields, got %v %v", cmd, err)
	}
	t.Setenv(SigstoreVerifierEnv, "/env/verifier")
	cmd, err = ResolveSigstoreVerifier("")
	if err != nil || cmd[0] != "/env/verifier" {
		t.Fatalf("env fallback, got %v %v", cmd, err)
	}
}

func TestSigstoreResponseRoundTripsJSON(t *testing.T) {
	var resp SigstoreResponse
	if err := json.Unmarshal([]byte(goodResponse("sha256:"+strings.Repeat("0", 64))), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Certificate == nil || resp.Certificate.SubjectAlternativeName != goodSAN || len(resp.TlogEntries) != 1 {
		t.Fatalf("decoded: %+v", resp)
	}
}
