package corpus

// #610 — the web-source contract (Recursio → NOMOS).
//
// A crawled page is not a file on disk. It was fetched from somewhere, at some
// instant, by some crawler, under some scope and robots/licence decision, and
// what NOMOS holds afterwards is a capture — never the site's ongoing truth.
// Until now the source manifest had no field to say any of that: a web source
// could only masquerade as a local `html` file with a hash and no provenance.
//
// WebSource is the additive block that carries the provenance. It is optional on
// every source (existing manifests stay byte-identical, doctrine §2.1), and when
// present it is validated fail-closed: a web source without a canonical URL, a
// stable content hash, a fetch instant, a crawler version, or a decided
// robots/licence status is REFUSED by the manifest and feed gates. The five
// admission buckets the issue asks for (canonical content, external reference,
// binary/media, unsupported, excluded-by-policy) are not re-invented here: they
// are the FSQ-02 fields already on every source — source_role, atomization
// status, admission status — and a web source carries them like any other.
//
// The field-for-field correspondence with connector.FetchResult is deliberate
// and documented on each field so an export from the fetch layer maps without
// interpretation; the corpus package does not import connector, so the mapping
// is a contract rather than a call.

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Robots and licence decisions. `undecided` is a legal value to WRITE — a
// crawler may honestly not know yet — but it is never a legal value to ADMIT:
// Validate refuses it, so an undecided page cannot enter a feed by omission.
const (
	WebDecisionAllowed    = "allowed"
	WebDecisionDisallowed = "disallowed"
	WebDecisionUndecided  = "undecided"
)

// Scope policies: how the page came to be in scope of the crawl.
const (
	WebScopeSeed     = "seed"      // explicitly listed as a starting point
	WebScopeInScope  = "in_scope"  // discovered, inside the declared crawl scope
	WebScopeOutScope = "out_scope" // discovered, outside scope — recorded, never admitted
)

// Stable error code substrings. Downstream audits key off these strings, so
// they are part of the public contract, like the FSQ-02 codes.
const (
	ErrCodeWebNoCanonicalURL  = "WEB_SOURCE_NO_CANONICAL_URL"
	ErrCodeWebBadURL          = "WEB_SOURCE_BAD_URL"
	ErrCodeWebNoContentHash   = "WEB_SOURCE_NO_CONTENT_HASH"
	ErrCodeWebUnstableHash    = "WEB_SOURCE_UNSTABLE_HASH"
	ErrCodeWebNoFetchedAt     = "WEB_SOURCE_NO_FETCHED_AT"
	ErrCodeWebNoCrawlerVer    = "WEB_SOURCE_NO_CRAWLER_VERSION"
	ErrCodeWebBadHTTPStatus   = "WEB_SOURCE_BAD_HTTP_STATUS"
	ErrCodeWebRobotsUndecided = "WEB_SOURCE_ROBOTS_UNDECIDED"
	ErrCodeWebLicenceUndecid  = "WEB_SOURCE_LICENCE_UNDECIDED"
	ErrCodeWebDisallowedAdmit = "WEB_SOURCE_DISALLOWED_BUT_ADMITTED"
	ErrCodeWebBadScope        = "WEB_SOURCE_BAD_SCOPE_POLICY"
	ErrCodeWebOutScopeAdmit   = "WEB_SOURCE_OUT_OF_SCOPE_BUT_ADMITTED"
	ErrCodeWebDepthNoParent   = "WEB_SOURCE_DEPTH_WITHOUT_PARENT"
	ErrCodeWebNegativeDepth   = "WEB_SOURCE_NEGATIVE_DEPTH"
)

// WebSourceClaimBoundary travels with every emitted web source so the limit is
// on the artifact, not only in this file.
const WebSourceClaimBoundary = "Web source captured at fetched_at by the named crawler; " +
	"a point-in-time capture, never the site's ongoing truth. Robots and licence " +
	"decisions are recorded as made, not adjudicated by NOMOS."

// webHashRe is the only accepted shape: an algorithm prefix and hex digest. A
// bare hash with no algorithm cannot be re-verified and is not stable.
var webHashRe = regexp.MustCompile(`^(sha256|sha384|sha512):[0-9a-f]{64,128}$`)

