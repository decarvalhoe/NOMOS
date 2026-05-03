package fidelity

import (
	"regexp"
	"strings"
)

// SemRole is the semantic role assigned to a block or atom.
type SemRole string

const (
	SemDefinition SemRole = "definition"
	SemRule       SemRole = "rule"
	SemException  SemRole = "exception"
	SemExample    SemRole = "example"
	SemNote       SemRole = "note"
	SemWarning    SemRole = "warning"
	SemProcedure  SemRole = "procedure"
	SemReference  SemRole = "reference"
	SemStructure  SemRole = "structure"
	SemUnknown    SemRole = "unknown"
)

// AllSemRoles returns all valid semantic roles.
func AllSemRoles() []SemRole {
	return []SemRole{
		SemDefinition, SemRule, SemException, SemExample,
		SemNote, SemWarning, SemProcedure, SemReference,
		SemStructure, SemUnknown,
	}
}

// IsValid returns true if the role is recognized.
func (r SemRole) IsValid() bool {
	for _, v := range AllSemRoles() {
		if r == v {
			return true
		}
	}
	return false
}

// ClassifiedNode is a CAST node enriched with semantic role and context.
type ClassifiedNode struct {
	NodeID      string  `json:"node_id"`
	Role        SemRole `json:"role"`
	Confidence  string  `json:"confidence"`
	Text        string  `json:"text"`
	Kind        NodeKind `json:"kind"`
	Depth       int     `json:"depth"`
	ParentID    string  `json:"parent_id,omitempty"`
	ParentRole  SemRole `json:"parent_role,omitempty"`
	SiblingPrev SemRole `json:"sibling_prev,omitempty"`
	SiblingNext SemRole `json:"sibling_next,omitempty"`
}

// ClassifierResult holds the full classification output.
type ClassifierResult struct {
	TotalNodes int              `json:"total_nodes"`
	Classified int              `json:"classified"`
	ByRole     map[SemRole]int  `json:"by_role"`
	Nodes      []ClassifiedNode `json:"nodes"`
}

// Ordered pattern groups — checked in priority order.
// Each group: if ANY pattern matches, the role is assigned.

var notePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|\b)(?:note\s*[:.]\s|remarque|n\.b\.\s|nb\s*:)`),
	regexp.MustCompile(`(?i)\b(?:observation|pr[ée]cision|commentaire)\b`),
}

var warningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|\b)(?:attention|avertissement|warning|danger|alerte|mise en garde)\b`),
	regexp.MustCompile(`(?i)\b(?:important\s*:)\b`),
	regexp.MustCompile(`(?i)\b(?:risque|caution)\b`),
}

var procedurePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:proc[ée]dure|[ée]tape[s]?\s*\d|step\s*\d|processus|workflow)\b`),
	regexp.MustCompile(`(?i)(?:^|\b)(?:1[\.\)]\s|premi[eè]rement|d'abord|ensuite|enfin)\b`),
	regexp.MustCompile(`(?i)\b(?:suivre les [ée]tapes|follow the steps|instructions)\b`),
}

var exceptionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:sauf|except[ée]?|exception|unless|notwithstanding|par d[ée]rogation)\b`),
	regexp.MustCompile(`(?i)\b(?:ne sont? pas couvert|ne s'applique pas|does not apply|is excluded|exclusion)\b`),
	regexp.MustCompile(`(?i)\b(?:sous r[ée]serve|subject to|[àa] condition|hors)\b`),
}

var definitionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|\b)(?:on entend par|au sens d[ue]|d[ée]signe|is defined as|refers? to)\b`),
	regexp.MustCompile(`(?i)\b(?:the term\b.*\bmeans?\b)`),
	regexp.MustCompile(`(?i)\b(?:d[ée]finition|glossaire|terminologie|vocabulary)\b`),
	regexp.MustCompile(`(?i)^[A-Z][a-z]{5,}[^.]{0,60}\s*:\s+[a-z]`),
}

var rulePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:doit|doivent|shall|must|is required|est tenu|sont tenus|obligatoire)\b`),
	regexp.MustCompile(`(?i)\b(?:il est interdit|ne peut pas|ne doivent pas|shall not|must not|prohibited)\b`),
	regexp.MustCompile(`(?i)\b(?:pris en charge|est couvert|sont couverts|is covered|are covered)\b`),
	regexp.MustCompile(`(?i)\b(?:dans un d[ée]lai de|within \d+ days?|au plus tard)\b`),
}

var examplePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:par exemple|exemple|example|e\.g\.|for instance|such as|illustr)\b`),
	regexp.MustCompile(`(?i)\b(?:cas pratique|use case|sc[ée]nario)\b`),
}

var referencePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:conform[ée]ment [àa]|en application de|pursuant to|in accordance with)\b`),
	regexp.MustCompile(`(?i)(?:\bcf\.\s)`),
	regexp.MustCompile(`(?i)\b(?:voir|see also)\b`),
	regexp.MustCompile(`(?i)\b(?:article \d+|art\.\s*\d+)\b`),
	regexp.MustCompile(`(?i)\b(?:ISO\s*\d+|RFC\s*\d+|GDPR|HIPAA|SOC|MDR|IVDR)\b`),
}

