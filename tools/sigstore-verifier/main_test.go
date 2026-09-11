package main

// The proof is the refusal: a REAL public-good bundle verifies offline, and
// each way of tampering with the artifact digest, the identity, the bundle
// bytes (signature, certificate, inclusion promise) or the trust material is
// refused with a named code and exit 1.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fixtureBundle = "testdata/bundle-provenance.sigstore.json"
	fixtureRoot   = "testdata/trusted-root-public-good.json"
	fixtureDigest = "sha512:76176ffa33808b54602c7c35de5c6e9a4deb96066dba6533f50ac234f4f1f4c6b3527515dc17c06fbe2860030f410eee69ea20079bd3a2c6f3dcf3b329b10751"
	fixtureSAN    = "https://github.com/sigstore/sigstore-js/.github/workflows/release.yml@refs/heads/main"
	fixtureIssuer = "https://token.actions.githubusercontent.com"
)

func goodRequest() Request {
	return Request{
		SchemaVersion: RequestSchema, BundlePath: fixtureBundle, TrustedRootPath: fixtureRoot, ArtifactDigest: fixtureDigest,
		CertificateIdentity: Identity{SAN: fixtureSAN, Issuer: fixtureIssuer},
		Require:             Require{TlogEntries: 1, SignedCertificateTimestamps: 1, ObserverTimestamps: 1},
	}
}

func runRequest(t *testing.T, req Request) (Response, int) {
	t.Helper()
	raw, _ := json.Marshal(req)
	var stdout, stderr bytes.Buffer
	code := run(bytes.NewReader(raw), &stdout, &stderr)
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON (exit %d): %v\nstdout=%s\nstderr=%s", code, err, stdout.String(), stderr.String())
	}
	return resp, code
}

// tamperedBundle writes a copy of the fixture bundle with edit applied to its JSON.
func tamperedBundle(t *testing.T, edit func(b map[string]any)) string {
	t.Helper()
	raw, err := os.ReadFile(fixtureBundle)
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	edit(b)
	out, _ := json.Marshal(b)
	p := filepath.Join(t.TempDir(), "bundle.json")
	os.WriteFile(p, out, 0o644)
	return p
}

// flipBase64 decodes, flips one byte in the middle, and re-encodes — so the
// DECODED bytes differ (changing a trailing base64 character can leave the
// decoded bytes identical, which is not a tamper).
func flipBase64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	b[len(b)/2] ^= 0x01
	return base64.StdEncoding.EncodeToString(b)
}

func TestRealBundleVerifiesOffline(t *testing.T) {
	resp, code := runRequest(t, goodRequest())
	if code != 0 || !resp.Verified || resp.Refusal != nil {
		t.Fatalf("expected verified, got exit %d, %+v", code, resp.Refusal)
	}
	if resp.Mode != "offline" || resp.SchemaVersion != ResponseSchema {
		t.Fatalf("mode/schema: %+v", resp)
	}
	if resp.ArtifactDigest != fixtureDigest {
		t.Fatalf("artifact digest %s", resp.ArtifactDigest)
	}
	if resp.Certificate == nil || resp.Certificate.SubjectAlternativeName != fixtureSAN || resp.Certificate.Issuer != fixtureIssuer {
		t.Fatalf("certificate: %+v", resp.Certificate)
	}
	if len(resp.TlogEntries) != 1 || len(resp.Timestamps) < 1 {
		t.Fatalf("tlog/timestamps: %+v %+v", resp.TlogEntries, resp.Timestamps)
	}
	if !strings.Contains(resp.ClaimBoundary, "Nothing was signed") {
		t.Fatalf("claim boundary must travel, got %q", resp.ClaimBoundary)
	}
}

