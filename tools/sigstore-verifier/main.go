// nomos-sigstore-verifier — the external process behind `nomos attest
// verify-sigstore` (#637).
//
// The NOMOS engine (cli/) carries three direct dependencies and a probe that
// forbids the Sigstore libraries inside it; this module is where sigstore-go
// lives, behind a versioned JSON protocol on stdin/stdout (ADR-0005). It
// VERIFIES a supplied bundle offline against supplied trust material. It never
// signs, never talks to Fulcio or Rekor, never fetches a trusted root.
//
// Exit codes: 0 verified · 1 refused (verified=false, refusal set) · 2 protocol
// error (request unreadable). A response is written on stdout in every case
// where the request could be parsed.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	RequestSchema   = "nomos.sigstore-verify.request.v1"
	ResponseSchema  = "nomos.sigstore-verify.response.v1"
	VerifierName    = "nomos-sigstore-verifier"
	VerifierVersion = "0.1.0"
)

type Identity struct {
	SAN         string `json:"san,omitempty"`
	SANRegex    string `json:"san_regex,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
	IssuerRegex string `json:"issuer_regex,omitempty"`
}

type Require struct {
	TlogEntries                 int `json:"tlog_entries"`
	SignedCertificateTimestamps int `json:"signed_certificate_timestamps"`
	ObserverTimestamps          int `json:"observer_timestamps"`
	// NoCTLog must be true for signed_certificate_timestamps == 0 to be honoured;
	// an omitted field is a JSON zero, not a decision.
	NoCTLog bool `json:"no_ct_log,omitempty"`
}

type Request struct {
	SchemaVersion       string   `json:"schema_version"`
	BundlePath          string   `json:"bundle_path"`
	TrustedRootPath     string   `json:"trusted_root_path"`
	ArtifactPath        string   `json:"artifact_path,omitempty"`
	ArtifactDigest      string   `json:"artifact_digest,omitempty"`
	CertificateIdentity Identity `json:"certificate_identity"`
	Require             Require  `json:"require"`
}

type Refusal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Verifier struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	Library        string `json:"library"`
	LibraryVersion string `json:"library_version"`
}

type Certificate struct {
	SubjectAlternativeName string            `json:"subject_alternative_name"`
	Issuer                 string            `json:"issuer"`
	Extensions             map[string]string `json:"extensions,omitempty"`
}

type Timestamp struct {
	Type      string `json:"type"`
	URI       string `json:"uri"`
	Timestamp string `json:"timestamp"`
}

type TlogEntry struct {
	LogIndex            int64  `json:"log_index"`
	LogKeyID            string `json:"log_key_id"`
	IntegratedTime      string `json:"integrated_time"`
	HasInclusionProof   bool   `json:"has_inclusion_proof"`
	HasInclusionPromise bool   `json:"has_inclusion_promise"`
}

type Response struct {
	SchemaVersion  string       `json:"schema_version"`
	Verifier       Verifier     `json:"verifier"`
	Mode           string       `json:"mode"`
	Verified       bool         `json:"verified"`
	Refusal        *Refusal     `json:"refusal,omitempty"`
	MediaType      string       `json:"media_type,omitempty"`
	ArtifactDigest string       `json:"artifact_digest,omitempty"`
	Certificate    *Certificate `json:"certificate,omitempty"`
	Timestamps     []Timestamp  `json:"timestamps,omitempty"`
	TlogEntries    []TlogEntry  `json:"tlog_entries,omitempty"`
	ClaimBoundary  string       `json:"claim_boundary"`
}

const claimBoundary = "Offline verification of a SUPPLIED bundle against SUPPLIED trust material: " +
	"signature over the artifact digest, certificate chain and identity, transparency-log inclusion " +
	"and timestamps as required. Nothing was signed, issued or published; the trust material's " +
	"freshness is the caller's responsibility."

func libraryVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/sigstore/sigstore-go" {
				return d.Version
			}
		}
	}
	return "unknown"
}

func newResponse() Response {
	return Response{
		SchemaVersion: ResponseSchema,
		Verifier:      Verifier{Name: VerifierName, Version: VerifierVersion, Library: "sigstore-go", LibraryVersion: libraryVersion()},
		Mode:          "offline",
		ClaimBoundary: claimBoundary,
	}
}

func refuse(resp *Response, code, format string, args ...any) {
	resp.Verified = false
	resp.Refusal = &Refusal{Code: code, Message: fmt.Sprintf(format, args...)}
}

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: read request: %v\n", VerifierName, err)
		return 2
	}
	var probe struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		fmt.Fprintf(stderr, "%s: request is not JSON: %v\n", VerifierName, err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	switch probe.SchemaVersion {
	case RequestSchema:
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			fmt.Fprintf(stderr, "%s: request: %v\n", VerifierName, err)
			return 2
		}
		resp := verifyRequest(req)
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintf(stderr, "%s: write response: %v\n", VerifierName, err)
			return 2
		}
		if resp.Verified {
			return 0
		}
		return 1
	case IssueRequestSchema:
		var req IssueRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			fmt.Fprintf(stderr, "%s: request: %v\n", VerifierName, err)
			return 2
		}
		resp := issueRequest(req)
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintf(stderr, "%s: write response: %v\n", VerifierName, err)
			return 2
		}
		if resp.Issued {
			return 0
		}
		return 1
	default:
		fmt.Fprintf(stderr, "%s: request schema_version %q, want %q or %q\n", VerifierName, probe.SchemaVersion, RequestSchema, IssueRequestSchema)
		return 2
	}
}

func verifyRequest(req Request) Response {
	resp := newResponse()
	if req.BundlePath == "" || req.TrustedRootPath == "" {
		refuse(&resp, "REQUEST_INCOMPLETE", "bundle_path and trusted_root_path are required")
		return resp
	}
	if (req.ArtifactPath == "") == (req.ArtifactDigest == "") {
		refuse(&resp, "REQUEST_INCOMPLETE", "exactly one of artifact_path or artifact_digest is required")
		return resp
	}
	id := req.CertificateIdentity
	if id.SAN == "" && id.SANRegex == "" {
		refuse(&resp, "REQUEST_INCOMPLETE", "certificate_identity needs san or san_regex — an unidentified signer is not a verification")
		return resp
	}
	if id.Issuer == "" && id.IssuerRegex == "" {
		refuse(&resp, "REQUEST_INCOMPLETE", "certificate_identity needs issuer or issuer_regex")
		return resp
	}

	trusted, err := root.NewTrustedRootFromPath(req.TrustedRootPath)
	if err != nil {
		refuse(&resp, "TRUSTED_ROOT_UNREADABLE", "%v", err)
		return resp
	}
	b, err := bundle.LoadJSONFromPath(req.BundlePath)
	if err != nil {
		refuse(&resp, "BUNDLE_UNREADABLE", "%v", err)
		return resp
	}
	resp.MediaType = b.MediaType

	tlogN, sctN, obsN := req.Require.TlogEntries, req.Require.SignedCertificateTimestamps, req.Require.ObserverTimestamps
	if tlogN <= 0 {
		tlogN = 1
	}
	if sctN <= 0 && !req.Require.NoCTLog {
		sctN = 1
	}
	if obsN <= 0 {
		obsN = 1
	}
	// sctN == 0 is an explicit choice for an injected environment without a CT
	// log; the caller's request records it, the default stays 1.
	verifierOpts := []verify.VerifierOption{verify.WithTransparencyLog(tlogN), verify.WithObserverTimestamps(obsN)}
	if sctN > 0 {
		verifierOpts = append(verifierOpts, verify.WithSignedCertificateTimestamps(sctN))
	}
	verifier, err := verify.NewVerifier(trusted, verifierOpts...)
	if err != nil {
		refuse(&resp, "VERIFIER_CONFIG", "%v", err)
		return resp
	}

	certID, err := verify.NewShortCertificateIdentity(id.Issuer, id.IssuerRegex, id.SAN, id.SANRegex)
	if err != nil {
		refuse(&resp, "IDENTITY_INVALID", "%v", err)
		return resp
	}

	var artifactOpt verify.ArtifactPolicyOption
	digestAlg, digestHex := "sha256", ""
	if req.ArtifactPath != "" {
		data, err := os.ReadFile(req.ArtifactPath)
		if err != nil {
			refuse(&resp, "ARTIFACT_UNREADABLE", "%v", err)
			return resp
		}
		sum := sha256.Sum256(data)
		digestHex = hex.EncodeToString(sum[:])
		artifactOpt = verify.WithArtifactDigest("sha256", sum[:])
	} else {
		alg, hexStr, ok := strings.Cut(strings.ToLower(strings.TrimSpace(req.ArtifactDigest)), ":")
		want := map[string]int{"sha256": 32, "sha384": 48, "sha512": 64}[alg]
		if !ok || want == 0 {
			refuse(&resp, "ARTIFACT_DIGEST_INVALID", "artifact_digest must be sha256|sha384|sha512:<hex>")
			return resp
		}
		digest, err := hex.DecodeString(hexStr)
		if err != nil || len(digest) != want {
			refuse(&resp, "ARTIFACT_DIGEST_INVALID", "artifact_digest is not %d hex bytes for %s", want, alg)
			return resp
		}
		digestAlg, digestHex = alg, hexStr
		artifactOpt = verify.WithArtifactDigest(alg, digest)
	}
	// The digest we report is the one the policy was built from; if the bundle
	// does not cover it, verification fails below and the caller sees both.
	resp.ArtifactDigest = digestAlg + ":" + digestHex

	result, err := verifier.Verify(b, verify.NewPolicy(artifactOpt, verify.WithCertificateIdentity(certID)))
	if err != nil {
		refuse(&resp, classify(err), "%v", err)
		return resp
	}

	if result.Signature != nil && result.Signature.Certificate != nil {
		c := result.Signature.Certificate
		ext := map[string]string{}
		for k, v := range map[string]string{
			"build_signer_uri":         c.Extensions.BuildSignerURI,
			"build_signer_digest":      c.Extensions.BuildSignerDigest,
			"runner_environment":       c.Extensions.RunnerEnvironment,
			"source_repository_uri":    c.Extensions.SourceRepositoryURI,
			"source_repository_digest": c.Extensions.SourceRepositoryDigest,
			"source_repository_ref":    c.Extensions.SourceRepositoryRef,
			"build_config_uri":         c.Extensions.BuildConfigURI,
			"build_trigger":            c.Extensions.BuildTrigger,
			"run_invocation_uri":       c.Extensions.RunInvocationURI,
		} {
			if v != "" {
				ext[k] = v
			}
		}
		resp.Certificate = &Certificate{SubjectAlternativeName: c.SubjectAlternativeName, Issuer: c.Extensions.Issuer, Extensions: ext}
	}
	for _, ts := range result.VerifiedTimestamps {
		resp.Timestamps = append(resp.Timestamps, Timestamp{Type: ts.Type, URI: ts.URI, Timestamp: ts.Timestamp.UTC().Format("2006-01-02T15:04:05Z")})
	}
	if entries, err := b.TlogEntries(); err == nil {
		for _, e := range entries {
			resp.TlogEntries = append(resp.TlogEntries, TlogEntry{
				LogIndex: e.LogIndex(), LogKeyID: hex.EncodeToString([]byte(e.LogKeyID())),
				IntegratedTime:    e.IntegratedTime().UTC().Format("2006-01-02T15:04:05Z"),
				HasInclusionProof: e.HasInclusionProof(), HasInclusionPromise: e.HasInclusionPromise(),
			})
		}
	}
	resp.Verified = true
	return resp
}

// classify maps a verification error onto a stable refusal code from its text;
// sigstore-go returns plain errors, so the message is preserved verbatim.
func classify(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "identity"), strings.Contains(msg, "certificate identities"), strings.Contains(msg, "san"), strings.Contains(msg, "issuer"):
		return "IDENTITY_MISMATCH"
	case strings.Contains(msg, "artifact"), strings.Contains(msg, "digest"), strings.Contains(msg, "subject"):
		return "ARTIFACT_MISMATCH"
	case strings.Contains(msg, "inclusion"), strings.Contains(msg, "checkpoint"), strings.Contains(msg, "tlog"), strings.Contains(msg, "transparency"), strings.Contains(msg, "log entry"):
		return "INCLUSION_INVALID"
	case strings.Contains(msg, "signature"):
		return "SIGNATURE_INVALID"
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "chain"), strings.Contains(msg, "sct"):
		return "CERTIFICATE_INVALID"
	case errors.Is(err, os.ErrNotExist):
		return "INPUT_UNREADABLE"
	default:
		return "VERIFICATION_FAILED"
	}
}