// classifyRules is the ordered pipeline of role detectors.
// Order matters: more specific roles are checked first.
var classifyRules = []struct {
	role     SemRole
	patterns []*regexp.Regexp
}{
	{SemNote, notePatterns},
	{SemWarning, warningPatterns},
	{SemProcedure, procedurePatterns},
	{SemException, exceptionPatterns},
	{SemExample, examplePatterns},
	{SemDefinition, definitionPatterns},
	{SemRule, rulePatterns},
	{SemReference, referencePatterns},
}

// ClassifyBlocks assigns a semantic role to every content node in a CAST.
func ClassifyBlocks(cast CAST) ClassifierResult {
	// Build indices.
	nodeIdx := make(map[string]CNode, len(cast.Nodes))
	childrenOf := make(map[string][]string)
	for _, n := range cast.Nodes {
		nodeIdx[n.ID] = n
		if n.ParentID != "" {
			childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n.ID)
		}
	}

	// First pass: classify each node.
	roleOf := make(map[string]SemRole, len(cast.Nodes))
	for _, n := range cast.Nodes {
		roleOf[n.ID] = classifyBlock(n, nodeIdx)
	}

	// Second pass: build classified nodes with context.
	var nodes []ClassifiedNode
	byRole := map[SemRole]int{}
	classified := 0

	for _, n := range cast.Nodes {
		if n.Kind == KindDocument || n.Kind == KindThematicBreak {
			continue
		}

		role := roleOf[n.ID]
		conf := roleConfidence(n, role)
		depth := nodeDepth(n.ID, nodeIdx)

		cn := ClassifiedNode{
			NodeID:     n.ID,
			Role:       role,
			Confidence: conf,
			Text:       n.Text,
			Kind:       n.Kind,
			Depth:      depth,
			ParentID:   n.ParentID,
		}

		// Parent context.
		if n.ParentID != "" {
			cn.ParentRole = roleOf[n.ParentID]
		}

		// Sibling context.
		if siblings, ok := childrenOf[n.ParentID]; ok {
			for si, sibID := range siblings {
				if sibID != n.ID {
					continue
				}
				if si > 0 {
					cn.SiblingPrev = roleOf[siblings[si-1]]
				}
				if si < len(siblings)-1 {
					cn.SiblingNext = roleOf[siblings[si+1]]
				}
				break
			}
		}

		nodes = append(nodes, cn)
		byRole[role]++
		if role != SemUnknown {
			classified++
		}
	}

	return ClassifierResult{
		TotalNodes: len(nodes),
		Classified: classified,
		ByRole:     byRole,
		Nodes:      nodes,
	}
}

func classifyBlock(n CNode, idx map[string]CNode) SemRole {
	// Structural nodes.
	if n.Kind == KindHeading || n.Kind == KindDocument {
		return SemStructure
	}
	if n.Kind == KindCodeBlock {
		return SemExample
	}
	if n.Kind == KindBlockquote {
		return SemNote
	}
	if n.Kind == KindTable || n.Kind == KindTableRow || n.Kind == KindTableCell {
		return SemReference
	}
	if n.Kind == KindHTML {
		return SemNote
	}

	text := n.Text
	if text == "" {
		text = n.RawText
	}

	// Heading context boost.
	headingCtx := ""
	if parent, ok := idx[n.ParentID]; ok && parent.Kind == KindHeading {
		headingCtx = strings.ToLower(parent.Text)
	}

	return classifyText(text, headingCtx)
}

func classifyText(text, headingCtx string) SemRole {
	// Heading context can force a role.
	if headingCtx != "" {
		lh := strings.ToLower(headingCtx)
		if containsStr(lh, "d\u00e9finition", "definition", "glossaire") {
			return SemDefinition
		}
		if containsStr(lh, "exception", "exclusion", "limitation") {
			return SemException
		}
		if containsStr(lh, "exemple", "example") {
			return SemExample
		}
		if containsStr(lh, "proc\u00e9dure", "procedure", "workflow", "\u00e9tape") {
			return SemProcedure
		}
		if containsStr(lh, "avertissement", "warning", "attention") {
			return SemWarning
		}
	}

	// Rule-based pattern matching in priority order.
	for _, rule := range classifyRules {
		for _, p := range rule.patterns {
			if p.MatchString(text) {
				// Exception + rule co-occurrence: exception wins when no rule keyword.
				if rule.role == SemException {
					hasRule := false
					for _, rp := range rulePatterns {
						if rp.MatchString(text) {
							hasRule = true
							break
						}
					}
					if hasRule {
						continue // let rule win
					}
				}
				return rule.role
			}
		}
	}

	return SemUnknown
}

func roleConfidence(n CNode, role SemRole) string {
	if role == SemUnknown {
		return "low"
	}
	if role == SemStructure {
		return "high"
	}
	// Structural node types have inherent confidence.
	switch n.Kind {
	case KindCodeBlock, KindBlockquote, KindTable:
		return "high"
	}
	switch role {
	case SemRule, SemDefinition, SemWarning:
		return "high"
	case SemException, SemExample, SemProcedure:
		return "medium"
	case SemNote, SemReference:
		return "medium"
	}
	return "medium"
}

func containsStr(text string, substrs ...string) bool {
	for _, s := range substrs {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

func nodeDepth(id string, idx map[string]CNode) int {
	d := 0
	cur := id
	seen := map[string]bool{}
	for {
		n, ok := idx[cur]
		if !ok || n.ParentID == "" || seen[cur] {
			break
		}
		seen[cur] = true
		cur = n.ParentID
		d++
	}
	return d
}
