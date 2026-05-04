package corpus

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParcoursFile is the top-level YAML structure for a parcours file.
type ParcoursFile struct {
	Parcours Parcours `yaml:"parcours"`
}

// Parcours describes a structured learning or business path.
type Parcours struct {
	ID                string           `yaml:"id"`
	Code              string           `yaml:"code"`
	Name              string           `yaml:"name"`
	Description       string           `yaml:"description"`
	ObjectifPrincipal string           `yaml:"objectif_principal"`
	Domain            string           `yaml:"domain"`
	Version           string           `yaml:"version"`
	Owner             string           `yaml:"owner"`
	Status            string           `yaml:"status"`
	Etapes            []Etape          `yaml:"etapes"`
	Modules           []ParcoursModule `yaml:"modules"`
}

// ParcoursModule is the RBOK production parcours module shape.
type ParcoursModule struct {
	Code             string              `yaml:"code"`
	Name             string              `yaml:"name"`
	Type             string              `yaml:"type"`
	Description      string              `yaml:"description"`
	AIInstructions   string              `yaml:"ai_instructions"`
	SourceRBOK       string              `yaml:"source_rbok"`
	ContenuReference string              `yaml:"contenu_reference"`
	Objectives       []ParcoursObjective `yaml:"objectives"`
}

// ParcoursObjective is a production parcours objective.
type ParcoursObjective struct {
	Key         string             `yaml:"key"`
	Titre       string             `yaml:"titre"`
	Description string             `yaml:"description"`
	Questions   []ParcoursQuestion `yaml:"questions"`
}

// ParcoursQuestion is a production parcours question.
type ParcoursQuestion struct {
	Key      string `yaml:"key"`
	Label    string `yaml:"label"`
	Type     string `yaml:"type"`
	HelpText string `yaml:"help_text"`
}

