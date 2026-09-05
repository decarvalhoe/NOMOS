package attestation

// #637 — offline verification of a SUPPLIED Sigstore bundle through a versioned
// process boundary. This package computes no Sigstore cryptography: the
// verifier lives in tools/sigstore-verifier (its own module, ADR-0005) and is
// spoken to over JSON on stdin/stdout. What this side does is fail closed:
//
//   • no verifier available          → error, no verdict;
//   • verifier exits non-zero / silent → error, no verdict;
//   • response schema unknown         → error;
//   • verified=false                  → refusal with the verifier's code;
//   • verified=true but the response's artifact digest is not the digest NOMOS
//     computed itself, or the reported identity does not match what the caller
//     required → refusal (the verifier's word is checked, not trusted).
//
// A record binds the request and the exact response bytes by digest.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	SigstoreRequestSchema  = "nomos.sigstore-verify.request.v1"
	SigstoreResponseSchema = "nomos.sigstore-verify.response.v1"
	SigstoreRecordSchema   = "nomos.sigstore-verification-record.v1"
	// SigstoreVerifierEnv names the external verifier command when no flag is given.
	SigstoreVerifierEnv = "NOMOS_SIGSTORE_VERIFIER"
	// SigstoreVerifierBinary is looked up on PATH as a last resort.
	SigstoreVerifierBinary = "nomos-sigstore-verifier"
	SigstoreDefaultTimeout = 60 * time.Second
	SigstoreClaimBoundary  = "NOMOS verified a SUPPLIED Sigstore bundle offline against SUPPLIED trust " +
		"material through an external verifier, and independently re-checked the artifact digest and the " +
		"signer identity in the verifier's response. NOMOS did not sign, did not obtain an identity, did " +
		"not write to any transparency log, and makes no claim about the trust material's freshness."
)

// Refusal codes raised on THIS side of the boundary.
const (
	CodeSigstoreVerifierUnavailable  = "SIGSTORE_VERIFIER_UNAVAILABLE"
	CodeSigstoreVerifierFailed       = "SIGSTORE_VERIFIER_FAILED"
	CodeSigstoreBadResponse          = "SIGSTORE_BAD_RESPONSE"
	CodeSigstoreRefused              = "SIGSTORE_REFUSED"
	CodeSigstoreDigestDisagreement   = "SIGSTORE_DIGEST_DISAGREEMENT"
	CodeSigstoreIdentityDisagreement = "SIGSTORE_IDENTITY_DISAGREEMENT"
)

// SigstoreError carries a stable code.
type SigstoreError struct {
	Code    string
	Message string
}

func (e *SigstoreError) Error() string { return e.Code + ": " + e.Message }

