package corpus

import (
	"testing"
)

const rbokRealStyleDoc = `# Référentiel RBOK

| Champ | Valeur |
|---|---|
| Reference | REF-001 |
| Statut | Actif |

## Chapitre 1 - Garanties

La garantie habitation couvre les risques suivants :

| Risque | Franchise | Plafond |
|---|---|---|
| Dégât des eaux | 150€ | 50 000€ |
| Incendie | 300€ | 100 000€ |

### Section 1.1 - Conditions

> [!NOTE] Conditions spéciales
> Les conditions suivantes s'appliquent aux résidences secondaires.

L'assuré doit déclarer le sinistre dans un délai de 5 jours.

Exemple de code :

` + "```yaml\ngarantie:\n  type: degat_eaux\n  franchise: 150\n```" + `

Voir [les annexes](annexes.md) pour plus de détails.

![Schéma de couverture](schema.png)

## Chapitre 2 - Exclusions

Pas de couverture pour :
- Guerre et terrorisme
- Catastrophe nucléaire
`

func TestTypedNodesDetectedFromRealStyle(t *testing.T) {
	result := ExtractMarkdown(rbokRealStyleDoc, "rbok-ref")
	EmitTypedNodesFromExtraction(&result)

	tables, codes, callouts, links, images := CountSemanticNodes(result.Nodes)
	t.Logf("total=%d tables=%d code=%d callout=%d links=%d images=%d",
		len(result.Nodes), tables, codes, callouts, links, images)

	if tables == 0 {
		t.Fatal("expected tables from pipe-delimited content")
	}
	if codes == 0 {
		t.Fatal("expected code blocks from fenced content")
	}
	if callouts == 0 {
		t.Fatal("expected callouts from > [!NOTE] content")
	}
	if links == 0 {
		t.Fatal("expected links from [text](url) content")
	}
	if images == 0 {
		t.Fatal("expected images from ![alt](url) content")
	}
}

func TestTypedNodesNoDuplicates(t *testing.T) {
	result := ExtractMarkdown(rbokRealStyleDoc, "rbok-ref")
	EmitTypedNodesFromExtraction(&result)

	tables, codes, callouts, links, images := CountSemanticNodes(result.Nodes)
	// With paragraph-only scan (not alinea), each block should appear once.
	if tables > 1 {
		t.Fatalf("expected 1 table, got %d (duplicate?)", tables)
	}
	if codes > 1 {
		t.Fatalf("expected 1 code block, got %d (duplicate?)", codes)
	}
	if callouts > 1 {
		t.Fatalf("expected 1 callout, got %d (duplicate?)", callouts)
	}
	if links > 1 {
		t.Fatalf("expected 1 link, got %d (duplicate?)", links)
	}
	if images > 1 {
		t.Fatalf("expected 1 image, got %d (duplicate?)", images)
	}
}

// Test with plain blockquote (no [!NOTE] marker) — common in real RBOK.
func TestTypedNodesPlainBlockquote(t *testing.T) {
	doc := `# Document

## Section

> Cet article est applicable à compter du 1er janvier 2026.
> Il remplace les dispositions antérieures.

Texte normal après la citation.
`
	result := ExtractMarkdown(doc, "plain-bq")
	EmitTypedNodesFromExtraction(&result)

	_, _, callouts, _, _ := CountSemanticNodes(result.Nodes)
	if callouts == 0 {
		t.Fatal("expected plain blockquote detected as callout")
	}
}

// Test with real RBOK-style table (no header row text, just pipes).
func TestTypedNodesRBOKTable(t *testing.T) {
	doc := `# Barème

## Tarifs

| Zone | Prime annuelle | Franchise |
|------|---------------|-----------|
| Zone 1 | 250€ | 150€ |
| Zone 2 | 350€ | 200€ |
| Zone 3 | 500€ | 300€ |

Les tarifs sont indicatifs.
`
	result := ExtractMarkdown(doc, "bareme")
	EmitTypedNodesFromExtraction(&result)

	tables, _, _, _, _ := CountSemanticNodes(result.Nodes)
	if tables == 0 {
		t.Fatal("expected table from pipe-separated content")
	}
}

// Test with inline links in body text.
func TestTypedNodesInlineLinks(t *testing.T) {
	doc := `# Document

## Références

Voir [Code des assurances](https://legifrance.gouv.fr/codes/id/LEGITEXT000006073984) et
[Directive Solvabilité II](https://eur-lex.europa.eu/legal-content/FR/TXT/?uri=CELEX:32009L0138).
`
	result := ExtractMarkdown(doc, "refs")
	EmitTypedNodesFromExtraction(&result)

	_, _, _, links, _ := CountSemanticNodes(result.Nodes)
	if links < 2 {
		t.Fatalf("expected >= 2 links, got %d", links)
	}
}

// Test that code blocks with language annotation are detected.
func TestTypedNodesCodeBlockWithLang(t *testing.T) {
	doc := "# Config\n\n## Exemple\n\n```json\n{\"type\": \"habitation\"}\n```\n"
	result := ExtractMarkdown(doc, "config")
	EmitTypedNodesFromExtraction(&result)

	_, codes, _, _, _ := CountSemanticNodes(result.Nodes)
	if codes == 0 {
		t.Fatal("expected code block")
	}
	// Check language metadata.
	for _, n := range result.Nodes {
		if n.NodeType == NodeCodeBlock {
			lang, _ := n.Metadata["language"].(string)
			if lang != "json" {
				t.Fatalf("expected language=json, got %q", lang)
			}
		}
	}
}

// Test empty document produces no typed nodes.
func TestTypedNodesEmptyDoc(t *testing.T) {
	result := ExtractMarkdown("# Empty\n\nNo special content here.\n", "empty")
	EmitTypedNodesFromExtraction(&result)

	tables, codes, callouts, links, images := CountSemanticNodes(result.Nodes)
	if tables+codes+callouts+links+images != 0 {
		t.Fatalf("expected 0 typed nodes for plain text, got t=%d c=%d q=%d l=%d i=%d",
			tables, codes, callouts, links, images)
	}
}
