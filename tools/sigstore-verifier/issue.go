package main

// Keyless issuance against INJECTED, NON-PRODUCTION endpoints (#645). Same
// binary, same protocol style as verification. The endpoint policy is applied
// here too (defense in depth with the NOMOS side): Sigstore public instances
// are refused, and only loopback / reserved test domains / explicitly
// allow-listed hosts are contacted. After issuance the bundle is verified with
// THIS binary's own verify path against the supplied trust material before
// `issued: true` is ever written; a refusal leaves no bundle file.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sigstore/sigstore-go/pkg/sign"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	IssueRequestSchema  = "nomos.sigstore-issue.request.v1"
	IssueResponseSchema = "nomos.sigstore-issue.response.v1"
)

var productionSuffixes = []string{"sigstore.dev", "sigstage.dev"}
var allowedSuffixes = []string{"localhost", ".localhost", ".invalid", ".test", ".local", ".example", ".internal"}

type IssueRequest struct {
	SchemaVersion   string   `json:"schema_version"`
	ArtifactPath    string   `json:"artifact_path"`
	OutBundlePath   string   `json:"out_bundle_path"`
	FulcioURL       string   `json:"fulcio_url"`
	RekorURL        string   `json:"rekor_url"`
	IDToken         string   `json:"id_token"`
	TrustedRootPath string   `json:"trusted_root_path"`
	Identity        Identity `json:"certificate_identity"`
	AllowHosts      []string `json:"allow_hosts,omitempty"`
}

type IssueResponse struct {
	SchemaVersion  string   `json:"schema_version"`
	Verifier       Verifier `json:"verifier"`
	NonProduction  bool     `json:"non_production"`
	Issued         bool     `json:"issued"`
	Refusal        *Refusal `json:"refusal,omitempty"`
	BundlePath     string   `json:"bundle_path,omitempty"`
	ArtifactDigest string   `json:"artifact_digest,omitempty"`
	Endpoints      struct {
		Fulcio string `json:"fulcio"`
		Rekor  string `json:"rekor"`
	} `json:"endpoints"`
	Certificate   *Certificate `json:"certificate,omitempty"`
	Verification  *Response    `json:"verification,omitempty"`
	ClaimBoundary string       `json:"claim_boundary"`
}

const issueClaimBoundary = "Keyless issuance against INJECTED, NON-PRODUCTION Fulcio/Rekor endpoints with a supplied " +
	"identity token, then verified by this binary against the supplied trust material. No production service " +
	"was contacted, no public log was written; the bundle is trustworthy exactly as far as the injected environment is."

func refuseIssue(resp *IssueResponse, code, format string, args ...any) {
	resp.Issued = false
	resp.Refusal = &Refusal{Code: code, Message: fmt.Sprintf(format, args...)}
}

// checkEndpointPolicy mirrors the NOMOS-side policy; production is refused with no override.
func checkEndpointPolicy(rawURL string, allowHosts []string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "ENDPOINT_NOT_ALLOWED", fmt.Errorf("endpoint %q is not an http(s) URL with a host", rawURL)
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range productionSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return "PRODUCTION_FORBIDDEN", fmt.Errorf("endpoint %s is a Sigstore public instance; production issuance is not available in this tool", host)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return "", nil
		}
		return "ENDPOINT_NOT_ALLOWED", fmt.Errorf("endpoint %s is a non-loopback IP", host)
	}
	for _, suffix := range allowedSuffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return "", nil
		}
	}
	for _, a := range allowHosts {
		if strings.EqualFold(strings.TrimSpace(a), host) {
			return "", nil
		}
	}
	return "ENDPOINT_NOT_ALLOWED", fmt.Errorf("endpoint %s is neither loopback, a reserved test domain, nor allow-listed", host)
}