func sigstoreErr(code, format string, args ...any) error {
	return &SigstoreError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// SigstoreIdentity is the signer the caller REQUIRES.
type SigstoreIdentity struct {
	SAN         string `json:"san,omitempty"`
	SANRegex    string `json:"san_regex,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
	IssuerRegex string `json:"issuer_regex,omitempty"`
}

// SigstoreRequire are the verification thresholds.
type SigstoreRequire struct {
	TlogEntries                 int `json:"tlog_entries"`
	SignedCertificateTimestamps int `json:"signed_certificate_timestamps"`
	ObserverTimestamps          int `json:"observer_timestamps"`
}

// SigstoreRequest is what crosses the boundary.
type SigstoreRequest struct {
	SchemaVersion       string           `json:"schema_version"`
	BundlePath          string           `json:"bundle_path"`
	TrustedRootPath     string           `json:"trusted_root_path"`
	ArtifactPath        string           `json:"artifact_path,omitempty"`
	ArtifactDigest      string           `json:"artifact_digest,omitempty"`
	CertificateIdentity SigstoreIdentity `json:"certificate_identity"`
	Require             SigstoreRequire  `json:"require"`
}

// SigstoreResponse is what comes back. Fields the engine re-checks are the
// artifact digest and the certificate identity.
type SigstoreResponse struct {
	SchemaVersion string `json:"schema_version"`
	Verifier      struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		Library        string `json:"library"`
		LibraryVersion string `json:"library_version"`
	} `json:"verifier"`
	Mode     string `json:"mode"`
	Verified bool   `json:"verified"`
	Refusal  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"refusal,omitempty"`
	MediaType      string `json:"media_type,omitempty"`
	ArtifactDigest string `json:"artifact_digest,omitempty"`
	Certificate    *struct {
		SubjectAlternativeName string            `json:"subject_alternative_name"`
		Issuer                 string            `json:"issuer"`
		Extensions             map[string]string `json:"extensions,omitempty"`
	} `json:"certificate,omitempty"`
	Timestamps []struct {
		Type      string `json:"type"`
		URI       string `json:"uri"`
		Timestamp string `json:"timestamp"`
	} `json:"timestamps,omitempty"`
	TlogEntries []struct {
		LogIndex            int64  `json:"log_index"`
		LogKeyID            string `json:"log_key_id"`
		IntegratedTime      string `json:"integrated_time"`
		HasInclusionProof   bool   `json:"has_inclusion_proof"`
		HasInclusionPromise bool   `json:"has_inclusion_promise"`
	} `json:"tlog_entries,omitempty"`
	ClaimBoundary string `json:"claim_boundary"`
}

// SigstoreVerifier runs a verification request.
type SigstoreVerifier interface {
	Verify(req SigstoreRequest) (SigstoreResponse, []byte, int, error)
}

// ExternalSigstoreVerifier is the shipped implementation: one process per
// request. A non-zero exit is not by itself an error here — exit 1 with a
// well-formed refusal is a verdict — but silence or garbage is.
type ExternalSigstoreVerifier struct {
	Command []string
	Timeout time.Duration
}

// ResolveSigstoreVerifier picks the command: explicit, then env, then PATH.
// Returns an error (no verdict possible) when nothing is available.
func ResolveSigstoreVerifier(explicit string) ([]string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.Fields(explicit), nil
	}
	if env := strings.TrimSpace(os.Getenv(SigstoreVerifierEnv)); env != "" {
		return strings.Fields(env), nil
	}
	if p, err := exec.LookPath(SigstoreVerifierBinary); err == nil {
		return []string{p}, nil
	}
	return nil, sigstoreErr(CodeSigstoreVerifierUnavailable,
		"no external verifier: pass --verifier, set %s, or put %s on PATH (build it from tools/sigstore-verifier). No verdict.",
		SigstoreVerifierEnv, SigstoreVerifierBinary)
}

// Verify implements SigstoreVerifier.
func (e ExternalSigstoreVerifier) Verify(req SigstoreRequest) (SigstoreResponse, []byte, int, error) {
	if len(e.Command) == 0 || strings.TrimSpace(e.Command[0]) == "" {
		return SigstoreResponse{}, nil, -1, sigstoreErr(CodeSigstoreVerifierUnavailable, "empty verifier command")
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return SigstoreResponse{}, nil, -1, sigstoreErr(CodeSigstoreVerifierFailed, "encode request: %v", err)
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = SigstoreDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.Command[0], e.Command[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exit := 0
	if runErr != nil {
		var ee *exec.ExitError
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			return SigstoreResponse{}, nil, -1, sigstoreErr(CodeSigstoreVerifierFailed, "verifier %q timed out after %s; no verdict", e.Command[0], timeout)
		case errors.As(runErr, &ee):
			exit = ee.ExitCode()
		default:
			return SigstoreResponse{}, nil, -1, sigstoreErr(CodeSigstoreVerifierUnavailable, "verifier %q could not run: %v; no verdict", e.Command[0], runErr)
		}
	}
	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return SigstoreResponse{}, nil, exit, sigstoreErr(CodeSigstoreVerifierFailed, "verifier %q exited %d without a response%s; no verdict", e.Command[0], exit, tail(stderr.String()))
	}
	var resp SigstoreResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return SigstoreResponse{}, raw, exit, sigstoreErr(CodeSigstoreBadResponse, "verifier %q wrote something that is not %s JSON: %v%s", e.Command[0], SigstoreResponseSchema, err, tail(stderr.String()))
	}
	return resp, raw, exit, nil
}

