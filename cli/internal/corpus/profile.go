package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Known profile names.
const ProfileRBOKLawbook = "rbok-lawbook"

// OutputFlag controls which feed sections are emitted.
type OutputFlag string

const (
	OutputIndex              OutputFlag = "index"
	OutputGovernance         OutputFlag = "governance"
	OutputCitation           OutputFlag = "citation"
	OutputImport             OutputFlag = "import"
	OutputFeed               OutputFlag = "feed"
	OutputRAGMetadata        OutputFlag = "rag_metadata"
	OutputAtomizationReport  OutputFlag = "atomization_report"
	OutputTraceabilityMatrix OutputFlag = "traceability_matrix"
)

var allOutputFlags = []OutputFlag{
	OutputIndex,
	OutputGovernance,
	OutputCitation,
	OutputImport,
	OutputFeed,
	OutputRAGMetadata,
	OutputAtomizationReport,
	OutputTraceabilityMatrix,
}

// Profile describes a corpus processing profile.
type Profile struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Outputs     []OutputFlag `json:"outputs"`
}

var profileRegistry = map[string]Profile{
	ProfileRBOKLawbook: {
		Name:        ProfileRBOKLawbook,
		Description: "RBOK Lawbook corpus profile: classifies sources and generates governance-aware lawbook feed, RAG metadata, traceability matrix, and import outputs.",
		Outputs:     allOutputFlags,
	},
}

// LookupProfile returns the profile for a given name, or an error if unknown.
func LookupProfile(name string) (Profile, error) {
	p, ok := profileRegistry[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		known := KnownProfiles()
		return Profile{}, fmt.Errorf("unknown profile %q; known profiles: %s", name, strings.Join(known, ", "))
	}
	return p, nil
}