func issueRequest(req IssueRequest) IssueResponse {
	resp := IssueResponse{SchemaVersion: IssueResponseSchema, Verifier: newResponse().Verifier, NonProduction: true, ClaimBoundary: issueClaimBoundary}
	resp.Endpoints.Fulcio, resp.Endpoints.Rekor = req.FulcioURL, req.RekorURL
	for name, u := range map[string]string{"fulcio": req.FulcioURL, "rekor": req.RekorURL} {
		if strings.TrimSpace(u) == "" {
			refuseIssue(&resp, "REQUEST_INCOMPLETE", "%s_url is required", name)
			return resp
		}
		if code, err := checkEndpointPolicy(u, req.AllowHosts); err != nil {
			refuseIssue(&resp, code, "%v", err)
			return resp
		}
	}
	if req.ArtifactPath == "" || req.OutBundlePath == "" || req.TrustedRootPath == "" {
		refuseIssue(&resp, "REQUEST_INCOMPLETE", "artifact_path, out_bundle_path and trusted_root_path are required")
		return resp
	}
	if strings.TrimSpace(req.IDToken) == "" {
		refuseIssue(&resp, "REQUEST_INCOMPLETE", "id_token is required; this tool never obtains one")
		return resp
	}
	if (req.Identity.SAN == "" && req.Identity.SANRegex == "") || (req.Identity.Issuer == "" && req.Identity.IssuerRegex == "") {
		refuseIssue(&resp, "REQUEST_INCOMPLETE", "certificate_identity (san/issuer) is required so the issued certificate can be checked")
		return resp
	}
	data, err := os.ReadFile(req.ArtifactPath)
	if err != nil {
		refuseIssue(&resp, "ARTIFACT_UNREADABLE", "%v", err)
		return resp
	}
	sum := sha256.Sum256(data)
	resp.ArtifactDigest = "sha256:" + hex.EncodeToString(sum[:])

	keypair, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		refuseIssue(&resp, "KEYPAIR", "%v", err)
		return resp
	}
	opts := sign.BundleOptions{
		CertificateProvider:        sign.NewFulcio(&sign.FulcioOptions{BaseURL: req.FulcioURL, Timeout: 20 * time.Second, Retries: 1}),
		CertificateProviderOptions: &sign.CertificateProviderOptions{IDToken: req.IDToken},
		TransparencyLogs:           []sign.Transparency{sign.NewRekor(&sign.RekorOptions{BaseURL: req.RekorURL, Timeout: 20 * time.Second, Retries: 1, Version: 1})},
	}
	b, err := sign.Bundle(&sign.PlainData{Data: data}, keypair, opts)
	if err != nil {
		refuseIssue(&resp, classifyIssue(err), "%v", err)
		return resp
	}
	raw, err := protojson.Marshal(b)
	if err != nil {
		refuseIssue(&resp, "BUNDLE_ENCODE", "%v", err)
		return resp
	}
	if err := os.WriteFile(req.OutBundlePath, raw, 0o644); err != nil {
		refuseIssue(&resp, "BUNDLE_WRITE", "%v", err)
		return resp
	}
	// Verify what was just issued, with this binary's verify path, against the
	// SUPPLIED trust material. The injected environment runs no CT log, so no
	// SCT is required — that is recorded in the verification response.
	v := verifyRequest(Request{
		SchemaVersion: RequestSchema, BundlePath: req.OutBundlePath, TrustedRootPath: req.TrustedRootPath,
		ArtifactPath: req.ArtifactPath, CertificateIdentity: req.Identity,
		Require: Require{TlogEntries: 1, SignedCertificateTimestamps: 0, ObserverTimestamps: 1, NoCTLog: true},
	})
	resp.Verification = &v
	if !v.Verified {
		os.Remove(req.OutBundlePath)
		code := "ISSUED_BUNDLE_UNVERIFIABLE"
		msg := "issued bundle failed verification"
		if v.Refusal != nil {
			msg = v.Refusal.Code + ": " + v.Refusal.Message
		}
		refuseIssue(&resp, code, "%s — bundle removed", msg)
		return resp
	}
	resp.BundlePath = req.OutBundlePath
	resp.Certificate = v.Certificate
	resp.Issued = true
	return resp
}

func classifyIssue(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"), strings.Contains(msg, "dial tcp"), strings.Contains(msg, "timeout"), strings.Contains(msg, "eof"):
		return "SERVICE_UNAVAILABLE"
	case strings.Contains(msg, "fulcio"), strings.Contains(msg, "401"), strings.Contains(msg, "certificate"),
		strings.Contains(msg, "identity provider"), strings.Contains(msg, "unauthorized"), strings.Contains(msg, "token"), strings.Contains(msg, "signingcert"):
		return "CERTIFICATE_ISSUANCE_FAILED"
	case strings.Contains(msg, "rekor"), strings.Contains(msg, "log entry"), strings.Contains(msg, "tlog"):
		return "LOG_ENTRY_FAILED"
	default:
		return "ISSUANCE_FAILED"
	}
}