func TestTamperIsRefused(t *testing.T) {
	sigPath := func(b map[string]any) map[string]any { return b["dsseEnvelope"].(map[string]any) }
	cases := []struct {
		name  string
		mut   func(r *Request)
		codes []string
	}{
		{"wrong artifact digest", func(r *Request) {
			r.ArtifactDigest = "sha512:" + strings.Repeat("0", 128)
		}, []string{"ARTIFACT_MISMATCH"}},
		{"wrong signer identity", func(r *Request) {
			r.CertificateIdentity.SAN = "https://github.com/evil/repo/.github/workflows/release.yml@refs/heads/main"
		}, []string{"IDENTITY_MISMATCH"}},
		{"wrong issuer", func(r *Request) {
			r.CertificateIdentity.Issuer = "https://accounts.google.com"
		}, []string{"IDENTITY_MISMATCH"}},
		{"signature byte flipped", func(r *Request) {
			r.BundlePath = tamperedBundle(t, func(b map[string]any) {
				sigs := sigPath(b)["signatures"].([]any)
				s0 := sigs[0].(map[string]any)
				s0["sig"] = flipBase64(s0["sig"].(string))
			})
		}, []string{"SIGNATURE_INVALID", "VERIFICATION_FAILED", "INCLUSION_INVALID"}},
		{"payload byte flipped", func(r *Request) {
			r.BundlePath = tamperedBundle(t, func(b map[string]any) {
				env := sigPath(b)
				env["payload"] = flipBase64(env["payload"].(string))
			})
		}, []string{"SIGNATURE_INVALID", "ARTIFACT_MISMATCH", "VERIFICATION_FAILED", "INCLUSION_INVALID"}},
		{"certificate byte flipped", func(r *Request) {
			r.BundlePath = tamperedBundle(t, func(b map[string]any) {
				chain := b["verificationMaterial"].(map[string]any)["x509CertificateChain"].(map[string]any)["certificates"].([]any)
				c0 := chain[0].(map[string]any)
				c0["rawBytes"] = flipBase64(c0["rawBytes"].(string))
			})
		}, []string{"CERTIFICATE_INVALID", "BUNDLE_UNREADABLE", "SIGNATURE_INVALID", "VERIFICATION_FAILED", "IDENTITY_MISMATCH", "INCLUSION_INVALID"}},
		{"inclusion promise flipped", func(r *Request) {
			r.BundlePath = tamperedBundle(t, func(b map[string]any) {
				entries := b["verificationMaterial"].(map[string]any)["tlogEntries"].([]any)
				e0 := entries[0].(map[string]any)
				ip := e0["inclusionPromise"].(map[string]any)
				ip["signedEntryTimestamp"] = flipBase64(ip["signedEntryTimestamp"].(string))
			})
		}, []string{"INCLUSION_INVALID", "VERIFICATION_FAILED", "SIGNATURE_INVALID"}},
		{"tlog entry removed", func(r *Request) {
			r.BundlePath = tamperedBundle(t, func(b map[string]any) {
				b["verificationMaterial"].(map[string]any)["tlogEntries"] = []any{}
			})
		}, []string{"INCLUSION_INVALID", "VERIFICATION_FAILED", "BUNDLE_UNREADABLE"}},
		{"trusted root not the issuing one", func(r *Request) {
			r.TrustedRootPath = writeEmptyRoot(t)
		}, []string{"TRUSTED_ROOT_UNREADABLE", "CERTIFICATE_INVALID", "VERIFICATION_FAILED", "INCLUSION_INVALID", "SIGNATURE_INVALID"}},
		{"no identity required", func(r *Request) {
			r.CertificateIdentity = Identity{}
		}, []string{"REQUEST_INCOMPLETE"}},
		{"artifact and digest both given", func(r *Request) {
			r.ArtifactPath = fixtureRoot
		}, []string{"REQUEST_INCOMPLETE"}},
		{"unsupported digest algorithm", func(r *Request) {
			r.ArtifactDigest = "md5:abcd"
		}, []string{"ARTIFACT_DIGEST_INVALID"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := goodRequest()
			tc.mut(&req)
			resp, code := runRequest(t, req)
			if code != 1 || resp.Verified {
				t.Fatalf("tamper accepted: exit %d verified=%v", code, resp.Verified)
			}
			if resp.Refusal == nil || resp.Refusal.Code == "" {
				t.Fatal("refusal must carry a code")
			}
			ok := false
			for _, c := range tc.codes {
				ok = ok || resp.Refusal.Code == c
			}
			if !ok {
				t.Fatalf("refusal code %s (%s) not in %v", resp.Refusal.Code, resp.Refusal.Message, tc.codes)
			}
		})
	}
}

func writeEmptyRoot(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "root.json")
	os.WriteFile(p, []byte(`{"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1","tlogs":[],"certificateAuthorities":[],"ctlogs":[],"timestampAuthorities":[]}`), 0o644)
	return p
}

func TestProtocolErrorsExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(strings.NewReader("garbage"), &stdout, &stderr); code != 2 {
		t.Fatalf("garbage request: exit %d", code)
	}
	req := goodRequest()
	req.SchemaVersion = "nomos.sigstore-verify.request.v0"
	raw, _ := json.Marshal(req)
	stdout.Reset()
	if code := run(bytes.NewReader(raw), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("unknown schema must exit 2 with no response, got %d %q", code, stdout.String())
	}
}