// KnownProfiles returns sorted profile names.
func KnownProfiles() []string {
	names := make([]string, 0, len(profileRegistry))
	for k := range profileRegistry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ProfileFeedInput configures a profiled feed run.
type ProfileFeedInput struct {
	Profile      string       `json:"profile"`
	CorpusRoot   string       `json:"corpus_root"`
	MatrixPath   string       `json:"matrix_path"`
	ManifestPath string       `json:"manifest_path"`
	Outputs      []OutputFlag `json:"outputs"`
}

// ProfileFeedResult holds the profiled feed output sections.
type ProfileFeedResult struct {
	Profile     string                         `json:"profile"`
	Sections    map[OutputFlag]json.RawMessage `json:"sections"`
	SourceCount int                            `json:"source_count"`
	UnitCount   int                            `json:"unit_count"`
	Errors      []string                       `json:"errors,omitempty"`
	Warnings    []string                       `json:"warnings,omitempty"`
}

// ProfileAtomizationReport summarizes extraction completeness and
// traceability posture for a profiled feed run.
type ProfileAtomizationReport struct {
	Format                string         `json:"format"`
	Profile               string         `json:"profile"`
	GeneratedAt           string         `json:"generated_at"`
	SourceCount           int            `json:"source_count"`
	AtomizedSourceCount   int            `json:"atomized_source_count"`
	SkippedSourceCount    int            `json:"skipped_source_count"`
	DocumentCount         int            `json:"document_count"`
	TotalNodes            int            `json:"total_nodes"`
	NodeTypes             map[string]int `json:"node_types"`
	SourceClasses         map[string]int `json:"source_classes"`
	CorpusLayers          map[string]int `json:"corpus_layers"`
	Authorities           map[string]int `json:"authorities"`
	MissingSourceHash     int            `json:"missing_source_hash"`
	MissingLocator        int            `json:"missing_locator"`
	DeterministicOrdering bool           `json:"deterministic_ordering"`
}

// TraceabilityEntry is one source-to-node row in the profile traceability matrix.
type TraceabilityEntry struct {
	NodeID       string `json:"node_id"`
	DocumentID   string `json:"document_id"`
	NodeType     string `json:"node_type"`
	CanonicalRef string `json:"canonical_ref"`
	DisplayRef   string `json:"display_ref"`
	ParentID     string `json:"parent_id,omitempty"`
	SourcePath   string `json:"source_path"`
	SourceHash   string `json:"source_hash"`
	SourceClass  string `json:"source_class"`
	CorpusLayer  string `json:"corpus_layer"`
	Authority    string `json:"authority"`
	Locator      string `json:"locator"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
	Domain       string `json:"domain"`
}

type profileArtifacts struct {
	Assembly     MultiFeedAssembly
	Report       ProfileAtomizationReport
	Traceability []TraceabilityEntry
}

// RunProfileFeed executes a profiled corpus feed generation.
func RunProfileFeed(input ProfileFeedInput) (ProfileFeedResult, error) {
	profile, err := LookupProfile(input.Profile)
	if err != nil {
		return ProfileFeedResult{}, err
	}

	outputs := input.Outputs
	if len(outputs) == 0 {
		outputs = profile.Outputs
	}

	result := ProfileFeedResult{
		Profile:  profile.Name,
		Sections: make(map[OutputFlag]json.RawMessage),
	}

	// Classify all sources using the profile policy.
	classifications, classifyMessages := classifyCorpusSources(input.CorpusRoot)
	result.SourceCount = len(classifications)
	for _, message := range classifyMessages {
		if isBlockingProfileMessage(message) {
			result.Errors = append(result.Errors, message)
		} else {
			result.Warnings = append(result.Warnings, message)
		}
	}

	artifacts, artifactMessages := buildProfileArtifacts(profile.Name, input.CorpusRoot, classifications)
	result.UnitCount = artifacts.Assembly.TotalNodes
	for _, message := range artifactMessages {
		if isBlockingProfileMessage(message) {
			result.Errors = append(result.Errors, message)
		} else {
			result.Warnings = append(result.Warnings, message)
		}
	}

	// Build sections.
	for _, out := range outputs {
		section, err := buildSection(out, classifications, artifacts)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("section %s: %v", out, err))
			continue
		}
		result.Sections[out] = section
	}

	return result, nil
}

func isBlockingProfileMessage(message string) bool {
	return strings.HasPrefix(message, "blocked binary:") || strings.HasPrefix(message, "scan corpus:")
}

func classifyCorpusSources(corpusRoot string) ([]RBOKSourceClassification, []string) {
	if corpusRoot == "" {
		return nil, []string{"corpus_root is empty"}
	}

	var classifications []RBOKSourceClassification
	var errors []string

	err := filepath.WalkDir(corpusRoot, func(filePath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(corpusRoot, filePath)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		c := ClassifyRBOKSource(rel)
		classifications = append(classifications, c)

		if isBinaryFile(filePath) {
			if shouldBlockBinary(rel, c) {
				errors = append(errors, fmt.Sprintf("blocked binary: %s", rel))
			} else if shouldWarnNonAtomizedBinary(c) {
				errors = append(errors, fmt.Sprintf("non-atomized binary admitted as %s: %s", c.Role, rel))
			}
		}

		return nil
	})
	if err != nil {
		return nil, []string{fmt.Sprintf("scan corpus: %v", err)}
	}

	return classifications, errors
}

func shouldWarnNonAtomizedBinary(c RBOKSourceClassification) bool {
	switch c.Role {
	case RoleOutOfScope, RoleArchive, RoleDerived, RoleSchema:
		return false
	default:
		return true
	}
}

func shouldBlockBinary(rel string, c RBOKSourceClassification) bool {
	switch c.Role {
	case RoleOutOfScope, RoleArchive, RoleReference, RoleDerived, RoleSupporting, RoleEvidence, RoleOperational, RoleSchema:
		return false
	}
	if isKnownDocumentBinary(rel) {
		return false
	}
	return true
}

func isKnownDocumentBinary(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico":
		return true
	default:
		return false
	}
}

func isBinaryFile(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// IndexEntry is a single entry in the index section.
type IndexEntry struct {
	Path     string     `json:"path"`
	Priority string     `json:"priority"`
	Role     SourceRole `json:"role"`
}

// GovernanceEntry summarises governance posture per source.
type GovernanceEntry struct {
	Path        string   `json:"path"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	AllowedUses []string `json:"allowed_uses"`
	Blocked     bool     `json:"blocked"`
}

func buildSection(flag OutputFlag, classifications []RBOKSourceClassification, artifacts profileArtifacts) (json.RawMessage, error) {
	switch flag {
	case OutputIndex:
		entries := make([]IndexEntry, 0, len(classifications))
		for _, c := range classifications {
			if c.Role == RoleOutOfScope {
				continue
			}
			entries = append(entries, IndexEntry{
				Path:     c.Path,
				Priority: c.Priority,
				Role:     c.Role,
			})
		}
		return json.Marshal(entries)

	case OutputGovernance:
		entries := make([]GovernanceEntry, 0, len(classifications))
		for _, c := range classifications {
			entries = append(entries, GovernanceEntry{
				Path:        c.Path,
				Priority:    c.Priority,
				Status:      c.Status,
				AllowedUses: c.AllowedUses,
				Blocked:     c.Role == RoleOutOfScope,
			})
		}
		return json.Marshal(entries)

	case OutputCitation:
		var citeable []IndexEntry
		for _, c := range classifications {
			for _, use := range c.AllowedUses {
				if use == "citation_internal" || use == "citation_external" {
					citeable = append(citeable, IndexEntry{
						Path:     c.Path,
						Priority: c.Priority,
						Role:     c.Role,
					})
					break
				}
			}
		}
		return json.Marshal(citeable)

	case OutputImport:
		var importable []IndexEntry
		for _, c := range classifications {
			for _, use := range c.AllowedUses {
				if use == "structured_contract" || use == "vector_index" {
					importable = append(importable, IndexEntry{
						Path:     c.Path,
						Priority: c.Priority,
						Role:     c.Role,
					})
					break
				}
			}
		}
		return json.Marshal(importable)

	case OutputFeed:
		return json.Marshal(artifacts.Assembly)

	case OutputRAGMetadata:
		return json.Marshal(artifacts.Assembly.RAGMetadata)

	case OutputAtomizationReport:
		return json.Marshal(artifacts.Report)

	case OutputTraceabilityMatrix:
		return json.Marshal(artifacts.Traceability)

	default:
		return nil, fmt.Errorf("unknown output flag %q", flag)
	}
}

func buildProfileArtifacts(profileName string, corpusRoot string, classifications []RBOKSourceClassification) (profileArtifacts, []string) {
	now := time.Now().UTC()
	generatedAt := now.Format(time.RFC3339)
	var feeds []LawbookFeed
	var messages []string

	ordered := append([]RBOKSourceClassification(nil), classifications...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Path < ordered[j].Path
	})

	for _, c := range ordered {
		if !shouldAtomizeProfileSource(c) {
			continue
		}

		ext := strings.ToLower(filepath.Ext(c.Path))
		switch {
		case ext == ".md" || ext == ".mdx":
			feed, err := buildMarkdownProfileFeed(corpusRoot, c, generatedAt)
			if err != nil {
				messages = append(messages, fmt.Sprintf("atomize markdown %s: %v", c.Path, err))
				continue
			}
			if feed.NodeCount == 0 {
				messages = append(messages, fmt.Sprintf("non-atomized text source: %s", c.Path))
				continue
			}
			feeds = append(feeds, feed)

		case (ext == ".yaml" || ext == ".yml") && c.SourceClass == "runtime_binding":
			feed, err := buildParcoursProfileFeed(corpusRoot, c, generatedAt)
			if err != nil {
				messages = append(messages, fmt.Sprintf("atomize parcours %s: %v", c.Path, err))
				continue
			}
			if feed.NodeCount == 0 {
				messages = append(messages, fmt.Sprintf("non-atomized parcours source: %s", c.Path))
				continue
			}
			feeds = append(feeds, feed)

		case ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".cue":
			messages = append(messages, fmt.Sprintf("structured source not atomized by rbok-lawbook profile yet: %s", c.Path))
		}
	}

	assembly := AssembleMultiFeed(feeds, MultiAssembleOptions{Now: now})
	traceability := buildTraceabilityMatrix(assembly)
	report := buildAtomizationReport(profileName, generatedAt, len(classifications), len(feeds), assembly, traceability)

	return profileArtifacts{
		Assembly:     assembly,
		Report:       report,
		Traceability: traceability,
	}, messages
}