// WebSource is the provenance block of a source fetched from the web.
//
// Correspondence with connector.FetchResult (nomos-connector-evidence-v1):
//
//	CanonicalURL          — the URL the source is KNOWN BY (after redirects,
//	                        canonical-link resolution); FetchResult.URL is the
//	                        URL that was requested. Both are kept because they
//	                        legitimately differ.
//	FetchedURL            — FetchResult.URL
//	HTTPStatus            — FetchResult.StatusCode
//	ContentType           — FetchResult.ContentType
//	ETag / LastModified   — FetchResult.ETag / FetchResult.LastModified
//	ContentHash           — FetchResult.SHA256 over the raw bytes as fetched
//	NormalizedContentHash — hash over the crawler's normalised text; what the
//	                        Markdown export and the feed unit derive from. Two
//	                        fetches that differ only in boilerplate share it.
//	FetchedAt             — FetchResult.FetchedAt (RFC 3339, UTC)
type WebSource struct {
	CanonicalURL          string `yaml:"canonical_url" json:"canonical_url"`
	FetchedURL            string `yaml:"fetched_url,omitempty" json:"fetched_url,omitempty"`
	HTTPStatus            int    `yaml:"http_status" json:"http_status"`
	ContentType           string `yaml:"content_type,omitempty" json:"content_type,omitempty"`
	ETag                  string `yaml:"etag,omitempty" json:"etag,omitempty"`
	LastModified          string `yaml:"last_modified,omitempty" json:"last_modified,omitempty"`
	ContentHash           string `yaml:"content_hash" json:"content_hash"`
	NormalizedContentHash string `yaml:"normalized_content_hash,omitempty" json:"normalized_content_hash,omitempty"`
	FetchedAt             string `yaml:"fetched_at" json:"fetched_at"`
	CrawlerVersion        string `yaml:"crawler_version" json:"crawler_version"`
	ScopePolicy           string `yaml:"scope_policy" json:"scope_policy"`
	RobotsDecision        string `yaml:"robots_decision" json:"robots_decision"`
	LicenceDecision       string `yaml:"licence_decision" json:"licence_decision"`
	ParentURL             string `yaml:"parent_url,omitempty" json:"parent_url,omitempty"`
	Depth                 int    `yaml:"depth" json:"depth"`
	// ClaimBoundary is filled by Normalize when empty, so a source authored by
	// hand still carries the limit once it passes through the engine.
	ClaimBoundary string `yaml:"claim_boundary,omitempty" json:"claim_boundary,omitempty"`
}

var (
	validWebDecisions = map[string]bool{
		WebDecisionAllowed: true, WebDecisionDisallowed: true, WebDecisionUndecided: true,
	}
	validWebScopes = map[string]bool{
		WebScopeSeed: true, WebScopeInScope: true, WebScopeOutScope: true,
	}
)

// Normalize fills the claim boundary. It changes nothing else: the engine does
// not rewrite provenance, it only refuses to carry it silently.
func (w *WebSource) Normalize() {
	if w != nil && strings.TrimSpace(w.ClaimBoundary) == "" {
		w.ClaimBoundary = WebSourceClaimBoundary
	}
}