// Etape is a stage within a parcours.
type Etape struct {
	ID          string     `yaml:"id"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Ordre       int        `yaml:"ordre"`
	Objectifs   []Objectif `yaml:"objectifs"`
}

// Objectif is a goal within an étape.
type Objectif struct {
	ID          string    `yaml:"id"`
	Description string    `yaml:"description"`
	Criteres    []Critere `yaml:"criteres"`
}

// Critere is a single verifiable criterion within an objectif.
type Critere struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Criticality string `yaml:"criticality"`
}

// ParcoursUnit is a canonical unit extracted from a parcours.
// Each critère becomes one unit, traceable to its étape and objectif.
//
// FSQ-04 (#367): YAML scalar provenance fields (RawText, DecodedValue,
// YAMLPath, NodeKind, SchemaRole, BusinessRuleMode) are populated by
// ExtractParcoursFromBytes from the position-aware YAML AST. They expose
// which YAML scalar fed BusinessRule, with raw and decoded forms kept
// separate so reordering or duplicate values cannot collide.
type ParcoursUnit struct {
	UnitID       string `json:"unit_id"       yaml:"unit_id"`
	UnitType     string `json:"unit_type"     yaml:"unit_type"`
	Name         string `json:"name"          yaml:"name"`
	Domain       string `json:"domain"        yaml:"domain"`
	Criticality  string `json:"criticality"   yaml:"criticality"`
	BusinessRule string `json:"business_rule" yaml:"business_rule"`
	EtapeID      string `json:"etape_id"      yaml:"etape_id"`
	EtapeName    string `json:"etape_name"    yaml:"etape_name"`
	ObjectifID   string `json:"objectif_id"   yaml:"objectif_id"`
	ParcoursID   string `json:"parcours_id"   yaml:"parcours_id"`
	Owner        string `json:"owner"         yaml:"owner"`
	Status       string `json:"status"        yaml:"status"`

	// FSQ-04 YAML scalar provenance — see package doc on ParcoursUnit.
	RawText          string `json:"raw_text,omitempty"           yaml:"raw_text,omitempty"`
	DecodedValue     string `json:"decoded_value,omitempty"      yaml:"decoded_value,omitempty"`
	YAMLPath         string `json:"yaml_path,omitempty"          yaml:"yaml_path,omitempty"`
	NodeKind         string `json:"node_kind,omitempty"          yaml:"node_kind,omitempty"`
	SchemaRole       string `json:"schema_role,omitempty"        yaml:"schema_role,omitempty"`
	BusinessRuleMode string `json:"business_rule_mode,omitempty" yaml:"business_rule_mode,omitempty"`
	StartByte        int    `json:"start_byte,omitempty"         yaml:"start_byte,omitempty"`
	EndByte          int    `json:"end_byte,omitempty"           yaml:"end_byte,omitempty"`
	StartLine        int    `json:"start_line,omitempty"         yaml:"start_line,omitempty"`
	EndLine          int    `json:"end_line,omitempty"           yaml:"end_line,omitempty"`
	StartColumn      int    `json:"start_column,omitempty"       yaml:"start_column,omitempty"`
}

// ExtractResult holds extracted units and metadata.
type ExtractResult struct {
	ParcoursID   string         `json:"parcours_id"   yaml:"parcours_id"`
	ParcoursName string         `json:"parcours_name" yaml:"parcours_name"`
	Domain       string         `json:"domain"        yaml:"domain"`
	TotalEtapes  int            `json:"total_etapes"  yaml:"total_etapes"`
	TotalUnits   int            `json:"total_units"   yaml:"total_units"`
	Units        []ParcoursUnit `json:"units"         yaml:"units"`
}

// ExtractParcours reads a parcours YAML file and extracts canonical units.
func ExtractParcours(path string) (ExtractResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExtractResult{}, fmt.Errorf("read parcours: %w", err)
	}
	return ExtractParcoursFromBytes(data)
}

// ExtractParcoursFromBytes extracts canonical units from parcours YAML bytes.
//
// FSQ-04 (#367): the extractor parses the YAML twice — once into typed
// structs (the existing path-driven binding) and once into a position-aware
// *yaml.Node tree. The node tree is used to index scalars by yaml_path so
// each emitted unit can carry the raw byte slice, decoded value, key path,
// and node kind of the YAML scalar that fed its BusinessRule. Selection is
// strictly by key path: two scalars with identical decoded values but
// different paths bind to distinct units (no value-collision).
func ExtractParcoursFromBytes(data []byte) (ExtractResult, error) {
	var file ParcoursFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return ExtractResult{}, fmt.Errorf("parse parcours: %w", err)
	}

	p := file.Parcours
	parcoursID := firstNonEmpty(p.ID, p.Code)
	if parcoursID == "" {
		return ExtractResult{}, fmt.Errorf("parcours.id is required")
	}
	domain := firstNonEmpty(p.Domain, "rbok")
	owner := firstNonEmpty(p.Owner, "unknown")

	scalars := indexYAMLScalars(data)

	// businessRuleMode reflects what the BusinessRule field of a parcours
	// unit currently holds. Today it is the YAML-decoded scalar value (what
	// yaml.Unmarshal produced into the typed struct field), so callers must
	// interpret BusinessRule as the decoded form.
	const businessRuleMode = "decoded"

	var units []ParcoursUnit
	for ei, etape := range p.Etapes {
		for oi, objectif := range etape.Objectifs {
			for ci, critere := range objectif.Criteres {
				path := fmt.Sprintf("parcours.etapes[%d].objectifs[%d].criteres[%d].description",
					ei, oi, ci)
				unit := ParcoursUnit{
					UnitID:           makeParcoursUnitID(parcoursID, etape.ID, critere.ID),
					UnitType:         mapCritereType(critere.Type),
					Name:             critere.Description,
					Domain:           domain,
					Criticality:      normalizeCriticality(critere.Criticality),
					BusinessRule:     critere.Description,
					EtapeID:          etape.ID,
					EtapeName:        etape.Name,
					ObjectifID:       objectif.ID,
					ParcoursID:       parcoursID,
					Owner:            owner,
					Status:           "partial",
					BusinessRuleMode: businessRuleMode,
					SchemaRole:       "criterion_description",
				}
				applyYAMLScalarMeta(&unit, scalars, path)
				units = append(units, unit)
			}
		}
	}
	for mi, module := range p.Modules {
		moduleID := firstNonEmpty(module.Code, module.Name)
		if moduleID == "" {
			continue
		}
		modulePath := fmt.Sprintf("parcours.modules[%d]", mi)
		businessRule, businessRuleField := firstNonEmptyField(
			[2]string{"ai_instructions", module.AIInstructions},
			[2]string{"description", module.Description},
			[2]string{"name", module.Name},
		)
		unit := ParcoursUnit{
			UnitID:           makeParcoursUnitID(parcoursID, "MODULE", moduleID),
			UnitType:         mapModuleType(module.Type),
			Name:             firstNonEmpty(module.Name, moduleID),
			Domain:           domain,
			Criticality:      "medium",
			BusinessRule:     businessRule,
			EtapeID:          moduleID,
			EtapeName:        module.Name,
			ObjectifID:       firstNonEmpty(module.ContenuReference, module.SourceRBOK),
			ParcoursID:       parcoursID,
			Owner:            owner,
			Status:           "partial",
			BusinessRuleMode: businessRuleMode,
			SchemaRole:       "module_" + businessRuleField,
		}
		if businessRuleField != "" {
			applyYAMLScalarMeta(&unit, scalars, modulePath+"."+businessRuleField)
		}
		units = append(units, unit)
		for oi, objective := range module.Objectives {
			for qi, question := range objective.Questions {
				questionID := firstNonEmpty(question.Key, question.Label)
				if questionID == "" {
					continue
				}
				questionPath := fmt.Sprintf("%s.objectives[%d].questions[%d]",
					modulePath, oi, qi)
				businessRule, businessRuleField := firstNonEmptyField(
					[2]string{"help_text", question.HelpText},
					[2]string{"label", question.Label},
				)
				schemaRole := "question_" + businessRuleField
				lookupPath := questionPath + "." + businessRuleField
				if businessRule == "" && objective.Description != "" {
					// Fall back to the objective-level description; record
					// its path so provenance still points to the YAML key.
					businessRule = objective.Description
					businessRuleField = "description"
					schemaRole = "objective_description"
					lookupPath = fmt.Sprintf("%s.objectives[%d].description", modulePath, oi)
				}
				unit := ParcoursUnit{
					UnitID:           makeParcoursUnitID(parcoursID, moduleID, questionID),
					UnitType:         mapQuestionType(question.Type),
					Name:             firstNonEmpty(question.Label, questionID),
					Domain:           domain,
					Criticality:      "medium",
					BusinessRule:     businessRule,
					EtapeID:          moduleID,
					EtapeName:        module.Name,
					ObjectifID:       firstNonEmpty(objective.Key, objective.Titre),
					ParcoursID:       parcoursID,
					Owner:            owner,
					Status:           "partial",
					BusinessRuleMode: businessRuleMode,
					SchemaRole:       schemaRole,
				}
				if businessRule != "" {
					applyYAMLScalarMeta(&unit, scalars, lookupPath)
				}
				units = append(units, unit)
			}
		}
	}

	return ExtractResult{
		ParcoursID:   parcoursID,
		ParcoursName: p.Name,
		Domain:       domain,
		TotalEtapes:  len(p.Etapes) + len(p.Modules),
		TotalUnits:   len(units),
		Units:        units,
	}, nil
}

// firstNonEmptyField returns the first non-empty value/name pair from the
// supplied (field-name, value) tuples. The returned field name lets callers
// reconstruct the YAML key path of the chosen scalar without a second
// reflection pass.
func firstNonEmptyField(candidates ...[2]string) (string, string) {
	for _, c := range candidates {
		if strings.TrimSpace(c[1]) != "" {
			return c[1], c[0]
		}
	}
	return "", ""
}

// applyYAMLScalarMeta copies the indexed scalar's raw bytes, decoded value,
// span, and node kind onto the ParcoursUnit at path, when an entry exists.
// Missing paths are silently skipped — the unit just lacks YAML provenance.
func applyYAMLScalarMeta(unit *ParcoursUnit, scalars map[string]yamlScalarInfo, path string) {
	loc, ok := scalars[path]
	if !ok {
		return
	}
	unit.YAMLPath = path
	unit.RawText = loc.RawText
	unit.DecodedValue = loc.DecodedValue
	unit.NodeKind = loc.NodeKind
	unit.StartByte = loc.StartByte
	unit.EndByte = loc.EndByte
	unit.StartLine = loc.StartLine
	unit.EndLine = loc.StartLine + strings.Count(loc.RawText, "\n")
	unit.StartColumn = loc.StartColumn
}

func makeParcoursUnitID(parcoursID string, etapeID string, critereID string) string {
	slug := fmt.Sprintf("RBOK-PARCOURS-%s-%s-%s", parcoursID, etapeID, critereID)
	return toUpperSlug(slug)
}

func toUpperSlug(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func mapCritereType(t string) string {
	switch strings.ToLower(t) {
	case "rule":
		return "rule"
	case "formula":
		return "formula"
	case "exception":
		return "exception"
	case "scenario":
		return "scenario"
	case "term":
		return "term"
	case "decision":
		return "decision"
	default:
		return "rule"
	}
}

func mapModuleType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "decision":
		return "decision"
	case "scenario":
		return "scenario"
	default:
		return "workflow"
	}
}

func mapQuestionType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "number", "integer", "decimal":
		return "formula"
	case "select", "boolean", "text", "textarea":
		return "rule"
	default:
		return "rule"
	}
}

func normalizeCriticality(c string) string {
	switch strings.ToLower(c) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(c)
	default:
		return "medium"
	}
}

// ----------------------------------------------------------------------------
// FSQ-04 (#367) YAML scalar indexer.
//
// indexYAMLScalars walks the YAML AST exposed by yaml.v3 and produces a
// map keyed by yaml_path (e.g. "parcours.modules[2].questions[7].help_text")
// whose values carry the raw byte slice (with quotes if quoted), the
// decoded value, the byte span, and a coarse node-kind label.
//
// Selection is strictly path-driven; the indexer does not match on value.
// ----------------------------------------------------------------------------

// yamlScalarInfo captures everything the parcours extractor needs to know
// about one YAML scalar value at a known key path.
type yamlScalarInfo struct {
	RawText      string
	DecodedValue string
	NodeKind     string
	StartByte    int
	EndByte      int
	StartLine    int
	StartColumn  int
}

// indexYAMLScalars parses data with yaml.v3 and returns a path → scalar map.
// Parse errors yield an empty map; callers fall back to absent provenance.
func indexYAMLScalars(data []byte) map[string]yamlScalarInfo {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return map[string]yamlScalarInfo{}
	}
	offsets := computeYAMLLineOffsets(data)
	out := map[string]yamlScalarInfo{}
	walkYAMLNode(&root, "", data, offsets, out)
	return out
}

// walkYAMLNode recursively descends a yaml.Node, populating out with one
// entry per scalar value reachable from the document root.
func walkYAMLNode(n *yaml.Node, path string, data []byte, offsets []int, out map[string]yamlScalarInfo) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkYAMLNode(c, path, data, offsets, out)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			keyNode := n.Content[i]
			valNode := n.Content[i+1]
			childPath := keyNode.Value
			if path != "" {
				childPath = path + "." + keyNode.Value
			}
			if valNode.Kind == yaml.ScalarNode {
				out[childPath] = scalarInfoFromNode(valNode, data, offsets)
			} else {
				walkYAMLNode(valNode, childPath, data, offsets, out)
			}
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if c.Kind == yaml.ScalarNode {
				out[childPath] = scalarInfoFromNode(c, data, offsets)
			} else {
				walkYAMLNode(c, childPath, data, offsets, out)
			}
		}
	case yaml.ScalarNode:
		// Top-level scalar (rare for parcours docs); record at current path.
		if path != "" {
			out[path] = scalarInfoFromNode(n, data, offsets)
		}
	case yaml.AliasNode:
		// Aliases share an anchor's content. We could resolve the alias
		// target, but parcours docs do not currently use anchors; record
		// nothing rather than synthesise a phantom span.
	}
}

// scalarInfoFromNode computes raw bytes, decoded value, byte span, and a
// coarse node-kind label for a single yaml.ScalarNode.
func scalarInfoFromNode(n *yaml.Node, data []byte, offsets []int) yamlScalarInfo {
	startByte, endByte := scalarByteSpan(n, data, offsets)
	rawText := ""
	if startByte >= 0 && endByte > startByte && endByte <= len(data) {
		rawText = string(data[startByte:endByte])
	}
	return yamlScalarInfo{
		RawText:      rawText,
		DecodedValue: n.Value,
		NodeKind:     yamlScalarNodeKind(n),
		StartByte:    startByte,
		EndByte:      endByte,
		StartLine:    n.Line,
		StartColumn:  n.Column,
	}
}

// yamlScalarNodeKind maps a yaml.v3 scalar tag to one of the FSQ-04 node
// kind labels: scalar_string, scalar_int, scalar_float, scalar_bool. Other
// tags fall back to scalar_string so callers always see a populated kind.
func yamlScalarNodeKind(n *yaml.Node) string {
	switch n.Tag {
	case "!!int":
		return "scalar_int"
	case "!!float":
		return "scalar_float"
	case "!!bool":
		return "scalar_bool"
	case "!!str", "":
		return "scalar_string"
	default:
		return "scalar_string"
	}
}

// scalarByteSpan converts a yaml.ScalarNode's (Line, Column) start position
// into byte offsets for [start, end) in data. The end is determined by the
// node's Style:
//
//   - DoubleQuotedStyle / SingleQuotedStyle — scan for the matching closing
//     quote, honouring '\' escapes (double-quoted) and ” escapes (single).
//   - LiteralStyle / FoldedStyle — block scalars; spans the lines indented
//     deeper than the scalar's indicator column.
//   - PlainStyle / FlowStyle / 0 — scan to the end of line, then trim
//     trailing whitespace. (Multi-line plain scalars are uncommon in
//     parcours docs and degrade to the first-line span; documented as a
//     judgment call.)
func scalarByteSpan(n *yaml.Node, data []byte, offsets []int) (int, int) {
	if n.Line <= 0 || n.Line > len(offsets) {
		return 0, 0
	}
	start := offsets[n.Line-1] + n.Column - 1
	if start < 0 || start > len(data) {
		return 0, 0
	}
	switch n.Style {
	case yaml.DoubleQuotedStyle:
		return start, scanDoubleQuotedEnd(data, start)
	case yaml.SingleQuotedStyle:
		return start, scanSingleQuotedEnd(data, start)
	case yaml.LiteralStyle, yaml.FoldedStyle:
		return start, scanBlockScalarEnd(data, start, n.Column)
	default:
		return start, scanPlainScalarEnd(data, start)
	}
}

func scanDoubleQuotedEnd(data []byte, start int) int {
	if start >= len(data) || data[start] != '"' {
		return scanPlainScalarEnd(data, start)
	}
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++ // skip escaped byte
		case '"':
			return i + 1
		}
	}
	return len(data)
}

func scanSingleQuotedEnd(data []byte, start int) int {
	if start >= len(data) || data[start] != '\'' {
		return scanPlainScalarEnd(data, start)
	}
	for i := start + 1; i < len(data); i++ {
		if data[i] == '\'' {
			if i+1 < len(data) && data[i+1] == '\'' {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(data)
}

func scanPlainScalarEnd(data []byte, start int) int {
	end := start
	for end < len(data) && data[end] != '\n' {
		end++
	}
	for end > start {
		c := data[end-1]
		if c == ' ' || c == '\t' || c == '\r' {
			end--
			continue
		}
		break
	}
	return end
}

// scanBlockScalarEnd consumes a literal- or folded-style block scalar by
// walking forward until a line is found that is less indented than the
// scalar's start column (or the input ends). It is a best-effort span: for
// the parcours flow we only need the bytes to round-trip the raw form, not
// to re-parse the YAML grammar.
func scanBlockScalarEnd(data []byte, start, startCol int) int {
	indicatorIndentCol := lineIndentColumnAt(data, start)
	if indicatorIndentCol <= 0 {
		indicatorIndentCol = startCol
	}
	// Skip the rest of the indicator line (starts with '|' or '>').
	i := start
	for i < len(data) && data[i] != '\n' {
		i++
	}
	if i < len(data) {
		i++ // consume the newline
	}
	blockIndent := 0
	probe := i
	for probe < len(data) {
		lineStart := probe
		col := 1
		for probe < len(data) && data[probe] == ' ' {
			probe++
			col++
		}
		if probe >= len(data) {
			break
		}
		if data[probe] == '\r' || data[probe] == '\n' {
			probe = scanToNextLine(data, probe)
			continue
		}
		if col <= indicatorIndentCol {
			return lineStart
		}
		blockIndent = col
		break
	}
	if blockIndent == 0 {
		return i
	}
	for i < len(data) {
		// Measure indentation of the current line.
		lineStart := i
		col := 1
		for i < len(data) && data[i] == ' ' {
			i++
			col++
		}
		if i >= len(data) {
			return i
		}
		if data[i] == '\n' {
			i++
			continue
		}
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			i += 2
			continue
		}
		if col < blockIndent {
			return lineStart
		}
		for i < len(data) && data[i] != '\n' {
			i++
		}
		if i < len(data) {
			i++
		}
	}
	return i
}

func scanToNextLine(data []byte, i int) int {
	for i < len(data) && data[i] != '\n' {
		i++
	}
	if i < len(data) {
		i++
	}
	return i
}

func lineIndentColumnAt(data []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	lineStart := offset
	for lineStart > 0 && data[lineStart-1] != '\n' {
		lineStart--
	}
	col := 1
	for lineStart < len(data) && data[lineStart] == ' ' {
		lineStart++
		col++
	}
	return col
}

// computeYAMLLineOffsets returns a slice where index i is the byte offset
// of the start of line (i+1) in data. Line numbers in yaml.v3 are 1-based.
func computeYAMLLineOffsets(data []byte) []int {
	offsets := []int{0}
	for i, b := range data {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}
