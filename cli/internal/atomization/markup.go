// VRC-31 / #568 — the HTML/XML adapter (plan C2): online legal sources (Fedlex
// serves HTML/XML) become atoms with DOM-path + byte-offset locators, and the
// ELI identity of the nearest carrying ancestor is PRESERVED in the canonical
// reference — never invented when absent.
//
// Claim boundary of this slice: structural ingestion only. Atoms carry the
// decoded text, a span that provably maps back to the original bytes (the
// tests re-slice the source and compare), and the identity found in the
// document. No legal interpretation, no completeness claim about the act.
//
// XML rides the stdlib decoder (exact token offsets, namespace-aware — this is
// what the committed Fedlex RDF/XML receipts need). HTML rides the tree-sitter
// grammar already in the toolchain (doc 45 §4 C2).
package atomization

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/html"
)

// MarkupFormat selects the markup adapter.
type MarkupFormat string

const (
	FormatXML  MarkupFormat = "xml"
	FormatHTML MarkupFormat = "html"
)

// eliPrefix recognizes a European Legislation Identifier carried by a document
// attribute (Fedlex addresses every act under /eli/). Detection is a prefix
// match on the attribute VALUE, not a guess from context.
const eliMarker = "/eli/"

// markupAtom is one text-bearing unit found in the markup before it becomes an
// Atom (kept internal so the public surface stays Atom/AtomSet).
type markupAtom struct {
	text      string // decoded text (entities resolved)
	domPath   string // /rdf:RDF/rdf:Description[2]/jolux:title
	identity  string // nearest ancestor ELI URI, "" when the document carries none
	startByte int    // span over the ORIGINAL bytes, half-open [start, end)
	endByte   int
	depth     int
}

// AtomizeXML converts an XML document (e.g. a Fedlex ELI RDF/XML entry) into
// an AtomSet. Every non-blank character-data run becomes one atom whose span
// is the byte range of the raw token in the original input.
func AtomizeXML(source []byte, opts AtomizeOptions) (AtomSet, error) {
	units, err := xmlUnits(source)
	if err != nil {
		return AtomSet{}, err
	}
	return markupAtomSet(source, units, opts), nil
}

// AtomizeHTML converts an HTML document into an AtomSet using the tree-sitter
// HTML grammar (byte ranges are native to the parse tree).
func AtomizeHTML(source []byte, opts AtomizeOptions) (AtomSet, error) {
	units, err := htmlUnits(source)
	if err != nil {
		return AtomSet{}, err
	}
	return markupAtomSet(source, units, opts), nil
}

// xmlUnits walks the token stream, tracking the element stack (with per-name
// sibling indices for deterministic DOM paths) and the nearest ELI identity.
func xmlUnits(source []byte) ([]markupAtom, error) {
	type frame struct {
		name     string
		path     string
		identity string
		// children counts sibling elements per name to index repeated nodes.
		children map[string]int
	}
	dec := xml.NewDecoder(strings.NewReader(string(source)))
	// Fedlex serves UTF-8; foreign charsets are out of this slice's claim.
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return nil, fmt.Errorf("unsupported charset %q (utf-8 only in this slice)", charset)
	}

	var stack []frame
	var out []markupAtom
	prevEnd := int64(0)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml: %w", err)
		}
		tokenStart, tokenEnd := prevEnd, dec.InputOffset()
		prevEnd = tokenEnd

		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			parentPath, parentIdentity := "", ""
			var siblings map[string]int
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				parentPath, parentIdentity = top.path, top.identity
				siblings = top.children
			} else {
				siblings = map[string]int{}
			}
			siblings[name]++
			seg := name
			if n := siblings[name]; n > 1 {
				seg = fmt.Sprintf("%s[%d]", name, n)
			}
			identity := parentIdentity
			for _, attr := range t.Attr {
				local := attr.Name.Local
				if (local == "about" || local == "resource" || local == "href" || local == "id") &&
					strings.Contains(attr.Value, eliMarker) {
					identity = attr.Value
					break
				}
			}
			stack = append(stack, frame{
				name:     name,
				path:     parentPath + "/" + seg,
				identity: identity,
				children: map[string]int{},
			})
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text == "" || len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			out = append(out, markupAtom{
				text:      text,
				domPath:   top.path,
				identity:  top.identity,
				startByte: int(tokenStart),
				endByte:   int(tokenEnd),
				depth:     len(stack),
			})
		}
	}
	return out, nil
}

