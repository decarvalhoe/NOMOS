package corpus

// #610 — a web source without provenance or a stable hash is refused.
//
// Doctrine §2.3: the proof is the failure. Every rule that refuses something has
// a test that removes exactly one thing and shows the refusal, with its stable
// error code — the codes are the public contract downstream audits key off.

import (
	"strings"
	"testing"
)

func validWebSource() WebSource {
	return WebSource{
		CanonicalURL:          "https://example.invalid/reglement/art-7",
		FetchedURL:            "https://www.example.invalid/reglement/art-7?utm=x",
		HTTPStatus:            200,
		ContentType:           "text/html; charset=utf-8",
		ETag:                  `"abc123"`,
		LastModified:          "Fri, 04 Sep 2026 10:00:00 GMT",
		ContentHash:           "sha256:" + strings.Repeat("ab", 32),
		NormalizedContentHash: "sha256:" + strings.Repeat("cd", 32),
		FetchedAt:             "2026-09-04T10:00:00Z",
		CrawlerVersion:        "recursio/0.4.2",
		ScopePolicy:           WebScopeInScope,
		RobotsDecision:        WebDecisionAllowed,
		LicenceDecision:       WebDecisionAllowed,
		ParentURL:             "https://example.invalid/reglement/",
		Depth:                 1,
	}
}

func TestWebSource_ValidCaptureIsAccepted(t *testing.T) {
	w := validWebSource()
	if err := w.Validate(true); err != nil {
		t.Fatalf("a complete capture was refused: %v", err)
	}
	if err := w.Validate(false); err != nil {
		t.Fatalf("a complete capture was refused when not admitted: %v", err)
	}
}

func TestWebSource_NormalizeCarriesTheClaimBoundary(t *testing.T) {
	w := validWebSource()
	if w.ClaimBoundary != "" {
		t.Fatal("fixture should start without a boundary")
	}
	w.Normalize()
	if !strings.Contains(w.ClaimBoundary, "never the site's ongoing truth") {
		t.Fatalf("boundary not carried: %q", w.ClaimBoundary)
	}
	// Normalize never rewrites a boundary someone set deliberately.
	w.ClaimBoundary = "custom"
	w.Normalize()
	if w.ClaimBoundary != "custom" {
		t.Fatal("Normalize overwrote an explicit boundary")
	}
}

func TestWebSource_EachMissingProvenanceFieldIsRefusedWithItsCode(t *testing.T) {
	// The acceptance criterion of #610, one field at a time: a source without
	// provenance or a stable hash is refused, and the refusal is machine-readable.
	cases := map[string]struct {
		mutate func(*WebSource)
		code   string
	}{
		"no canonical url":       {func(w *WebSource) { w.CanonicalURL = "  " }, ErrCodeWebNoCanonicalURL},
		"canonical url not http": {func(w *WebSource) { w.CanonicalURL = "ftp://example.invalid/x" }, ErrCodeWebBadURL},
		"canonical url no host":  {func(w *WebSource) { w.CanonicalURL = "https:///path" }, ErrCodeWebBadURL},
		"fetched url malformed":  {func(w *WebSource) { w.FetchedURL = "not a url at all" }, ErrCodeWebBadURL},
		"parent url malformed":   {func(w *WebSource) { w.ParentURL = "javascript:alert(1)" }, ErrCodeWebBadURL},
		"http status zero":       {func(w *WebSource) { w.HTTPStatus = 0 }, ErrCodeWebBadHTTPStatus},
		"http status absurd":     {func(w *WebSource) { w.HTTPStatus = 999 }, ErrCodeWebBadHTTPStatus},
		"no content hash":        {func(w *WebSource) { w.ContentHash = "" }, ErrCodeWebNoContentHash},
		"bare hash no algorithm": {func(w *WebSource) { w.ContentHash = strings.Repeat("ab", 32) }, ErrCodeWebUnstableHash},
		"placeholder hash":       {func(w *WebSource) { w.ContentHash = "placeholder:not-fetched" }, ErrCodeWebUnstableHash},
		"short hash":             {func(w *WebSource) { w.ContentHash = "sha256:abcd" }, ErrCodeWebUnstableHash},
		"uppercase hex":          {func(w *WebSource) { w.ContentHash = "sha256:" + strings.Repeat("AB", 32) }, ErrCodeWebUnstableHash},
		"bad normalized hash":    {func(w *WebSource) { w.NormalizedContentHash = "md5:whatever" }, ErrCodeWebUnstableHash},
		"no fetched_at":          {func(w *WebSource) { w.FetchedAt = "" }, ErrCodeWebNoFetchedAt},
		"fetched_at not rfc3339": {func(w *WebSource) { w.FetchedAt = "04/09/2026" }, ErrCodeWebNoFetchedAt},
		"no crawler version":     {func(w *WebSource) { w.CrawlerVersion = "" }, ErrCodeWebNoCrawlerVer},
		"unknown scope":          {func(w *WebSource) { w.ScopePolicy = "whatever" }, ErrCodeWebBadScope},
		"unknown robots value":   {func(w *WebSource) { w.RobotsDecision = "maybe" }, ErrCodeWebRobotsUndecided},
		"unknown licence value":  {func(w *WebSource) { w.LicenceDecision = "probably" }, ErrCodeWebLicenceUndecid},
		"negative depth":         {func(w *WebSource) { w.Depth = -1 }, ErrCodeWebNegativeDepth},
		"depth without parent":   {func(w *WebSource) { w.Depth = 2; w.ParentURL = "" }, ErrCodeWebDepthNoParent},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := validWebSource()
			tc.mutate(&w)
			err := w.Validate(false)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("%s: expected code %s, got: %v", name, tc.code, err)
			}
		})
	}
}

