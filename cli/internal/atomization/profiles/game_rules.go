package profiles

// GameRules returns the profile for tabletop/RPG game rule books.
// Inspired by Kingdoms & Warfare (K&W) and similar structured rule systems.
func GameRules() Profile {
	return Profile{
		ID:          "game-rules",
		Name:        "Game Rules (K&W style)",
		Description: "Profile for tabletop RPG and strategy game rule books with structured mechanics, abilities, and reference tables.",
		Domain:      "game-design",
		NodeTypes: []NodeTypeDef{
			{Name: "rulebook", Description: "Top-level rule book", Structural: true, CanContain: []string{"chapter", "appendix"}},
			{Name: "chapter", Description: "Major rules chapter", Structural: true, CanContain: []string{"section", "rule", "sidebar"}},
			{Name: "section", Description: "Named section within a chapter", Structural: true, CanContain: []string{"rule", "ability", "class_feature", "table", "example"}},
			{Name: "rule", Description: "A single game rule or mechanic", Structural: false, CanContain: []string{"sub_rule", "exception"}},
			{Name: "sub_rule", Description: "Clarification or sub-case of a rule", Structural: false},
			{Name: "exception", Description: "Exception to a rule", Structural: false},
			{Name: "ability", Description: "Named ability, spell, or action", Structural: false},
			{Name: "class_feature", Description: "Class or unit feature/trait", Structural: false},
			{Name: "stat_block", Description: "Unit or creature stat block", Structural: false},
			{Name: "table", Description: "Reference table (levels, costs, effects)", Structural: false},
			{Name: "sidebar", Description: "Designer commentary or variant rule", Structural: false},
			{Name: "example", Description: "Play example or scenario", Structural: false},
			{Name: "appendix", Description: "Reference appendix", Structural: true, CanContain: []string{"table", "rule", "definition"}},
			{Name: "definition", Description: "Game term definition", Structural: false},
			{Name: "glossary_entry", Description: "Glossary entry", Structural: false},
		},
		Hierarchy: []HierarchyLevel{
			{Depth: 0, NodeType: "rulebook", Label: "Rulebook"},
			{Depth: 1, NodeType: "chapter", Label: "Chapter"},
			{Depth: 2, NodeType: "section", Label: "Section"},
			{Depth: 3, NodeType: "rule", Label: "Rule"},
			{Depth: 4, NodeType: "sub_rule", Label: "Sub-rule"},
		},
		Metadata: []MetadataField{
			{Name: "game_system", Required: true, Type: "string"},
			{Name: "edition", Required: true, Type: "string"},
			{Name: "version", Required: true, Type: "string"},
			{Name: "author", Required: true, Type: "string"},
			{Name: "publisher", Required: false, Type: "string"},
			{Name: "status", Required: true, Type: "enum:official,playtest,homebrew,errata"},
			{Name: "domain", Required: false, Type: "string"},
			{Name: "license", Required: false, Type: "string"},
		},
	}
}