func shouldAtomizeProfileSource(c RBOKSourceClassification) bool {
	switch c.Role {
	case RoleLawbook, RoleSupporting, RoleEvidence, RoleOperational:
		return true
	default:
		return false
	}
}

func buildMarkdownProfileFeed(corpusRoot string, c RBOKSourceClassification, generatedAt string) (LawbookFeed, error) {
	absPath := filepath.Join(corpusRoot, filepath.FromSlash(c.Path))
	data, err := os.ReadFile(absPath)
	if err != nil {
		return LawbookFeed{}, err
	}
	hash, _, err := hashFile(absPath)
	if err != nil {
		return LawbookFeed{}, err
	}

	sourceHash := "sha256:" + hash
	docID := documentIDForPath(c.Path)
	result := ExtractMarkdown(string(data), documentSlugForPath(c.Path))
	defaults := NodeDefaults{
		DocumentID: docID,
		SourcePath: c.Path,
		SourceHash: sourceHash,
		Domain:     domainForClassification(c),
		Status:     statusForClassification(c),
		Priority:   priorityForClassification(c),
	}

	if errorCount := NormalizeExtractResult(&result, defaults); errorCount > 0 {
		return LawbookFeed{}, fmt.Errorf("%d node normalization error(s)", errorCount)
	}
	for i := range result.Nodes {
		applySourceClassification(&result.Nodes[i], c)
		result.Nodes[i].Locator = locatorForNode(c.Path, result.Nodes[i])
		if errs := ValidateNode(result.Nodes[i]); len(errs) > 0 {
			return LawbookFeed{}, fmt.Errorf("validate node %s: %s", result.Nodes[i].NodeID, strings.Join(errs, "; "))
		}
	}

	feed := BuildNormalizedFeed(result, feedIDForPath(c.Path), defaults, generatedAt)
	if errs := ValidateFeed(feed); len(errs) > 0 {
		return LawbookFeed{}, fmt.Errorf("validate feed: %s", strings.Join(errs, "; "))
	}
	return feed, nil
}

