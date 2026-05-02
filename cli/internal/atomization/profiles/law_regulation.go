package profiles

// LawRegulation returns the profile for legal and regulatory texts.
// Covers statutes, codes, regulations, directives, circulars.
func LawRegulation() Profile {
	return Profile{
		ID:          "law-regulation",
		Name:        "Law & Regulation",
		Description: "Profile for legal and regulatory texts: statutes, codes, regulations, directives, and circulars.",
		Domain:      "legal",
		NodeTypes: []NodeTypeDef{
			{Name: "code", Description: "Top-level legislative code", Structural: true, CanContain: []string{"book", "title", "part", "article"}},
			{Name: "book", Description: "Book division of a code", Structural: true, CanContain: []string{"title", "part", "chapter"}},
			{Name: "title", Description: "Title division", Structural: true, CanContain: []string{"chapter", "section", "article"}},
			{Name: "part", Description: "Part division", Structural: true, CanContain: []string{"chapter", "section", "article"}},
			{Name: "chapter", Description: "Chapter division", Structural: true, CanContain: []string{"section", "article"}},
			{Name: "section", Description: "Section within a chapter", Structural: true, CanContain: []string{"article", "paragraph"}},
			{Name: "article", Description: "Single legal article (atomic unit)", Structural: false, CanContain: []string{"paragraph", "alinea"}},
			{Name: "paragraph", Description: "Numbered paragraph within an article", Structural: false, CanContain: []string{"alinea"}},
			{Name: "alinea", Description: "Unnumbered sub-paragraph", Structural: false},
			{Name: "annex", Description: "Annex or schedule", Structural: true, CanContain: []string{"article", "paragraph"}},
			{Name: "definition", Description: "Legal definition", Structural: false},
			{Name: "table", Description: "Rate table or tariff schedule", Structural: false},
		},
		Hierarchy: []HierarchyLevel{
			{Depth: 0, NodeType: "code", Label: "Code"},
			{Depth: 1, NodeType: "book", Label: "Livre"},
			{Depth: 2, NodeType: "title", Label: "Titre"},
			{Depth: 3, NodeType: "chapter", Label: "Chapitre"},
			{Depth: 4, NodeType: "section", Label: "Section"},
			{Depth: 5, NodeType: "article", Label: "Article"},
			{Depth: 6, NodeType: "paragraph", Label: "Paragraphe"},
		},
		Metadata: []MetadataField{
			{Name: "jurisdiction", Required: true, Type: "string"},
			{Name: "publication_date", Required: false, Type: "date"},
			{Name: "effective_date", Required: false, Type: "date"},
			{Name: "version", Required: true, Type: "string"},
			{Name: "owner", Required: true, Type: "string"},
			{Name: "status", Required: true, Type: "enum:active,deprecated,abrogated,pending"},
			{Name: "domain", Required: true, Type: "string"},
			{Name: "confidentiality", Required: false, Type: "enum:public,internal,restricted,secret"},
			{Name: "language", Required: false, Type: "string"},
			{Name: "issuer", Required: false, Type: "string"},
		},
	}
}
