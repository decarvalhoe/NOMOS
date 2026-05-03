package profiles

// RBOKLawbook returns the profile for the RBOK lawbook corpus.
// Specific to French insurance regulatory and contractual texts.
func RBOKLawbook() Profile {
	return Profile{
		ID:          "rbok-lawbook",
		Name:        "RBOK Lawbook",
		Description: "Profile for RBOK lawbook corpus: French insurance law, regulations, contracts, and actuarial references.",
		Domain:      "insurance",
		NodeTypes: []NodeTypeDef{
			{Name: "code", Description: "Legislative code (e.g. Code des assurances)", Structural: true, CanContain: []string{"livre", "titre", "article"}},
			{Name: "livre", Description: "Book of a code", Structural: true, CanContain: []string{"titre", "chapitre"}},
			{Name: "titre", Description: "Title division", Structural: true, CanContain: []string{"chapitre", "section"}},
			{Name: "chapitre", Description: "Chapter", Structural: true, CanContain: []string{"section", "article"}},
			{Name: "section", Description: "Section", Structural: true, CanContain: []string{"article", "clause"}},
			{Name: "article", Description: "Legal article", Structural: false, CanContain: []string{"alinea"}},
			{Name: "alinea", Description: "Sub-paragraph of an article", Structural: false},
			{Name: "clause", Description: "Contractual clause", Structural: false, CanContain: []string{"sub_clause"}},
			{Name: "sub_clause", Description: "Sub-clause", Structural: false},
			{Name: "garantie", Description: "Insurance guarantee/warranty", Structural: false},
			{Name: "exclusion", Description: "Coverage exclusion", Structural: false},
			{Name: "franchise", Description: "Deductible rule", Structural: false},
			{Name: "definition", Description: "Defined term", Structural: false},
			{Name: "bareme", Description: "Rate table or tariff", Structural: false},
			{Name: "parcours_etape", Description: "Business path stage", Structural: true, CanContain: []string{"parcours_critere"}},
			{Name: "parcours_critere", Description: "Verifiable criterion", Structural: false},
		},
		Hierarchy: []HierarchyLevel{
			{Depth: 0, NodeType: "code", Label: "Code"},
			{Depth: 1, NodeType: "livre", Label: "Livre"},
			{Depth: 2, NodeType: "titre", Label: "Titre"},
			{Depth: 3, NodeType: "chapitre", Label: "Chapitre"},
			{Depth: 4, NodeType: "section", Label: "Section"},
			{Depth: 5, NodeType: "article", Label: "Article"},
			{Depth: 6, NodeType: "alinea", Label: "Alinéa"},
		},
		Metadata: []MetadataField{
			{Name: "jurisdiction", Required: true, Type: "string"},
			{Name: "version", Required: true, Type: "string"},
			{Name: "owner", Required: true, Type: "string"},
			{Name: "status", Required: true, Type: "enum:active,deprecated,abrogated,pending"},
			{Name: "domain", Required: true, Type: "string"},
			{Name: "confidentiality", Required: true, Type: "enum:public,internal,restricted,secret"},
			{Name: "branche", Required: false, Type: "string"},
			{Name: "effective_date", Required: false, Type: "date"},
		},
	}
}