// The v0.3 fixture carries an inclusion PROOF and a signed checkpoint: this is
// the proof/checkpoint path, and each is tampered separately.
const (
	proofBundle = "testdata/othername.sigstore.json"
	proofRoot   = "testdata/trusted-root-scaffolding.json"
	proofDigest = "sha256:bc103b4a84971ef6459b294a2b98568a2bfb72cded09d4acd1e16366a401f95b"
	proofSAN    = "foo!oidc.local"
	proofIssuer = "http://oidc.local:8080"
)

func proofRequest() Request {
	return Request{
		SchemaVersion: RequestSchema, BundlePath: proofBundle, TrustedRootPath: proofRoot, ArtifactDigest: proofDigest,
		CertificateIdentity: Identity{SAN: proofSAN, Issuer: proofIssuer},
		Require:             Require{TlogEntries: 1, SignedCertificateTimestamps: 1, ObserverTimestamps: 1},
	}
}

func tamperedProofBundle(t *testing.T, edit func(entry map[string]any)) string {
	t.Helper()
	raw, err := os.ReadFile(proofBundle)
	if err != nil {
		t.Fatal(err)
	}
	var b map[string]any
	json.Unmarshal(raw, &b)
	entries := b["verificationMaterial"].(map[string]any)["tlogEntries"].([]any)
	edit(entries[0].(map[string]any))
	out, _ := json.Marshal(b)
	p := filepath.Join(t.TempDir(), "bundle.json")
	os.WriteFile(p, out, 0o644)
	return p
}

func TestInclusionProofBundleVerifies(t *testing.T) {
	resp, code := runRequest(t, proofRequest())
	if code != 0 || !resp.Verified {
		t.Fatalf("expected verified, exit %d, %+v", code, resp.Refusal)
	}
	if len(resp.TlogEntries) != 1 || !resp.TlogEntries[0].HasInclusionProof {
		t.Fatalf("fixture must carry an inclusion proof: %+v", resp.TlogEntries)
	}
	if resp.Certificate == nil || resp.Certificate.SubjectAlternativeName != proofSAN {
		t.Fatalf("certificate: %+v", resp.Certificate)
	}
}

func TestInclusionProofAndCheckpointTamperIsRefused(t *testing.T) {
	cases := []struct {
		name string
		edit func(entry map[string]any)
	}{
		{"inclusion proof hash flipped", func(e map[string]any) {
			proof := e["inclusionProof"].(map[string]any)
			hashes := proof["hashes"].([]any)
			hashes[0] = flipBase64(hashes[0].(string))
		}},
		{"inclusion proof root hash flipped", func(e map[string]any) {
			proof := e["inclusionProof"].(map[string]any)
			proof["rootHash"] = flipBase64(proof["rootHash"].(string))
		}},
		{"checkpoint envelope edited", func(e map[string]any) {
			cp := e["inclusionProof"].(map[string]any)["checkpoint"].(map[string]any)
			cp["envelope"] = strings.Replace(cp["envelope"].(string), "\n", "\n \n", 1)
		}},
		{"inclusion proof log index moved", func(e map[string]any) {
			proof := e["inclusionProof"].(map[string]any)
			proof["logIndex"] = "7"
		}},
		{"canonicalized body flipped", func(e map[string]any) {
			e["canonicalizedBody"] = flipBase64(e["canonicalizedBody"].(string))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := proofRequest()
			req.BundlePath = tamperedProofBundle(t, tc.edit)
			resp, code := runRequest(t, req)
			if code != 1 || resp.Verified {
				t.Fatalf("tamper accepted: exit %d verified=%v", code, resp.Verified)
			}
			if resp.Refusal == nil || resp.Refusal.Code == "" {
				t.Fatal("refusal must carry a code")
			}
			t.Logf("%s → %s: %s", tc.name, resp.Refusal.Code, resp.Refusal.Message)
		})
	}
	// Wrong trust material for a valid bundle: the public-good root does not know the scaffolding log.
	req := proofRequest()
	req.TrustedRootPath = fixtureRoot
	resp, code := runRequest(t, req)
	if code != 1 || resp.Verified {
		t.Fatal("a bundle must not verify against trust material that does not cover its log and CA")
	}
}