// SigstoreVerificationRecord is what NOMOS writes: the request, the verifier's
// exact response bytes by digest, and the facts NOMOS re-checked itself.
type SigstoreVerificationRecord struct {
	SchemaVersion   string           `json:"schema_version"`
	VerifiedAt      string           `json:"verified_at"`
	Verifier        string           `json:"verifier"`
	VerifierVersion string           `json:"verifier_version"`
	Library         string           `json:"library"`
	LibraryVersion  string           `json:"library_version"`
	Mode            string           `json:"mode"`
	Request         SigstoreRequest  `json:"request"`
	RequestDigest   string           `json:"request_digest"`
	ResponseDigest  string           `json:"response_digest"`
	Response        SigstoreResponse `json:"response"`
	ArtifactDigest  string           `json:"artifact_digest"`
	SignerSAN       string           `json:"signer_san"`
	SignerIssuer    string           `json:"signer_issuer"`
	TlogEntries     int              `json:"tlog_entries"`
	Verified        bool             `json:"verified"`
	ClaimBoundary   string           `json:"claim_boundary"`
}

// VerifySigstoreBundle runs the request through the verifier and applies the
// engine's own checks. On success the record is returned; every failure is a
// *SigstoreError and no record is produced.
func VerifySigstoreBundle(v SigstoreVerifier, req SigstoreRequest, now time.Time) (SigstoreVerificationRecord, []byte, error) {
	req.SchemaVersion = SigstoreRequestSchema
	if (req.ArtifactPath == "") == (req.ArtifactDigest == "") {
		return SigstoreVerificationRecord{}, nil, sigstoreErr(CodeSigstoreRefused, "exactly one of artifact path or artifact digest is required")
	}
	if req.CertificateIdentity.SAN == "" && req.CertificateIdentity.SANRegex == "" {
		return SigstoreVerificationRecord{}, nil, sigstoreErr(CodeSigstoreRefused, "a required signer identity (san or san_regex) is mandatory — verifying an unnamed signer proves nothing")
	}
	if req.CertificateIdentity.Issuer == "" && req.CertificateIdentity.IssuerRegex == "" {
		return SigstoreVerificationRecord{}, nil, sigstoreErr(CodeSigstoreRefused, "a required issuer (issuer or issuer_regex) is mandatory")
	}

	// NOMOS's own view of the artifact digest, computed before asking anyone.
	expectedDigest := strings.ToLower(strings.TrimSpace(req.ArtifactDigest))
	if req.ArtifactPath != "" {
		data, err := os.ReadFile(req.ArtifactPath)
		if err != nil {
			return SigstoreVerificationRecord{}, nil, sigstoreErr(CodeSigstoreRefused, "artifact: %v", err)
		}
		sum := sha256.Sum256(data)
		expectedDigest = "sha256:" + hex.EncodeToString(sum[:])
	}
	if !validDigest(expectedDigest) {
		return SigstoreVerificationRecord{}, nil, sigstoreErr(CodeSigstoreRefused, "artifact digest must be sha256|sha384|sha512:<hex> of the right length, got %q", expectedDigest)
	}

	resp, raw, exit, err := v.Verify(req)
	if err != nil {
		return SigstoreVerificationRecord{}, raw, err
	}
	if resp.SchemaVersion != SigstoreResponseSchema {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreBadResponse, "response schema_version %q, want %q", resp.SchemaVersion, SigstoreResponseSchema)
	}
	if !resp.Verified {
		code, msg := "unspecified", "verifier returned verified=false without a refusal"
		if resp.Refusal != nil {
			code, msg = resp.Refusal.Code, resp.Refusal.Message
		}
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreRefused, "verifier refused (exit %d) %s: %s", exit, code, msg)
	}
	if exit != 0 {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreBadResponse, "verifier says verified=true but exited %d; contradictory, no verdict", exit)
	}
	if resp.Mode != "offline" {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreBadResponse, "verifier mode %q; only offline verification is accepted", resp.Mode)
	}
	// Re-check 1: the digest the verifier says it verified is the digest NOMOS computed.
	if strings.ToLower(resp.ArtifactDigest) != expectedDigest {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreDigestDisagreement,
			"verifier reports artifact %s, NOMOS computed %s — not the same artifact", resp.ArtifactDigest, expectedDigest)
	}
	// Re-check 2: the reported signer matches the REQUIRED identity.
	if resp.Certificate == nil {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreIdentityDisagreement, "verifier reports no signing certificate; an unidentified signer is not accepted")
	}
	if err := matchIdentity(req.CertificateIdentity, resp.Certificate.SubjectAlternativeName, resp.Certificate.Issuer); err != nil {
		return SigstoreVerificationRecord{}, raw, err
	}
	if len(resp.TlogEntries) < max(1, req.Require.TlogEntries) {
		return SigstoreVerificationRecord{}, raw, sigstoreErr(CodeSigstoreRefused, "response carries %d transparency-log entr(ies), %d required", len(resp.TlogEntries), max(1, req.Require.TlogEntries))
	}

	reqRaw, _ := json.Marshal(req)
	rec := SigstoreVerificationRecord{
		SchemaVersion: SigstoreRecordSchema, VerifiedAt: now.UTC().Format(time.RFC3339),
		Verifier: resp.Verifier.Name, VerifierVersion: resp.Verifier.Version, Library: resp.Verifier.Library, LibraryVersion: resp.Verifier.LibraryVersion,
		Mode: resp.Mode, Request: req, RequestDigest: digestOf(reqRaw), ResponseDigest: digestOf(raw), Response: resp,
		ArtifactDigest: expectedDigest, SignerSAN: resp.Certificate.SubjectAlternativeName, SignerIssuer: resp.Certificate.Issuer,
		TlogEntries: len(resp.TlogEntries), Verified: true, ClaimBoundary: SigstoreClaimBoundary,
	}
	return rec, raw, nil
}

