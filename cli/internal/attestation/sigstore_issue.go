package attestation

// #645 — keyless issuance against INJECTED, NON-PRODUCTION Fulcio/Rekor
// endpoints, through the same process boundary as verification (ADR-0005).
// Production is forbidden by default and there is no flag to allow it: the
// production activation, its OIDC permission and its public identity are #638
// (regulated lane). What NOMOS does here, fail closed:
//
//   • endpoint policy is applied BEFORE any process runs: a Sigstore public
//     instance host is refused outright; anything that is not loopback, a
//     reserved test domain, or explicitly allow-listed is refused;
//   • the external issuer's word is checked, not trusted: issued=true must
//     come with exit 0, non_production=true, a digest equal to the one NOMOS
//     computed, the required identity, and a bundle file that exists;
//   • NOMOS then verifies the produced bundle itself, through the verifier
//     path, against the supplied trust material — an independent verdict;
//   • any refusal leaves no bundle behind.

import (
	"encoding/json"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	SigstoreIssueRequestSchema    = "nomos.sigstore-issue.request.v1"
	SigstoreIssueResponseSchema   = "nomos.sigstore-issue.response.v1"
	SigstoreIssuanceRecordSchema  = "nomos.sigstore-issuance-record.v1"
	SigstoreIssuanceClaimBoundary = "NOMOS issued a keyless bundle against INJECTED, NON-PRODUCTION Fulcio/Rekor " +
		"endpoints with a fixture identity, and verified the result independently against the supplied trust " +
		"material. No production service was contacted, no production identity was obtained, no public log " +
		"was written; the trust in this bundle is exactly the trust in the injected environment."
)

const (
	CodeSigstoreProductionForbidden = "SIGSTORE_PRODUCTION_FORBIDDEN"
	CodeSigstoreEndpointNotAllowed  = "SIGSTORE_ENDPOINT_NOT_ALLOWED"
	CodeSigstoreIssueRefused        = "SIGSTORE_ISSUE_REFUSED"
	CodeSigstoreIssueBadResponse    = "SIGSTORE_ISSUE_BAD_RESPONSE"
)

// productionSuffixes are the Sigstore public instances. Refused always (#638).
var productionSuffixes = []string{"sigstore.dev", "sigstage.dev"}

// allowedSuffixes are what "injected / non-production" means by default.
var allowedSuffixes = []string{"localhost", ".localhost", ".invalid", ".test", ".local", ".example", ".internal"}

// CheckEndpointPolicy refuses production and unknown hosts. allowHosts are
// exact hostnames the caller has explicitly declared as controlled.
func CheckEndpointPolicy(rawURL string, allowHosts []string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return sigstoreErr(CodeSigstoreEndpointNotAllowed, "endpoint %q is not an http(s) URL with a host", rawURL)
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range productionSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return sigstoreErr(CodeSigstoreProductionForbidden,
				"endpoint %s is a Sigstore public instance; production issuance is #638 (regulated lane) and has no flag here", host)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return nil
		}
		return sigstoreErr(CodeSigstoreEndpointNotAllowed, "endpoint %s is a non-loopback IP; only injected, controlled endpoints are allowed", host)
	}
	for _, suffix := range allowedSuffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return nil
		}
	}
	for _, allowed := range allowHosts {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return nil
		}
	}
	return sigstoreErr(CodeSigstoreEndpointNotAllowed,
		"endpoint %s is neither loopback, a reserved test domain, nor explicitly allow-listed (--allow-host)", host)
}

// SigstoreIssueRequest crosses the boundary.
type SigstoreIssueRequest struct {
	SchemaVersion   string           `json:"schema_version"`
	ArtifactPath    string           `json:"artifact_path"`
	OutBundlePath   string           `json:"out_bundle_path"`
	FulcioURL       string           `json:"fulcio_url"`
	RekorURL        string           `json:"rekor_url"`
	IDToken         string           `json:"id_token"`
	TrustedRootPath string           `json:"trusted_root_path"`
	Identity        SigstoreIdentity `json:"certificate_identity"`
	AllowHosts      []string         `json:"allow_hosts,omitempty"`
}