func buildParcoursProfileFeed(corpusRoot string, c RBOKSourceClassification, generatedAt string) (LawbookFeed, error) {
	absPath := filepath.Join(corpusRoot, filepath.FromSlash(c.Path))
	extracted, err := ExtractParcours(absPath)
	if err != nil {
		return LawbookFeed{}, err
	}
	hash, _, err := hashFile(absPath)
	if err != nil {
		return LawbookFeed{}, err
	}

	sourceHash := "sha256:" + hash
	parcoursSlug := documentSlugForPath(firstNonEmpty(extracted.ParcoursID, c.Path))
	docID := toUpperSlug("RBOK-PARCOURS-" + firstNonEmpty(extracted.ParcoursID, c.Path))
	if docID == "" {
		docID = documentIDForPath(c.Path)
	}

	nodes := []LawbookNode{
		{
			NodeID:       docID,
			DocumentID:   docID,
			NodeType:     NodeDocument,
			CanonicalRef: "rbok/parcours/" + parcoursSlug,
			DisplayRef:   "parcours: " + firstNonEmpty(extracted.ParcoursName, extracted.ParcoursID, c.Path),
			Depth:        NodeDocument.Depth(),
			OrdinalPath:  "1",
			SourcePath:   c.Path,
			SourceHash:   sourceHash,
			Status:       statusForClassification(c),
			Priority:     priorityForClassification(c),
			Domain:       domainForClassification(c),
			Title:        firstNonEmpty(extracted.ParcoursName, extracted.ParcoursID),
			Text:         firstNonEmpty(extracted.ParcoursName, extracted.ParcoursID),
		},
	}
	applySourceClassification(&nodes[0], c)
	nodes[0].Locator = locatorForNode(c.Path, nodes[0])

	for i, unit := range extracted.Units {
		ordinal := fmt.Sprintf("1.%d", i+1)
		node := LawbookNode{
			NodeID:       uniqueNodeID(unit.UnitID, i),
			DocumentID:   docID,
			NodeType:     NodeArticle,
			CanonicalRef: fmt.Sprintf("rbok/parcours/%s/unit/%s", parcoursSlug, lawbookSlugify(unit.UnitID)),
			DisplayRef:   "runtime unit: " + firstNonEmpty(unit.Name, unit.UnitID),
			Depth:        NodeArticle.Depth(),
			OrdinalPath:  ordinal,
			SourcePath:   c.Path,
			SourceHash:   sourceHash,
			Status:       statusFromParcoursUnit(unit),
			Priority:     priorityFromCriticality(unit.Criticality),
			Domain:       firstNonEmpty(unit.Domain, domainForClassification(c)),
			Title:        firstNonEmpty(unit.Name, unit.UnitID),
			Text:         firstNonEmpty(unit.BusinessRule, unit.Name),
			ParentID:     docID,
			Metadata: map[string]any{
				"unit_id":        unit.UnitID,
				"unit_type":      unit.UnitType,
				"criticality":    unit.Criticality,
				"parcours_id":    unit.ParcoursID,
				"etape_id":       unit.EtapeID,
				"etape_name":     unit.EtapeName,
				"objectif_id":    unit.ObjectifID,
				"owner":          unit.Owner,
				"runtime_status": unit.Status,
			},
		}
		applySourceClassification(&node, c)
		node.Locator = locatorForNode(c.Path, node)
		if errs := ValidateNode(node); len(errs) > 0 {
			return LawbookFeed{}, fmt.Errorf("validate parcours node %s: %s", node.NodeID, strings.Join(errs, "; "))
		}
		nodes = append(nodes, node)
	}

	feed := LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        feedIDForPath(c.Path),
		DocumentID:    docID,
		Domain:        domainForClassification(c),
		GeneratedAt:   generatedAt,
		SourcePath:    c.Path,
		SourceHash:    sourceHash,
		NodeCount:     len(nodes),
		Nodes:         nodes,
	}
	if errs := ValidateFeed(feed); len(errs) > 0 {
		return LawbookFeed{}, fmt.Errorf("validate feed: %s", strings.Join(errs, "; "))
	}
	return feed, nil
}