// VerifySigstoreRecordResponse proves a record's response_digest is the digest
// of the given raw response bytes — the record binds what the verifier wrote.
func VerifySigstoreRecordResponse(rec SigstoreVerificationRecord, raw []byte) error {
	if rec.SchemaVersion != SigstoreRecordSchema {
		return fmt.Errorf("record schema_version %q, want %q", rec.SchemaVersion, SigstoreRecordSchema)
	}
	if got := digestOf(bytes.TrimSpace(raw)); got != rec.ResponseDigest {
		return fmt.Errorf("response bytes digest %s, record says %s", got, rec.ResponseDigest)
	}
	return nil
}

func matchIdentity(want SigstoreIdentity, san, issuer string) error {
	if want.SAN != "" && want.SAN != san {
		return sigstoreErr(CodeSigstoreIdentityDisagreement, "signer SAN %q, required %q", san, want.SAN)
	}
	if want.SANRegex != "" {
		re, err := regexp.Compile(want.SANRegex)
		if err != nil {
			return sigstoreErr(CodeSigstoreRefused, "san_regex: %v", err)
		}
		if !re.MatchString(san) {
			return sigstoreErr(CodeSigstoreIdentityDisagreement, "signer SAN %q does not match required /%s/", san, want.SANRegex)
		}
	}
	if want.Issuer != "" && want.Issuer != issuer {
		return sigstoreErr(CodeSigstoreIdentityDisagreement, "signer issuer %q, required %q", issuer, want.Issuer)
	}
	if want.IssuerRegex != "" {
		re, err := regexp.Compile(want.IssuerRegex)
		if err != nil {
			return sigstoreErr(CodeSigstoreRefused, "issuer_regex: %v", err)
		}
		if !re.MatchString(issuer) {
			return sigstoreErr(CodeSigstoreIdentityDisagreement, "signer issuer %q does not match required /%s/", issuer, want.IssuerRegex)
		}
	}
	return nil
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return " — stderr: " + strings.Join(lines, " | ")
}

var digestRe = regexp.MustCompile(`^(sha256:[0-9a-f]{64}|sha384:[0-9a-f]{96}|sha512:[0-9a-f]{128})$`)

func validDigest(d string) bool { return digestRe.MatchString(d) }