// Validate is the fail-closed contract. `admitted` says whether the enclosing
// source is admitted (FSQ-02); an admitted web source has to satisfy more —
// the robots and licence decisions must be favourable, and the page must be in
// scope — because admission is what lets it into a feed.
func (w WebSource) Validate(admitted bool) error {
	canonical := strings.TrimSpace(w.CanonicalURL)
	if canonical == "" {
		return fmt.Errorf("%s: canonical_url required — a web source must be known by an address",
			ErrCodeWebNoCanonicalURL)
	}
	if err := validateHTTPURL(canonical); err != nil {
		return fmt.Errorf("%s: canonical_url %q: %v", ErrCodeWebBadURL, canonical, err)
	}
	if fetched := strings.TrimSpace(w.FetchedURL); fetched != "" {
		if err := validateHTTPURL(fetched); err != nil {
			return fmt.Errorf("%s: fetched_url %q: %v", ErrCodeWebBadURL, fetched, err)
		}
	}
	if parent := strings.TrimSpace(w.ParentURL); parent != "" {
		if err := validateHTTPURL(parent); err != nil {
			return fmt.Errorf("%s: parent_url %q: %v", ErrCodeWebBadURL, parent, err)
		}
	}

	if w.HTTPStatus < 100 || w.HTTPStatus > 599 {
		return fmt.Errorf("%s: http_status %d is not an HTTP status code",
			ErrCodeWebBadHTTPStatus, w.HTTPStatus)
	}

	hash := strings.TrimSpace(w.ContentHash)
	if hash == "" {
		return fmt.Errorf("%s: content_hash required — a capture without a hash cannot be re-verified",
			ErrCodeWebNoContentHash)
	}
	if !webHashRe.MatchString(hash) {
		return fmt.Errorf("%s: content_hash %q must be algo:hex (sha256|sha384|sha512) — "+
			"a bare or placeholder hash is not stable", ErrCodeWebUnstableHash, hash)
	}
	if norm := strings.TrimSpace(w.NormalizedContentHash); norm != "" && !webHashRe.MatchString(norm) {
		return fmt.Errorf("%s: normalized_content_hash %q must be algo:hex",
			ErrCodeWebUnstableHash, norm)
	}

	fetchedAt := strings.TrimSpace(w.FetchedAt)
	if fetchedAt == "" {
		return fmt.Errorf("%s: fetched_at required — a capture is a point in time or it is nothing",
			ErrCodeWebNoFetchedAt)
	}
	if _, err := time.Parse(time.RFC3339, fetchedAt); err != nil {
		return fmt.Errorf("%s: fetched_at %q is not RFC 3339: %v", ErrCodeWebNoFetchedAt, fetchedAt, err)
	}

	if strings.TrimSpace(w.CrawlerVersion) == "" {
		return fmt.Errorf("%s: crawler_version required — the capture must name what produced it",
			ErrCodeWebNoCrawlerVer)
	}

	if !validWebScopes[w.ScopePolicy] {
		return fmt.Errorf("%s: scope_policy %q not in {seed, in_scope, out_scope}",
			ErrCodeWebBadScope, w.ScopePolicy)
	}
	if !validWebDecisions[w.RobotsDecision] {
		return fmt.Errorf("%s: robots_decision %q not in {allowed, disallowed, undecided}",
			ErrCodeWebRobotsUndecided, w.RobotsDecision)
	}
	if !validWebDecisions[w.LicenceDecision] {
		return fmt.Errorf("%s: licence_decision %q not in {allowed, disallowed, undecided}",
			ErrCodeWebLicenceUndecid, w.LicenceDecision)
	}

	if w.Depth < 0 {
		return fmt.Errorf("%s: depth %d", ErrCodeWebNegativeDepth, w.Depth)
	}
	if w.Depth > 0 && strings.TrimSpace(w.ParentURL) == "" {
		return fmt.Errorf("%s: depth %d without parent_url — a page reached by following a link "+
			"must say which", ErrCodeWebDepthNoParent, w.Depth)
	}

	// Admission raises the bar: what enters a feed must have been allowed in.
	if admitted {
		if w.RobotsDecision == WebDecisionUndecided {
			return fmt.Errorf("%s: an admitted web source cannot leave robots undecided",
				ErrCodeWebRobotsUndecided)
		}
		if w.LicenceDecision == WebDecisionUndecided {
			return fmt.Errorf("%s: an admitted web source cannot leave the licence undecided",
				ErrCodeWebLicenceUndecid)
		}
		if w.RobotsDecision == WebDecisionDisallowed || w.LicenceDecision == WebDecisionDisallowed {
			return fmt.Errorf("%s: robots=%s licence=%s — a disallowed page can be recorded, never admitted",
				ErrCodeWebDisallowedAdmit, w.RobotsDecision, w.LicenceDecision)
		}
		if w.ScopePolicy == WebScopeOutScope {
			return fmt.Errorf("%s: an out-of-scope page can be recorded, never admitted",
				ErrCodeWebOutScopeAdmit)
		}
	}
	return nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q is not http(s)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("no host")
	}
	return nil
}