func applySourceClassification(node *LawbookNode, c RBOKSourceClassification) {
	node.SourceClass = c.SourceClass
	node.CorpusLayer = c.CorpusLayer
	node.Authority = c.Authority
	node.AllowedUses = append([]string(nil), c.AllowedUses...)
	if node.Metadata == nil {
		node.Metadata = map[string]any{}
	}
	node.Metadata["source_role"] = string(c.Role)
	node.Metadata["source_priority"] = c.Priority
	node.Metadata["source_status"] = c.Status
	node.Metadata["source_class"] = c.SourceClass
	node.Metadata["corpus_layer"] = c.CorpusLayer
	node.Metadata["authority"] = c.Authority
	node.Metadata["allowed_uses"] = append([]string(nil), c.AllowedUses...)
}

func buildTraceabilityMatrix(assembly MultiFeedAssembly) []TraceabilityEntry {
	var rows []TraceabilityEntry
	for _, feed := range assembly.Feeds {
		for _, node := range feed.Nodes {
			rows = append(rows, TraceabilityEntry{
				NodeID:       node.NodeID,
				DocumentID:   node.DocumentID,
				NodeType:     string(node.NodeType),
				CanonicalRef: node.CanonicalRef,
				DisplayRef:   node.DisplayRef,
				ParentID:     node.ParentID,
				SourcePath:   node.SourcePath,
				SourceHash:   node.SourceHash,
				SourceClass:  node.SourceClass,
				CorpusLayer:  node.CorpusLayer,
				Authority:    node.Authority,
				Locator:      node.Locator,
				Status:       string(node.Status),
				Priority:     string(node.Priority),
				Domain:       node.Domain,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SourcePath != rows[j].SourcePath {
			return rows[i].SourcePath < rows[j].SourcePath
		}
		if rows[i].DocumentID != rows[j].DocumentID {
			return rows[i].DocumentID < rows[j].DocumentID
		}
		if rows[i].CanonicalRef != rows[j].CanonicalRef {
			return rows[i].CanonicalRef < rows[j].CanonicalRef
		}
		return rows[i].NodeID < rows[j].NodeID
	})
	return rows
}

func buildAtomizationReport(profileName, generatedAt string, sourceCount int, atomizedSourceCount int, assembly MultiFeedAssembly, traceability []TraceabilityEntry) ProfileAtomizationReport {
	report := ProfileAtomizationReport{
		Format:                "nomos.profile-atomization-report.v1",
		Profile:               profileName,
		GeneratedAt:           generatedAt,
		SourceCount:           sourceCount,
		AtomizedSourceCount:   atomizedSourceCount,
		SkippedSourceCount:    sourceCount - atomizedSourceCount,
		DocumentCount:         assembly.DocumentCount,
		TotalNodes:            assembly.TotalNodes,
		NodeTypes:             map[string]int{},
		SourceClasses:         map[string]int{},
		CorpusLayers:          map[string]int{},
		Authorities:           map[string]int{},
		DeterministicOrdering: true,
	}
	for _, row := range traceability {
		report.NodeTypes[row.NodeType]++
		report.SourceClasses[row.SourceClass]++
		report.CorpusLayers[row.CorpusLayer]++
		report.Authorities[row.Authority]++
		if strings.TrimSpace(row.SourceHash) == "" {
			report.MissingSourceHash++
		}
		if strings.TrimSpace(row.Locator) == "" {
			report.MissingLocator++
		}
	}
	return report
}

func documentIDForPath(rel string) string {
	id := toUpperSlug("RBOK-DOC-" + strings.TrimSuffix(rel, filepath.Ext(rel)))
	if id == "" {
		return "RBOK-DOC"
	}
	return id
}

func documentSlugForPath(rel string) string {
	slug := lawbookSlugify(strings.TrimSuffix(strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"), filepath.Ext(rel)))
	if slug == "" {
		return "rbok"
	}
	return slug
}

func feedIDForPath(rel string) string {
	feedID := lawbookSlugify(strings.TrimSuffix(strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"), filepath.Ext(rel))) + "-feed"
	feedID = strings.Trim(feedID, "-")
	if feedID == "" {
		return "rbok-feed"
	}
	return feedID
}

func locatorForNode(rel string, node LawbookNode) string {
	if node.OrdinalPath != "" {
		return rel + "#" + node.OrdinalPath
	}
	return rel + "#" + node.NodeID
}

func uniqueNodeID(base string, index int) string {
	id := toUpperSlug(base)
	if id == "" {
		return fmt.Sprintf("RBOK-NODE-%d", index+1)
	}
	return id
}

func domainForClassification(c RBOKSourceClassification) string {
	if c.CorpusLayer != "" && c.CorpusLayer != "canonical_core" {
		return c.CorpusLayer
	}
	return "rbok"
}

func statusForClassification(c RBOKSourceClassification) LawbookNodeStatus {
	switch strings.ToLower(strings.TrimSpace(c.Status)) {
	case "active":
		return StatusActive
	case "draft":
		return StatusNodeDraft
	case "pending":
		return StatusPending
	default:
		return StatusPending
	}
}

func priorityForClassification(c RBOKSourceClassification) LawbookPriority {
	switch strings.ToLower(strings.TrimSpace(c.Priority)) {
	case "primary":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	case "low", "archive", "reference":
		return PriorityLow
	default:
		return PriorityMedium
	}
}

func statusFromParcoursUnit(unit ParcoursUnit) LawbookNodeStatus {
	switch strings.ToLower(strings.TrimSpace(unit.Status)) {
	case "active":
		return StatusActive
	case "draft":
		return StatusNodeDraft
	case "pending":
		return StatusPending
	default:
		return StatusPending
	}
}

func priorityFromCriticality(criticality string) LawbookPriority {
	switch strings.ToLower(strings.TrimSpace(criticality)) {
	case "critical":
		return PriorityCritical
	case "high":
		return PriorityHigh
	case "low":
		return PriorityLow
	default:
		return PriorityMedium
	}
}

// WriteProfileFeedJSON serialises the result to a writer.
func WriteProfileFeedJSON(w io.Writer, result ProfileFeedResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