func TestWebSource_UndecidedIsRecordableButNeverAdmitted(t *testing.T) {
	// A crawler may honestly not know yet. That is a legal thing to write down
	// and an illegal thing to let into a feed.
	for _, field := range []string{"robots", "licence"} {
		t.Run(field, func(t *testing.T) {
			w := validWebSource()
			code := ErrCodeWebRobotsUndecided
			if field == "robots" {
				w.RobotsDecision = WebDecisionUndecided
			} else {
				w.LicenceDecision = WebDecisionUndecided
				code = ErrCodeWebLicenceUndecid
			}
			if err := w.Validate(false); err != nil {
				t.Fatalf("recording an undecided %s should be legal: %v", field, err)
			}
			err := w.Validate(true)
			if err == nil {
				t.Fatalf("an admitted source with undecided %s was accepted", field)
			}
			if !strings.Contains(err.Error(), code) {
				t.Fatalf("expected %s, got %v", code, err)
			}
		})
	}
}

func TestWebSource_DisallowedIsRecordableButNeverAdmitted(t *testing.T) {
	for _, field := range []string{"robots", "licence"} {
		t.Run(field, func(t *testing.T) {
			w := validWebSource()
			if field == "robots" {
				w.RobotsDecision = WebDecisionDisallowed
			} else {
				w.LicenceDecision = WebDecisionDisallowed
			}
			if err := w.Validate(false); err != nil {
				t.Fatalf("recording a disallowed page should be legal: %v", err)
			}
			err := w.Validate(true)
			if err == nil {
				t.Fatal("a disallowed page was admitted")
			}
			if !strings.Contains(err.Error(), ErrCodeWebDisallowedAdmit) {
				t.Fatalf("expected %s, got %v", ErrCodeWebDisallowedAdmit, err)
			}
		})
	}
}

func TestWebSource_OutOfScopeIsRecordableButNeverAdmitted(t *testing.T) {
	w := validWebSource()
	w.ScopePolicy = WebScopeOutScope
	if err := w.Validate(false); err != nil {
		t.Fatalf("recording an out-of-scope page should be legal: %v", err)
	}
	if err := w.Validate(true); err == nil || !strings.Contains(err.Error(), ErrCodeWebOutScopeAdmit) {
		t.Fatalf("expected %s, got %v", ErrCodeWebOutScopeAdmit, err)
	}
}

func TestWebSource_SeedNeedsNoParent(t *testing.T) {
	w := validWebSource()
	w.ScopePolicy = WebScopeSeed
	w.Depth = 0
	w.ParentURL = ""
	if err := w.Validate(true); err != nil {
		t.Fatalf("a depth-0 seed needs no parent: %v", err)
	}
}

// --- manifest and feed integration ---------------------------------------

func TestManifestSource_WithoutWebSourceIsUnchanged(t *testing.T) {
	// Zero regression: a local file source has no web_source and validates
	// exactly as before.
	m := ManifestSource{
		ID: "LOCAL-1", Path: "docs/a.md", Type: "markdown", Hash: "sha256:" + strings.Repeat("00", 32),
		AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationAtomized,
		SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("local source regressed: %v", err)
	}
}

func TestManifestSource_AdmittedWebSourceWithoutProvenanceIsRefused(t *testing.T) {
	// The gate the issue asks for: the manifest refuses a web source without
	// provenance, even though every FSQ-02 field is fine.
	w := validWebSource()
	w.ContentHash = ""
	m := ManifestSource{
		ID: "WEB-1", Path: "captures/art-7.md", Type: "html", Hash: "sha256:" + strings.Repeat("00", 32),
		AdmissionStatus: AdmissionAdmitted, AtomizationStatus: AtomizationAtomized,
		SourceRole: AdmissionRoleCanonical, FormatSupport: FormatSupported,
		WebSource: &w,
	}
	err := m.Validate()
	if err == nil {
		t.Fatal("manifest accepted a web source with no content hash")
	}
	if !strings.Contains(err.Error(), ErrCodeWebNoContentHash) {
		t.Fatalf("expected %s, got %v", ErrCodeWebNoContentHash, err)
	}
}

func TestManifestSource_ExcludedWebSourceMayBeUndecided(t *testing.T) {
	// A page excluded by policy is recorded with whatever the crawler knew; the
	// admitted-only rules do not apply because it never enters a feed.
	w := validWebSource()
	w.RobotsDecision = WebDecisionUndecided
	m := ManifestSource{
		ID: "WEB-2", Path: "captures/x.md", Type: "html", Hash: "sha256:" + strings.Repeat("00", 32),
		AdmissionStatus: AdmissionExcluded, ExclusionReason: "excluded_by_policy: robots undecided",
		SourceRole: AdmissionRoleReference, FormatSupport: FormatSupported,
		WebSource: &w,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("an excluded, undecided page should be recordable: %v", err)
	}
}
