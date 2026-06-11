package atomization

import (
	stdhtml "html"
	"strings"
	"testing"
)

// Synthetic ELI-shaped RDF/XML fixture: the SHAPE of a Fedlex entry (jolux
// vocabulary, rdf:about ELI URIs) with authored, license-safe content — same
// discipline as the portable golden corpus. The fake act number makes any
// accidental full-text claim impossible.
const eliRDF = `<?xml version="1.0" encoding="utf-8" ?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:jolux="http://data.legilux.public.lu/resource/ontology/jolux#">
  <rdf:Description rdf:about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000">
    <jolux:title>Loi exemple sur les espaces construits</jolux:title>
    <jolux:dateDocument>2026-01-01</jolux:dateDocument>
  </rdf:Description>
  <rdf:Description rdf:about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000/art_1">
    <jolux:title>Art. 1 Hauteurs &amp; gabarits</jolux:title>
  </rdf:Description>
</rdf:RDF>`

const plainXML = `<root><item>premier</item><item>second</item><note>sans identite</note></root>`

const sampleHTML = `<!doctype html>
<html>
  <body>
    <div about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000">
      <p>Premier alinea.</p>
      <p>Deuxieme alinea.</p>
    </div>
    <p>Hors identite.</p>
  </body>
</html>`

func opts(ref string) AtomizeOptions {
	return AtomizeOptions{DocumentRef: ref, SourceFile: ref + ".xml", Domain: "built-environment"}
}

// The C2 bar: every atom's span re-slices the ORIGINAL bytes back to its text
// (after entity decoding) — offsets are proven, not declared.
func assertSpanFidelity(t *testing.T, source []byte, set AtomSet) {
	t.Helper()
	for _, a := range set.Atoms {
		sp := a.SourceSpan
		if sp.StartByte < 0 || sp.EndByte > len(source) || sp.StartByte >= sp.EndByte {
			t.Fatalf("%s: span [%d,%d) out of bounds (%d bytes)", a.ID, sp.StartByte, sp.EndByte, len(source))
		}
		raw := string(source[sp.StartByte:sp.EndByte])
		if got := strings.TrimSpace(stdhtml.UnescapeString(raw)); got != a.Text {
			t.Fatalf("%s: span does not re-slice to the text\n span: %q\n text: %q", a.ID, got, a.Text)
		}
		if sp.DomPath == "" {
			t.Fatalf("%s: markup atom without dom_path", a.ID)
		}
	}
}

func TestAtomizeXML_ELIIdentityPreservedInLocators(t *testing.T) {
	set, err := AtomizeXML([]byte(eliRDF), opts("eli-fixture"))
	if err != nil {
		t.Fatalf("AtomizeXML: %v", err)
	}
	if set.AtomCount != 3 {
		t.Fatalf("atom count = %d, want 3 (title + date + art title)", set.AtomCount)
	}
	assertSpanFidelity(t, []byte(eliRDF), set)

	// The first description's children anchor on ITS ELI; the article's title
	// anchors on the ARTICLE ELI (nearest carrying ancestor wins).
	title := set.Atoms[0]
	if !strings.HasPrefix(title.CanonicalRef, "https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000#") {
		t.Fatalf("title canonical_ref lost the ELI identity: %q", title.CanonicalRef)
	}
	if title.SourceSpan.DomPath != "/RDF/Description/title" {
		t.Fatalf("title dom_path = %q", title.SourceSpan.DomPath)
	}
	art := set.Atoms[2]
	if !strings.HasPrefix(art.CanonicalRef, "https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000/art_1#") {
		t.Fatalf("article canonical_ref lost the article ELI: %q", art.CanonicalRef)
	}
	if art.SourceSpan.DomPath != "/RDF/Description[2]/title" {
		t.Fatalf("article dom_path = %q (sibling index lost?)", art.SourceSpan.DomPath)
	}
	// Entity decoding: the text carries '&', the span carries '&amp;'.
	if !strings.Contains(art.Text, "Hauteurs & gabarits") {
		t.Fatalf("entities not decoded: %q", art.Text)
	}
}