// htmlUnits parses with the tree-sitter HTML grammar and emits one unit per
// non-blank text node, with tag paths and native byte ranges.
func htmlUnits(source []byte) ([]markupAtom, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(html.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("html: %w", err)
	}
	defer tree.Close()

	var out []markupAtom
	var walk func(n *sitter.Node, path string, identity string, depth int, siblings map[string]int)
	walk = func(n *sitter.Node, path string, identity string, depth int, siblings map[string]int) {
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			switch child.Type() {
			case "element", "script_element", "style_element":
				tag, attrs := htmlTagInfo(child, source)
				if tag == "" {
					tag = child.Type()
				}
				siblings[tag]++
				seg := tag
				if c := siblings[tag]; c > 1 {
					seg = fmt.Sprintf("%s[%d]", tag, c)
				}
				childIdentity := identity
				for _, v := range attrs {
					if strings.Contains(v, eliMarker) {
						childIdentity = v
						break
					}
				}
				walk(child, path+"/"+seg, childIdentity, depth+1, map[string]int{})
			case "text":
				text := strings.TrimSpace(string(source[child.StartByte():child.EndByte()]))
				if text == "" {
					continue
				}
				out = append(out, markupAtom{
					text:      text,
					domPath:   path,
					identity:  identity,
					startByte: int(child.StartByte()),
					endByte:   int(child.EndByte()),
					depth:     depth,
				})
			default:
				walk(child, path, identity, depth, siblings)
			}
		}
	}
	walk(tree.RootNode(), "", "", 0, map[string]int{})
	return out, nil
}

// htmlTagInfo extracts the tag name and the attribute VALUES of an element's
// start tag (identity attributes live there). The tree-sitter `attribute` node
// covers `name="value"`; the value lives in its (quoted_)attribute_value child
// — only that part is an identity candidate, never the attribute name.
func htmlTagInfo(element *sitter.Node, source []byte) (string, []string) {
	var tag string
	var values []string
	for i := 0; i < int(element.ChildCount()); i++ {
		st := element.Child(i)
		if st.Type() != "start_tag" && st.Type() != "self_closing_tag" {
			continue
		}
		for j := 0; j < int(st.ChildCount()); j++ {
			part := st.Child(j)
			switch part.Type() {
			case "tag_name":
				tag = string(source[part.StartByte():part.EndByte()])
			case "attribute":
				for k := 0; k < int(part.ChildCount()); k++ {
					v := part.Child(k)
					switch v.Type() {
					case "attribute_value":
						values = append(values, string(source[v.StartByte():v.EndByte()]))
					case "quoted_attribute_value":
						raw := strings.Trim(string(source[v.StartByte():v.EndByte()]), `"'`)
						values = append(values, raw)
					}
				}
			}
		}
	}
	return tag, values
}

// markupAtomSet materializes Atoms with the shared conventions: stable IDs from
// the canonical ref, content hashes, line/col spans derived from byte offsets,
// and the ELI identity preserved in the canonical reference when present.
func markupAtomSet(source []byte, units []markupAtom, opts AtomizeOptions) AtomSet {
	defaultState := opts.DefaultState
	if !defaultState.IsValid() {
		defaultState = ReviewDraft
	}
	lineStarts := byteLineIndex(source)
	set := AtomSet{
		DocumentRef: opts.DocumentRef,
		SourceFile:  opts.SourceFile,
		SourceHash:  hashContent(string(source)),
	}
	// Deterministic order: document order (units already arrive in order; keep
	// a stable sort on startByte as a guard against walker reordering).
	sort.SliceStable(units, func(i, j int) bool { return units[i].startByte < units[j].startByte })
	seen := map[string]int{}
	for _, u := range units {
		anchor := opts.DocumentRef
		if u.identity != "" {
			anchor = u.identity // the ELI identity, preserved verbatim
		}
		canonicalRef := anchor + "#" + u.domPath
		// Repeated char-data runs under the same element (split by comments or
		// child elements) get a disambiguating ordinal — IDs must stay unique.
		seen[canonicalRef]++
		if n := seen[canonicalRef]; n > 1 {
			canonicalRef = fmt.Sprintf("%s~%d", canonicalRef, n)
		}
		startLine, startCol := lineColAt(lineStarts, u.startByte)
		endLine, endCol := lineColAt(lineStarts, u.endByte)
		atom := Atom{
			ID:           stableAtomID(canonicalRef),
			CanonicalRef: canonicalRef,
			Type:         AtomClause,
			Text:         u.text,
			ContentHash:  hashContent(u.text),
			SourceSpan: SourceSpan{
				File:      opts.SourceFile,
				StartLine: startLine,
				EndLine:   endLine,
				StartCol:  startCol,
				EndCol:    endCol,
				DomPath:   u.domPath,
				StartByte: u.startByte,
				EndByte:   u.endByte,
			},
			BlockID:     fmt.Sprintf("M-%06d", u.startByte),
			Depth:       u.depth,
			ReviewState: defaultState,
			Domain:      opts.Domain,
		}
		if opts.EmitFacets {
			f := DeriveFacets(atom)
			atom.Facets = &f
		}
		set.Atoms = append(set.Atoms, atom)
	}
	set.AtomCount = len(set.Atoms)
	return set
}

// byteLineIndex returns the byte offset of each line start (1-based lines).
func byteLineIndex(source []byte) []int {
	starts := []int{0}
	for i, b := range source {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineColAt converts a byte offset into 1-based line and column numbers.
func lineColAt(lineStarts []int, offset int) (int, int) {
	idx := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > offset }) - 1
	if idx < 0 {
		idx = 0
	}
	return idx + 1, offset - lineStarts[idx] + 1
}