// SigstoreIssueResponse comes back.
type SigstoreIssueResponse struct {
	SchemaVersion string `json:"schema_version"`
	Verifier      struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		Library        string `json:"library"`
		LibraryVersion string `json:"library_version"`
	} `json:"verifier"`
	NonProduction bool `json:"non_production"`
	Issued        bool `json:"issued"`
	Refusal       *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"refusal,omitempty"`
	BundlePath     string `json:"bundle_path,omitempty"`
	ArtifactDigest string `json:"artifact_digest,omitempty"`
	Endpoints      struct {
		Fulcio string `json:"fulcio"`
		Rekor  string `json:"rekor"`
	} `json:"endpoints"`
	Certificate *struct {
		SubjectAlternativeName string `json:"subject_alternative_name"`
		Issuer                 string `json:"issuer"`
	} `json:"certificate,omitempty"`
	Verification  *SigstoreResponse `json:"verification,omitempty"`
	ClaimBoundary string            `json:"claim_boundary"`
}

// SigstoreIssuer runs an issuance request.
type SigstoreIssuer interface {
	Issue(req SigstoreIssueRequest) (SigstoreIssueResponse, []byte, int, error)
}

// ExternalSigstoreIssuer is the shipped implementation (same binary as the verifier).
type ExternalSigstoreIssuer struct {
	Command []string
	Timeout time.Duration
}

// Issue implements SigstoreIssuer.
func (e ExternalSigstoreIssuer) Issue(req SigstoreIssueRequest) (SigstoreIssueResponse, []byte, int, error) {
	raw, exit, err := runBoundary(e.Command, e.Timeout, req)
	if err != nil {
		return SigstoreIssueResponse{}, raw, exit, err
	}
	var resp SigstoreIssueResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return SigstoreIssueResponse{}, raw, exit, sigstoreErr(CodeSigstoreIssueBadResponse, "issuer wrote something that is not %s JSON: %v", SigstoreIssueResponseSchema, err)
	}
	return resp, raw, exit, nil
}

// SigstoreIssuanceRecord is what NOMOS writes after a successful issuance AND
// its own independent verification of the produced bundle.
type SigstoreIssuanceRecord struct {
	SchemaVersion    string                     `json:"schema_version"`
	IssuedAt         string                     `json:"issued_at"`
	NonProduction    bool                       `json:"non_production"`
	Endpoints        map[string]string          `json:"endpoints"`
	Request          SigstoreIssueRequest       `json:"request"`
	RequestDigest    string                     `json:"request_digest"`
	ResponseDigest   string                     `json:"response_digest"`
	Response         SigstoreIssueResponse      `json:"response"`
	BundlePath       string                     `json:"bundle_path"`
	BundleDigest     string                     `json:"bundle_digest"`
	ArtifactDigest   string                     `json:"artifact_digest"`
	SignerSAN        string                     `json:"signer_san"`
	SignerIssuer     string                     `json:"signer_issuer"`
	IndependentCheck SigstoreVerificationRecord `json:"independent_verification"`
	ClaimBoundary    string                     `json:"claim_boundary"`
}

// IssueSigstoreBundle applies the endpoint policy, runs the issuer, checks its
// word, then verifies the bundle independently. Every failure removes the
// bundle file if one was written, and returns a *SigstoreError.
func IssueSigstoreBundle(issuer SigstoreIssuer, verifier SigstoreVerifier, req SigstoreIssueRequest, now time.Time) (SigstoreIssuanceRecord, error) {
	req.SchemaVersion = SigstoreIssueRequestSchema
	for name, u := range map[string]string{"fulcio": req.FulcioURL, "rekor": req.RekorURL} {
		if strings.TrimSpace(u) == "" {
			return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreEndpointNotAllowed, "%s endpoint is required; nothing is discovered or defaulted", name)
		}
		if err := CheckEndpointPolicy(u, req.AllowHosts); err != nil {
			return SigstoreIssuanceRecord{}, err
		}
	}
	if req.Identity.SAN == "" && req.Identity.SANRegex == "" {
		return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreIssueRefused, "a required signer identity is mandatory")
	}
	if req.Identity.Issuer == "" && req.Identity.IssuerRegex == "" {
		return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreIssueRefused, "a required issuer is mandatory")
	}
	if strings.TrimSpace(req.IDToken) == "" {
		return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreIssueRefused, "an id token is required (fixture or injected); NOMOS never fetches one")
	}
	if req.OutBundlePath == "" || req.TrustedRootPath == "" || req.ArtifactPath == "" {
		return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreIssueRefused, "artifact, out bundle path and trusted root are required")
	}
	data, err := os.ReadFile(req.ArtifactPath)
	if err != nil {
		return SigstoreIssuanceRecord{}, sigstoreErr(CodeSigstoreIssueRefused, "artifact: %v", err)
	}
	expectedDigest := digestOf(data)

	resp, raw, exit, err := issuer.Issue(req)
	fail := func(e error) (SigstoreIssuanceRecord, error) {
		os.Remove(req.OutBundlePath) // no partial artifact survives a refusal
		return SigstoreIssuanceRecord{}, e
	}
	if err != nil {
		return fail(err)
	}
	if resp.SchemaVersion != SigstoreIssueResponseSchema {
		return fail(sigstoreErr(CodeSigstoreIssueBadResponse, "response schema_version %q, want %q", resp.SchemaVersion, SigstoreIssueResponseSchema))
	}
	if !resp.Issued {
		code, msg := "unspecified", "issuer returned issued=false without a refusal"
		if resp.Refusal != nil {
			code, msg = resp.Refusal.Code, resp.Refusal.Message
		}
		return fail(sigstoreErr(CodeSigstoreIssueRefused, "issuer refused (exit %d) %s: %s", exit, code, msg))
	}
	if exit != 0 {
		return fail(sigstoreErr(CodeSigstoreIssueBadResponse, "issuer says issued=true but exited %d; contradictory, no bundle accepted", exit))
	}
	if !resp.NonProduction {
		return fail(sigstoreErr(CodeSigstoreProductionForbidden, "issuer did not mark the issuance non_production; refused"))
	}
	if strings.ToLower(resp.ArtifactDigest) != expectedDigest {
		return fail(sigstoreErr(CodeSigstoreDigestDisagreement, "issuer reports artifact %s, NOMOS computed %s", resp.ArtifactDigest, expectedDigest))
	}
	if resp.Certificate == nil {
		return fail(sigstoreErr(CodeSigstoreIdentityDisagreement, "issuer reports no certificate identity"))
	}
	if err := matchIdentity(req.Identity, resp.Certificate.SubjectAlternativeName, resp.Certificate.Issuer); err != nil {
		return fail(err)
	}
	if resp.BundlePath != req.OutBundlePath {
		return fail(sigstoreErr(CodeSigstoreIssueBadResponse, "issuer wrote %q, NOMOS asked for %q", resp.BundlePath, req.OutBundlePath))
	}
	bundleRaw, err := os.ReadFile(req.OutBundlePath)
	if err != nil {
		return fail(sigstoreErr(CodeSigstoreIssueBadResponse, "issued bundle unreadable: %v", err))
	}

	// Independent verification through the verifier path, with NOMOS's own re-checks.
	verifyReq := SigstoreRequest{
		BundlePath: req.OutBundlePath, TrustedRootPath: req.TrustedRootPath, ArtifactPath: req.ArtifactPath,
		CertificateIdentity: req.Identity,
		// The injected environment runs no CT log: no SCT can exist. Recorded, not hidden.
		Require: SigstoreRequire{TlogEntries: 1, SignedCertificateTimestamps: 0, ObserverTimestamps: 1, NoCTLog: true},
	}
	check, _, err := VerifySigstoreBundle(verifier, verifyReq, now)
	if err != nil {
		return fail(sigstoreErr(CodeSigstoreIssueRefused, "issued bundle failed NOMOS's independent verification: %v", err))
	}

	reqRaw, _ := json.Marshal(req)
	return SigstoreIssuanceRecord{
		SchemaVersion: SigstoreIssuanceRecordSchema, IssuedAt: now.UTC().Format(time.RFC3339), NonProduction: true,
		Endpoints: map[string]string{"fulcio": req.FulcioURL, "rekor": req.RekorURL},
		Request:   redactToken(req), RequestDigest: digestOf(reqRaw), ResponseDigest: digestOf(raw), Response: resp,
		BundlePath: req.OutBundlePath, BundleDigest: digestOf(bundleRaw), ArtifactDigest: expectedDigest,
		SignerSAN: resp.Certificate.SubjectAlternativeName, SignerIssuer: resp.Certificate.Issuer,
		IndependentCheck: check, ClaimBoundary: SigstoreIssuanceClaimBoundary,
	}, nil
}

func redactToken(req SigstoreIssueRequest) SigstoreIssueRequest {
	req.IDToken = "<redacted:" + digestOf([]byte(req.IDToken))[:23] + ">"
	return req
}