func TestAtomizeXML_NoIdentityFallsBackToDocRef_NeverInvented(t *testing.T) {
	set, err := AtomizeXML([]byte(plainXML), opts("plain"))
	if err != nil {
		t.Fatalf("AtomizeXML: %v", err)
	}
	assertSpanFidelity(t, []byte(plainXML), set)
	for _, a := range set.Atoms {
		if !strings.HasPrefix(a.CanonicalRef, "plain#") {
			t.Fatalf("expected doc-ref anchor, got %q (identity must never be invented)", a.CanonicalRef)
		}
	}
	// Sibling indices keep repeated elements distinct.
	if set.Atoms[0].SourceSpan.DomPath != "/root/item" || set.Atoms[1].SourceSpan.DomPath != "/root/item[2]" {
		t.Fatalf("sibling paths wrong: %q / %q", set.Atoms[0].SourceSpan.DomPath, set.Atoms[1].SourceSpan.DomPath)
	}
}

func TestAtomizeXML_MalformedFailsClosed(t *testing.T) {
	if _, err := AtomizeXML([]byte("<root><unclosed>"), opts("bad")); err == nil {
		t.Fatal("malformed XML produced atoms instead of an error")
	}
}

func TestAtomizeXML_DeterministicAndTamperEvident(t *testing.T) {
	a, _ := AtomizeXML([]byte(eliRDF), opts("eli-fixture"))
	b, _ := AtomizeXML([]byte(eliRDF), opts("eli-fixture"))
	if len(a.Atoms) != len(b.Atoms) {
		t.Fatal("non-deterministic atom count")
	}
	for i := range a.Atoms {
		if a.Atoms[i].ID != b.Atoms[i].ID || a.Atoms[i].ContentHash != b.Atoms[i].ContentHash {
			t.Fatalf("non-deterministic atom %d", i)
		}
	}
	// Adversarial: one byte changed INSIDE a span → that atom's hash changes
	// and the set's source hash changes (tamper-evident, revert-confirm style).
	tampered := strings.Replace(eliRDF, "Hauteurs", "Hauteur5", 1)
	c, err := AtomizeXML([]byte(tampered), opts("eli-fixture"))
	if err != nil {
		t.Fatalf("tampered parse: %v", err)
	}
	if c.SourceHash == a.SourceHash {
		t.Fatal("source hash did not change under tampering")
	}
	changed := false
	for i := range a.Atoms {
		if c.Atoms[i].ContentHash != a.Atoms[i].ContentHash {
			changed = true
		}
	}
	if !changed {
		t.Fatal("no atom hash changed under in-span tampering")
	}
}

func TestAtomizeHTML_TagPathsAndIdentity(t *testing.T) {
	set, err := AtomizeHTML([]byte(sampleHTML), opts("page"))
	if err != nil {
		t.Fatalf("AtomizeHTML: %v", err)
	}
	assertSpanFidelity(t, []byte(sampleHTML), set)
	if set.AtomCount != 3 {
		t.Fatalf("atom count = %d, want 3", set.AtomCount)
	}
	first, second, outside := set.Atoms[0], set.Atoms[1], set.Atoms[2]
	if !strings.HasPrefix(first.CanonicalRef, "https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000#") {
		t.Fatalf("identity from the div's about= not preserved: %q", first.CanonicalRef)
	}
	if first.SourceSpan.DomPath != "/html/body/div/p" || second.SourceSpan.DomPath != "/html/body/div/p[2]" {
		t.Fatalf("tag paths wrong: %q / %q", first.SourceSpan.DomPath, second.SourceSpan.DomPath)
	}
	if !strings.HasPrefix(outside.CanonicalRef, "page#") {
		t.Fatalf("identity leaked outside its subtree: %q", outside.CanonicalRef)
	}
}

func TestAtomizeXML_FacetsOptIn(t *testing.T) {
	o := opts("eli-fixture")
	o.EmitFacets = true
	set, err := AtomizeXML([]byte(eliRDF), o)
	if err != nil {
		t.Fatalf("AtomizeXML: %v", err)
	}
	if set.Atoms[0].Facets == nil {
		t.Fatal("facets requested but absent")
	}
	off, _ := AtomizeXML([]byte(eliRDF), opts("eli-fixture"))
	if off.Atoms[0].Facets != nil {
		t.Fatal("facets emitted without opt-in (regression on the additive default)")
	}
}
