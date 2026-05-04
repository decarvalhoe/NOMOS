package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const FeedFormat = "nomos.corpus-feed.v1"

// FeedUnit is a unit entry in the consumer feed.
//
// SFI-05 (#343): source-derived feed units (built from a canonical_atom
// SourceSegment) carry exact byte/line spans, the segment id, and the
// normalized-text hash so downstream consumers can re-prove provenance.
// Matrix-derived units leave these fields empty (omitempty).
type FeedUnit struct {
	UnitID       string           `json:"unit_id"`
	Name         string           `json:"name"`
	Domain       string           `json:"domain"`
	UnitType     string           `json:"unit_type"`
	Criticality  string           `json:"criticality"`
	Status       string           `json:"status"`
	BusinessRule string           `json:"business_rule"`
	SourceIDs    []string         `json:"source_ids"`
	TestRefs     []string         `json:"test_refs,omitempty"`
	Gaps         []string         `json:"gaps,omitempty"`
	Contract     *FeedContractRef `json:"contract,omitempty"`

	// SFI-05 (#343) canonical-atom segment linkage. Optional; only
	// populated for source-derived units.
	SourceSegmentID    string   `json:"source_segment_id,omitempty"`
	SourceID           string   `json:"source_id,omitempty"`
	SourcePath         string   `json:"source_path,omitempty"`
	StartByte          int      `json:"start_byte,omitempty"`
	EndByte            int      `json:"end_byte,omitempty"`
	StartLine          int      `json:"start_line,omitempty"`
	EndLine            int      `json:"end_line,omitempty"`
	NormalizedTextHash string   `json:"normalized_text_hash,omitempty"`
	HeadingPath        []string `json:"heading_path,omitempty"`

	// FSQ-04 (#367) YAML scalar provenance. Optional; populated for parcours
	// (YAML)-derived units. RawText is the byte slice of the YAML source as
	// it appears (with quotes if quoted); DecodedValue is the decoded scalar
	// value. YAMLPath is the key path (e.g. "parcours.modules[2].questions[7].help_text").
	// BusinessRuleMode states how the BusinessRule field is to be interpreted
	// ("raw" | "decoded" | "normalized"); for the parcours flow it is "decoded".
	RawText          string `json:"raw_text,omitempty"`
	DecodedValue     string `json:"decoded_value,omitempty"`
	YAMLPath         string `json:"yaml_path,omitempty"`
	StructuredPath   string `json:"structured_path,omitempty"`
	StructuredFormat string `json:"structured_format,omitempty"`
	NodeKind         string `json:"node_kind,omitempty"`
	SchemaRole       string `json:"schema_role,omitempty"`
	BusinessRuleMode string `json:"business_rule_mode,omitempty"`

	// FSQ-03 (#366) table-context linkage. Set on units derived from
	// table_row data segments so RAG composers can reconstruct the row's
	// column context without re-parsing the source.
	TableID       string   `json:"table_id,omitempty"`
	TableRole     string   `json:"table_role,omitempty"`
	RowIndex      int      `json:"row_index,omitempty"`
	ColumnHeaders []string `json:"column_headers,omitempty"`

	// FSQ-05 (#368) body-ledger linkage. Lists the SourceSegment IDs that
	// compose this feed unit. For single-segment source-derived units
	// this is [SourceSegmentID]; for composed table_row units it is the
	// row segment's id followed by its child table_cell segment ids.
	// Matrix-derived units leave it empty (omitempty).
	BodyLedgerSegmentIDs []string `json:"body_ledger_segment_ids,omitempty"`
}

// FeedContractRef is a simplified contract reference for consumers.
type FeedContractRef struct {
	Path     string `json:"path"`
	ObjectID string `json:"object_id"`
	Status   string `json:"status"`
}

// FeedSource is a source snapshot entry in the consumer feed.
//
// FSQ-02 (#365): the AdmissionStatus / AtomizationStatus / ExclusionReason
// / SourceRole / FormatSupport / DerivativeOf fields make the admission
// and atomization policy explicit. All six are omitempty so legacy
// consumers keep parsing the JSON; feed generation backfills defaults
// from the extension heuristic and validates the transition rules
// before returning the feed.
type FeedSource struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	Domain          string `json:"domain"`
	Type            string `json:"type"`
	Owner           string `json:"owner"`
	Confidentiality string `json:"confidentiality"`
	Hash            string `json:"hash"`
	Status          string `json:"status"`

	// FSQ-02 (#365) admission + atomization policy.
	AdmissionStatus   string `json:"admission_status,omitempty"`
	AtomizationStatus string `json:"atomization_status,omitempty"`
	ExclusionReason   string `json:"exclusion_reason,omitempty"`
	SourceRole        string `json:"source_role,omitempty"`
	FormatSupport     string `json:"format_support,omitempty"`
	DerivativeOf      string `json:"derivative_of,omitempty"`
}

// Admission returns a SourceAdmission projection of the feed source.
func (s FeedSource) Admission() SourceAdmission {
	return SourceAdmission{
		AdmissionStatus:   s.AdmissionStatus,
		AtomizationStatus: s.AtomizationStatus,
		ExclusionReason:   s.ExclusionReason,
		SourceRole:        s.SourceRole,
		FormatSupport:     s.FormatSupport,
		DerivativeOf:      s.DerivativeOf,
	}
}

// Validate enforces the FSQ-02 admission rules on the feed source.
func (s FeedSource) Validate() error {
	return s.Admission().Validate()
}

// FeedSnapshotSummary links a feed to the immutable corpus snapshot it was built from.
type FeedSnapshotSummary struct {
	Format     string `json:"format"`
	CorpusRoot string `json:"corpus_root,omitempty"`
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	TotalFiles int    `json:"total_files"`
	TotalBytes int64  `json:"total_bytes"`
}

// CorpusIndex is a compact source-to-unit/chunk index for consumers.
type CorpusIndex struct {
	Format      string              `json:"format"`
	GeneratedAt string              `json:"generated_at"`
	SourceCount int                 `json:"source_count"`
	UnitCount   int                 `json:"unit_count"`
	ChunkCount  int                 `json:"chunk_count"`
	Sources     []CorpusIndexSource `json:"sources"`
}

// CorpusIndexSource records which canonical units and RAG chunks came from a source.
type CorpusIndexSource struct {
	SourceID string   `json:"source_id"`
	Path     string   `json:"path"`
	Hash     string   `json:"hash"`
	Domain   string   `json:"domain"`
	Type     string   `json:"type"`
	UnitIDs  []string `json:"unit_ids,omitempty"`
	ChunkIDs []string `json:"chunk_ids,omitempty"`
}

// FeedLockfileStatus reports whether the snapshot was accepted by a lockfile.
type FeedLockfileStatus struct {
	Accepted        bool     `json:"accepted"`
	ApprovedCount   int      `json:"approved_count"`
	UnapprovedCount int      `json:"unapproved_count"`
	UnapprovedPaths []string `json:"unapproved_paths,omitempty"`
}

// Feed is the top-level consumer feed artifact.
type Feed struct {
	Format      string                      `json:"format"`
	GeneratedAt string                      `json:"generated_at"`
	ContentHash string                      `json:"content_hash"`
	UnitCount   int                         `json:"unit_count"`
	SourceCount int                         `json:"source_count"`
	Units       []FeedUnit                  `json:"units"`
	Sources     []FeedSource                `json:"sources"`
	Snapshot    *FeedSnapshotSummary        `json:"snapshot,omitempty"`
	CorpusIndex *CorpusIndex                `json:"corpus_index,omitempty"`
	RAGMetadata []ChunkMetadata             `json:"rag_metadata,omitempty"`
	Attestation *CorpusAttestationStatement `json:"attestation,omitempty"`
	Lockfile    *FeedLockfileStatus         `json:"lockfile,omitempty"`
}

// FeedInput provides the raw data for feed generation.
type FeedInput struct {
	MatrixYAML            []byte
	ManifestYAML          []byte
	Root                  string
	Snapshot              *Snapshot
	Lockfile              *Lockfile
	CorpusID              string
	ProjectID             string
	ScannerVersion        string
	AttestationScope      string
	AttestationVerdict    string
	AttestationConfidence string
	AttestationDiagnosis  *DiagnoseVerdict
	Policy                *Policy
	GeneratedAt           time.Time
}

// matrixFile mirrors the canonical-matrix YAML for parsing.
type matrixFile struct {
	SchemaVersion string       `yaml:"schema_version"`
	Units         []matrixUnit `yaml:"units"`
}

type matrixUnit struct {
	UnitID            string            `yaml:"unit_id"`
	UnitType          string            `yaml:"unit_type"`
	Name              string            `yaml:"name"`
	Domain            string            `yaml:"domain"`
	Criticality       string            `yaml:"criticality"`
	Status            string            `yaml:"status"`
	BusinessRule      string            `yaml:"business_rule"`
	SourceRefs        []matrixSourceRef `yaml:"source_refs"`
	TestRefs          []string          `yaml:"test_refs,omitempty"`
	Gaps              []string          `yaml:"gaps,omitempty"`
	CanonicalContract *matrixContract   `yaml:"canonical_contract,omitempty"`
}

type matrixSourceRef struct {
	SourceID string `yaml:"source_id"`
}

type matrixContract struct {
	Path     string `yaml:"path"`
	ObjectID string `yaml:"object_id"`
	Status   string `yaml:"status"`
}

type extractedFeedUnit struct {
	FeedUnit
	Content          string
	SourceID         string
	SourcePath       string
	SourceHash       string
	Priority         string
	SourceStatus     string
	Locator          string
	SourceSegment    SourceSegment
	HasSourceSegment bool
}

// GenerateFeed produces a consumer feed from a canonical matrix and source manifest.
func GenerateFeed(input FeedInput) (Feed, error) {
	var matrix matrixFile
	if len(input.MatrixYAML) > 0 {
		if err := yaml.Unmarshal(input.MatrixYAML, &matrix); err != nil {
			return Feed{}, fmt.Errorf("parse matrix: %w", err)
		}
	}

	var manifest SidecarManifest
	if err := yaml.Unmarshal(input.ManifestYAML, &manifest); err != nil {
		return Feed{}, fmt.Errorf("parse manifest: %w", err)
	}

	// FSQ-02 (#365): backfill admission/atomization defaults so legacy
	// manifest YAML (which omits the six classification fields) keeps
	// loading. Operator-declared values always win.
	for i := range manifest.Sources {
		adm := manifest.Sources[i].Admission()
		BackfillAdmission(&adm, manifest.Sources[i].Path)
		manifest.Sources[i].AdmissionStatus = adm.AdmissionStatus
		manifest.Sources[i].AtomizationStatus = adm.AtomizationStatus
		manifest.Sources[i].ExclusionReason = adm.ExclusionReason
		manifest.Sources[i].SourceRole = adm.SourceRole
		manifest.Sources[i].FormatSupport = adm.FormatSupport
		manifest.Sources[i].DerivativeOf = adm.DerivativeOf
		if err := manifest.Sources[i].Validate(); err != nil {
			return Feed{}, fmt.Errorf("manifest source %q: %w", manifest.Sources[i].ID, err)
		}
	}

	ts := input.GeneratedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	units := make([]FeedUnit, 0, len(matrix.Units))
	for _, u := range matrix.Units {
		fu := FeedUnit{
			UnitID:       u.UnitID,
			Name:         u.Name,
			Domain:       u.Domain,
			UnitType:     u.UnitType,
			Criticality:  u.Criticality,
			Status:       u.Status,
			BusinessRule: u.BusinessRule,
			TestRefs:     u.TestRefs,
			Gaps:         u.Gaps,
		}
		for _, ref := range u.SourceRefs {
			fu.SourceIDs = append(fu.SourceIDs, ref.SourceID)
		}
		if u.CanonicalContract != nil {
			fu.Contract = &FeedContractRef{
				Path:     u.CanonicalContract.Path,
				ObjectID: u.CanonicalContract.ObjectID,
				Status:   u.CanonicalContract.Status,
			}
		}
		units = append(units, fu)
	}

	extracted, err := extractFeedUnits(input.Root, manifest)
	if err != nil {
		return Feed{}, err
	}
	for _, item := range extracted {
		units = append(units, item.FeedUnit)
	}

	// FSQ-02 (#365): count units per source so we can enforce
	// "atomization_status=atomized implies ≥1 feed unit" fail-closed.
	unitsBySource := map[string]int{}
	for _, u := range units {
		for _, sid := range u.SourceIDs {
			unitsBySource[sid]++
		}
	}

	sources := make([]FeedSource, 0, len(manifest.Sources))
	for _, s := range manifest.Sources {
		fs := FeedSource{
			ID:                s.ID,
			Path:              s.Path,
			Domain:            s.Domain,
			Type:              s.Type,
			Owner:             s.Owner,
			Confidentiality:   s.Confidentiality,
			Hash:              s.Hash,
			Status:            s.Status,
			AdmissionStatus:   s.AdmissionStatus,
			AtomizationStatus: s.AtomizationStatus,
			ExclusionReason:   s.ExclusionReason,
			SourceRole:        s.SourceRole,
			FormatSupport:     s.FormatSupport,
			DerivativeOf:      s.DerivativeOf,
		}
		if err := fs.Validate(); err != nil {
			return Feed{}, fmt.Errorf("feed source %q: %w", fs.ID, err)
		}
		if err := ValidateAtomizedAgainstUnitCount(fs.Admission(), fs.ID, unitsBySource[fs.ID]); err != nil {
			return Feed{}, err
		}
		sources = append(sources, fs)
	}

	lockStatus, err := verifyFeedLockfile(input.Lockfile, input.Snapshot)
	if err != nil {
		return Feed{}, err
	}
	rag, err := buildRAGMetadata(extracted, sources, ts)
	if err != nil {
		return Feed{}, err
	}
	index := buildCorpusIndex(manifest, units, rag, ts)
	var snapshotSummary *FeedSnapshotSummary
	var attestation *CorpusAttestationStatement
	if input.Snapshot != nil {
		summary := summarizeSnapshot(*input.Snapshot)
		snapshotSummary = &summary
		if input.CorpusID != "" && input.ProjectID != "" {
			files := make([]string, 0, len(input.Snapshot.Sources))
			for _, source := range input.Snapshot.Sources {
				files = append(files, source.Path+" "+source.Hash)
			}
			attestationVerdict := firstNonEmpty(input.AttestationVerdict, VerdictAdmissible)
			attestationConfidence := firstNonEmpty(input.AttestationConfidence, "high")
			attestationScope := firstNonEmpty(input.AttestationScope, "restricted_snapshot")
			statement, err := GenerateCorpusAttestation(CorpusAttestationOptions{
				CorpusID:       input.CorpusID,
				ProjectID:      input.ProjectID,
				ScannerVersion: input.ScannerVersion,
				Scope:          attestationScope,
				Verdict:        attestationVerdict,
				Confidence:     attestationConfidence,
				FilesScanned:   input.Snapshot.TotalFiles,
				UnitsExtracted: len(units),
				ScannedFiles:   files,
				Diagnosis:      input.AttestationDiagnosis,
				Policy:         input.Policy,
				Now:            ts,
				Metadata: map[string]any{
					"repository": input.Snapshot.Repository,
					"branch":     input.Snapshot.Branch,
					"commit":     input.Snapshot.Commit,
				},
			})
			if err != nil {
				return Feed{}, err
			}
			attestation = &statement
		}
	}

	feed := Feed{
		Format:      FeedFormat,
		GeneratedAt: ts.Format(time.RFC3339),
		UnitCount:   len(units),
		SourceCount: len(sources),
		Units:       units,
		Sources:     sources,
		Snapshot:    snapshotSummary,
		CorpusIndex: index,
		RAGMetadata: rag,
		Attestation: attestation,
		Lockfile:    lockStatus,
	}

	feed.ContentHash = computeFeedHash(feed)

	return feed, nil
}

// MarshalFeed serialises a feed to indented JSON.
func MarshalFeed(feed Feed) ([]byte, error) {
	return json.MarshalIndent(feed, "", "  ")
}

func extractFeedUnits(root string, manifest SidecarManifest) ([]extractedFeedUnit, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	var result []extractedFeedUnit
	seenUnitIDs := map[string]int{}
	for _, source := range manifest.Sources {
		if source.Status != "" && source.Status != "active" && source.Status != "needs_review" {
			continue
		}
		if !shouldExtractSourceUnits(source) {
			continue
		}
		absPath := filepath.Join(root, filepath.FromSlash(source.Path))
		ext := strings.ToLower(filepath.Ext(source.Path))
		switch {
		case source.Type == "markdown" || ext == ".md" || ext == ".mdx":
			content, err := os.ReadFile(absPath)
			if err != nil {
				return nil, fmt.Errorf("read markdown %s: %w", source.Path, err)
			}
			units, err := markdownFeedUnitsFromBytes(content, source, seenUnitIDs)
			if err != nil {
				return nil, err
			}
			result = append(result, units...)
		case ext == ".yaml" || ext == ".yml":
			units, err := extractParcoursFeedUnits(absPath, source)
			if err != nil || len(units) == 0 {
				units, err = extractStructuredFeedUnits(absPath, source)
				if err != nil {
					return nil, err
				}
			}
			for _, unit := range units {
				unit.UnitID = uniqueFeedUnitID(unit.UnitID, seenUnitIDs)
				result = append(result, unit)
			}
		case ext == ".json":
			units, err := extractStructuredFeedUnits(absPath, source)
			if err != nil {
				return nil, err
			}
			for _, unit := range units {
				unit.UnitID = uniqueFeedUnitID(unit.UnitID, seenUnitIDs)
				result = append(result, unit)
			}
		}
	}
	return result, nil
}

func extractParcoursFeedUnits(path string, source ManifestSource) ([]extractedFeedUnit, error) {
	result, err := ExtractParcours(path)
	if err != nil {
		return nil, err
	}
	units := make([]extractedFeedUnit, 0, len(result.Units))
	for _, unit := range result.Units {
		content := strings.TrimSpace(unit.BusinessRule)
		if content == "" {
			content = unit.Name
		}
		if !isStructuredScalarTextFeedCandidate(content) {
			continue
		}
		segment, hasSegment := parcoursUnitSourceSegment(source, unit, content)
		headingPath := parcoursUnitHeadingPath(result, unit)
		feedUnit := FeedUnit{
			UnitID:       unit.UnitID,
			Name:         unit.Name,
			Domain:       feedDomain(firstNonEmpty(unit.Domain, source.Domain)),
			UnitType:     unit.UnitType,
			Criticality:  feedCriticality(unit.Criticality),
			Status:       "partial",
			BusinessRule: content,
			SourceIDs:    []string{source.ID},
			Gaps:         []string{"Extracted from parcours YAML; requires human canonical review."},
			// FSQ-04 (#367) YAML scalar provenance, propagated from the
			// extractor so consumers can re-prove which YAML key fed
			// BusinessRule and re-read its raw bytes.
			RawText:          unit.RawText,
			DecodedValue:     unit.DecodedValue,
			YAMLPath:         unit.YAMLPath,
			StructuredPath:   unit.YAMLPath,
			StructuredFormat: StructuredFormatYAML,
			NodeKind:         unit.NodeKind,
			SchemaRole:       unit.SchemaRole,
			BusinessRuleMode: unit.BusinessRuleMode,
			HeadingPath:      headingPath,
		}
		if hasSegment {
			feedUnit.SourceSegmentID = segment.SegmentID
			feedUnit.SourceID = source.ID
			feedUnit.SourcePath = source.Path
			feedUnit.StartByte = segment.StartByte
			feedUnit.EndByte = segment.EndByte
			feedUnit.StartLine = segment.StartLine
			feedUnit.EndLine = segment.EndLine
			feedUnit.NormalizedTextHash = segment.NormalizedTextHash
			feedUnit.BodyLedgerSegmentIDs = []string{segment.SegmentID}
		}
		units = append(units, extractedFeedUnit{
			FeedUnit:         feedUnit,
			Content:          content,
			SourceID:         source.ID,
			SourcePath:       source.Path,
			SourceHash:       source.Hash,
			Priority:         feedPriority(source.Priority),
			SourceStatus:     feedSourceStatus(source.Status),
			Locator:          source.Path + "#" + unit.UnitID,
			SourceSegment:    segment,
			HasSourceSegment: hasSegment,
		})
	}
	return units, nil
}

func extractStructuredFeedUnits(path string, source ManifestSource) ([]extractedFeedUnit, error) {
	format, ok := StructuredFormatForPath(source.Path)
	if !ok {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read structured %s: %w", source.Path, err)
	}
	scan, err := ScanStructuredScalars(source, content, format)
	if err != nil {
		return nil, fmt.Errorf("scan structured %s: %w", source.Path, err)
	}
	units := make([]extractedFeedUnit, 0, len(scan.Scalars))
	for _, scalar := range scan.Scalars {
		value := strings.TrimSpace(scalar.DecodedValue)
		if !isStructuredScalarFeedCandidate(scalar) {
			continue
		}
		unitID := scalar.Segment.CanonicalUnitID
		yamlPath := ""
		if format == StructuredFormatYAML {
			yamlPath = scalar.Path
		}
		feedUnit := FeedUnit{
			UnitID:               unitID,
			Name:                 scalar.Path,
			Domain:               feedDomain(source.Domain),
			UnitType:             "structured_scalar",
			Criticality:          "medium",
			Status:               "partial",
			BusinessRule:         value,
			SourceIDs:            []string{source.ID},
			Gaps:                 []string{"Extracted from structured scalar; requires human canonical review."},
			SourceSegmentID:      scalar.Segment.SegmentID,
			SourceID:             source.ID,
			SourcePath:           source.Path,
			StartByte:            scalar.StartByte,
			EndByte:              scalar.EndByte,
			StartLine:            scalar.StartLine,
			EndLine:              scalar.EndLine,
			NormalizedTextHash:   scalar.Segment.NormalizedTextHash,
			HeadingPath:          structuredHeadingPath(source, scalar),
			RawText:              scalar.RawText,
			DecodedValue:         scalar.DecodedValue,
			YAMLPath:             yamlPath,
			StructuredPath:       scalar.Path,
			StructuredFormat:     format,
			NodeKind:             scalar.NodeKind,
			SchemaRole:           "structured_scalar",
			BusinessRuleMode:     "decoded",
			BodyLedgerSegmentIDs: []string{scalar.Segment.SegmentID},
		}
		units = append(units, extractedFeedUnit{
			FeedUnit:         feedUnit,
			Content:          value,
			SourceID:         source.ID,
			SourcePath:       source.Path,
			SourceHash:       source.Hash,
			Priority:         feedPriority(source.Priority),
			SourceStatus:     feedSourceStatus(source.Status),
			Locator:          source.Path + "#" + scalar.Path,
			SourceSegment:    scalar.Segment,
			HasSourceSegment: true,
		})
	}
	return units, nil
}

func isStructuredScalarFeedCandidate(scalar StructuredScalar) bool {
	value := strings.TrimSpace(scalar.DecodedValue)
	if scalar.NodeKind != "scalar_string" {
		return false
	}
	return isStructuredScalarTextFeedCandidate(value)
}

func isStructuredScalarTextFeedCandidate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || isJunkSemantic([]byte(value)) {
		return false
	}
	return tokenCountSemantic(value) >= 3 && runeCount(value) >= 20
}

func shouldExtractSourceUnits(source ManifestSource) bool {
	return source.AdmissionStatus == AdmissionAdmitted &&
		source.AtomizationStatus == AtomizationAtomized
}

func structuredHeadingPath(source ManifestSource, scalar StructuredScalar) []string {
	var out []string
	if source.Path != "" {
		out = append(out, source.Path)
	}
	if scalar.Path != "" {
		out = append(out, scalar.Path)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parcoursUnitSourceSegment(source ManifestSource, unit ParcoursUnit, content string) (SourceSegment, bool) {
	if unit.EndByte <= unit.StartByte {
		return SourceSegment{}, false
	}
	startLine := unit.StartLine
	if startLine == 0 {
		startLine = 1
	}
	endLine := unit.EndLine
	if endLine < startLine {
		endLine = startLine
	}
	raw := unit.RawText
	if raw == "" {
		raw = content
	}
	normalized := firstNonEmpty(content, unit.DecodedValue, unit.RawText)
	seg := SourceSegment{
		SegmentID:          segmentID(source.ID, unit.StartByte, unit.EndByte, KindYAMLScalar),
		SourceID:           source.ID,
		SourcePath:         source.Path,
		Kind:               KindYAMLScalar,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          unit.StartByte,
		EndByte:            unit.EndByte,
		StartLine:          startLine,
		StartColumn:        unit.StartColumn,
		EndLine:            endLine,
		EndColumn:          unit.StartColumn + len([]rune(raw)),
		RawTextHash:        ComputeRawTextHash([]byte(raw)),
		NormalizedTextHash: ComputeNormalizedTextHash(normalized),
		CanonicalUnitID:    unit.UnitID,
		IncludeInFeed:      true,
		IncludeInRAG:       true,
	}
	if seg.StartColumn == 0 {
		seg.StartColumn = 1
	}
	if seg.EndColumn <= seg.StartColumn {
		seg.EndColumn = seg.StartColumn + 1
	}
	return seg, true
}

func parcoursUnitHeadingPath(result ExtractResult, unit ParcoursUnit) []string {
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if len(out) > 0 && out[len(out)-1] == v {
			return
		}
		out = append(out, v)
	}
	add(result.ParcoursName)
	add(unit.EtapeName)
	add(unit.Name)
	if len(out) == 0 {
		return nil
	}
	return out
}

func verifyFeedLockfile(lockfile *Lockfile, snapshot *Snapshot) (*FeedLockfileStatus, error) {
	if lockfile == nil || snapshot == nil {
		return nil, nil
	}
	unapproved := lockfile.Verify(*snapshot)
	status := &FeedLockfileStatus{
		Accepted:        len(unapproved) == 0,
		ApprovedCount:   len(snapshot.Sources) - len(unapproved),
		UnapprovedCount: len(unapproved),
	}
	for _, source := range unapproved {
		status.UnapprovedPaths = append(status.UnapprovedPaths, source.Path)
	}
	if !status.Accepted {
		return status, fmt.Errorf("%w: %d file(s) not approved", ErrSnapshotNotApproved, len(unapproved))
	}
	return status, nil
}

func buildRAGMetadata(units []extractedFeedUnit, sources []FeedSource, generatedAt time.Time) ([]ChunkMetadata, error) {
	metadata := make([]ChunkMetadata, 0, len(units))
	var sourceBacked []FeedUnit
	var sourceSegments []SourceSegment
	for _, unit := range units {
		content := strings.TrimSpace(unit.Content)
		if content == "" {
			continue
		}
		if unit.HasSourceSegment && strings.TrimSpace(unit.SourceSegmentID) != "" {
			sourceBacked = append(sourceBacked, unit.FeedUnit)
			sourceSegments = append(sourceSegments, unit.SourceSegment)
			continue
		}
		meta, err := Enrich(ChunkInput{
			Content:    content,
			SourceID:   unit.SourceID,
			SourcePath: unit.SourcePath,
			SourceHash: unit.SourceHash,
			Domain:     feedDomain(unit.Domain),
			UnitIDs:    []string{unit.UnitID},
			Locator:    unit.Locator,
			Priority:   feedPriority(unit.Priority),
			Status:     feedSourceStatus(unit.SourceStatus),
			Confidence: "medium",
			Tags:       []string{unit.UnitType},
		}, EnrichConfig{
			IngestionVersion: FeedFormat,
			Now:              generatedAt,
		})
		if err != nil {
			return nil, err
		}
		metadata = append(metadata, meta)
	}
	if len(sourceBacked) > 0 {
		chunks, err := ComposeRAGChunks(RAGComposeInput{
			FeedUnits: sourceBacked,
			Sources:   sources,
			Segments:  sourceSegments,
			Profile:   feedRAGComposeProfile(),
			BaseConfig: RAGBuildInput{
				Priority:   "primary",
				Status:     "active",
				Confidence: "medium",
			},
		})
		if err != nil {
			return nil, err
		}
		metadata = append(metadata, chunks...)
	}
	return metadata, nil
}

func feedRAGComposeProfile() SemanticQualityProfile {
	return SemanticQualityProfile{
		MinTokensByKind: map[string]int{
			profileKindDefault: 1,
		},
		MinCharsByKind: map[string]int{
			profileKindDefault: 1,
		},
	}
}

func buildCorpusIndex(manifest SidecarManifest, units []FeedUnit, chunks []ChunkMetadata, generatedAt time.Time) *CorpusIndex {
	unitIDsBySource := map[string][]string{}
	for _, unit := range units {
		for _, sourceID := range unit.SourceIDs {
			unitIDsBySource[sourceID] = append(unitIDsBySource[sourceID], unit.UnitID)
		}
	}
	chunkIDsBySource := map[string][]string{}
	for _, chunk := range chunks {
		chunkIDsBySource[chunk.SourceID] = append(chunkIDsBySource[chunk.SourceID], chunk.ChunkID)
	}

	sources := make([]CorpusIndexSource, 0, len(manifest.Sources))
	for _, source := range manifest.Sources {
		sources = append(sources, CorpusIndexSource{
			SourceID: source.ID,
			Path:     source.Path,
			Hash:     source.Hash,
			Domain:   source.Domain,
			Type:     source.Type,
			UnitIDs:  unitIDsBySource[source.ID],
			ChunkIDs: chunkIDsBySource[source.ID],
		})
	}
	return &CorpusIndex{
		Format:      "nomos.corpus-index.v1",
		GeneratedAt: generatedAt.Format(time.RFC3339),
		SourceCount: len(manifest.Sources),
		UnitCount:   len(units),
		ChunkCount:  len(chunks),
		Sources:     sources,
	}
}

func summarizeSnapshot(snapshot Snapshot) FeedSnapshotSummary {
	return FeedSnapshotSummary{
		Format:     snapshot.Format,
		CorpusRoot: snapshot.CorpusRoot,
		Repository: snapshot.Repository,
		Branch:     snapshot.Branch,
		Commit:     snapshot.Commit,
		TotalFiles: snapshot.TotalFiles,
		TotalBytes: snapshot.TotalBytes,
	}
}

func computeFeedHash(feed Feed) string {
	// Hash over units and sources, excluding the content_hash itself.
	tmp := feed
	tmp.ContentHash = ""
	data, _ := json.Marshal(tmp)
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func feedDomain(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "rbok"
	}
	return value
}

func feedPriority(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "primary"
	}
	return value
}

func feedSourceStatus(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "active"
	}
	return value
}

func feedCriticality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueFeedUnitID(base string, seen map[string]int) string {
	base = toUpperSlug(base)
	if base == "" {
		base = "RBOK-UNIT"
	}
	count := seen[base]
	seen[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s-%02d", base, count+1)
}

// markdownFeedUnitsFromBytes is the SFI-05 (#343) entry point for
// markdown-source-derived feed units. It scans the source bytes once,
// then defers to feedUnitsFromSegments for gating and emission.
func markdownFeedUnitsFromBytes(content []byte, source ManifestSource, seenUnitIDs map[string]int) ([]extractedFeedUnit, error) {
	if len(content) == 0 {
		return nil, nil
	}
	segments, err := ScanMarkdown(source.ID, source.Path, content)
	if err != nil {
		return nil, fmt.Errorf("scan markdown %s: %w", source.Path, err)
	}
	return feedUnitsFromSegments(content, segments, source, seenUnitIDs)
}

// feedUnitsFromSegments runs the SFI-04 source integrity gate over the
// supplied (content, segments) pair and emits one feed unit per
// canonical_atom segment. It fails closed: if the gate would reject
// the ledger, no feed unit is produced and a non-nil error including
// the finding count and the first finding's stable code is returned.
func feedUnitsFromSegments(content []byte, segments []SourceSegment, source ManifestSource, seenUnitIDs map[string]int) ([]extractedFeedUnit, error) {
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: source.ID, Path: source.Path, Content: content}},
		segments,
	)
	if report.Status != "pass" {
		first := report.Findings[0]
		return nil, fmt.Errorf(
			"feed: source integrity gate failed for %s: %d finding(s); first=%s: %s",
			source.Path, len(report.Findings), first.Code, first.Message,
		)
	}
	return buildFeedUnitsFromSegments(content, segments, source, seenUnitIDs), nil
}

// buildFeedUnitsFromSegments walks segments in order, maintains a
// heading stack so each canonical_atom inherits the enclosing
// HeadingPath, and emits one feed unit per canonical_atom. Caller
// must have already proved integrity via CheckSourceIntegrity.
func buildFeedUnitsFromSegments(content []byte, segments []SourceSegment, source ManifestSource, seenUnitIDs map[string]int) []extractedFeedUnit {
	type frame struct {
		level int
		title string
	}
	var stack []frame
	cloneAncestry := func() []string {
		if len(stack) == 0 {
			return nil
		}
		out := make([]string, len(stack))
		for i, f := range stack {
			out[i] = f.title
		}
		return out
	}

	var out []extractedFeedUnit
	for _, seg := range segments {
		if seg.Kind == KindHeading && seg.ParentSegmentID == "" {
			level, title := parseHeadingLevelTitle(string(content[seg.StartByte:seg.EndByte]))
			if level < 1 || level > 6 || strings.TrimSpace(title) == "" {
				continue
			}
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, frame{level: level, title: title})
			continue
		}
		if seg.Disposition != DispositionCanonicalAtom {
			continue
		}
		// SFI-03 (#341) parity: only emit canonical-atom feed units
		// that sit inside a heading scope. Pre-heading text would
		// surface as a coverage finding, not a feed unit.
		if len(stack) == 0 {
			continue
		}
		ancestors := cloneAncestry()
		title := stack[len(stack)-1].title
		// FSQ-03 (#366): table_row data segments carry pre-assembled
		// "Col1=Val1; Col2=Val2; ..." canonical text on RowCanonicalText.
		// Using the raw line bytes here would surface "| Val | Val |" pipe
		// noise into the feed and downstream RAG.
		var display string
		unitType := "rule"
		if seg.Kind == KindTableRow && strings.TrimSpace(seg.RowCanonicalText) != "" {
			display = strings.TrimSpace(seg.RowCanonicalText)
			unitType = "table_row"
		} else {
			text := strings.TrimRight(string(content[seg.StartByte:seg.EndByte]), "\n")
			display = strings.TrimSpace(text)
		}
		if display == "" {
			display = title
		}
		leafID := unitIDLeaf(source.Path, title, seg.Kind, seg.StartLine)
		unitID := uniqueFeedUnitID(toUpperSlug("RBOK-MD-"+source.ID+"-"+leafID), seenUnitIDs)
		fu := FeedUnit{
			UnitID:             unitID,
			Name:               title,
			Domain:             feedDomain(source.Domain),
			UnitType:           unitType,
			Criticality:        "medium",
			Status:             "partial",
			BusinessRule:       display,
			SourceIDs:          []string{source.ID},
			Gaps:               []string{"Extracted from corpus text; requires human canonical review."},
			SourceSegmentID:    seg.SegmentID,
			SourceID:           source.ID,
			SourcePath:         source.Path,
			StartByte:          seg.StartByte,
			EndByte:            seg.EndByte,
			StartLine:          seg.StartLine,
			EndLine:            seg.EndLine,
			NormalizedTextHash: seg.NormalizedTextHash,
			HeadingPath:        ancestors,
		}
		if seg.Kind == KindTableRow {
			fu.TableID = seg.TableID
			fu.TableRole = seg.TableRole
			fu.RowIndex = seg.RowIndex
			if len(seg.ColumnHeaders) > 0 {
				fu.ColumnHeaders = append([]string(nil), seg.ColumnHeaders...)
			}
			// FSQ-05 (#368): a table_row composed unit subsumes the row
			// segment plus every child table_cell, so the body-ledger
			// linkage must list all of them.
			fu.BodyLedgerSegmentIDs = collectBodyLedgerSegmentIDs(seg.SegmentID, segments)
		} else {
			// FSQ-05 (#368): single-segment source-derived units link to
			// exactly one body-ledger segment.
			fu.BodyLedgerSegmentIDs = []string{seg.SegmentID}
		}
		out = append(out, extractedFeedUnit{
			FeedUnit:         fu,
			Content:          display,
			SourceID:         source.ID,
			SourcePath:       source.Path,
			SourceHash:       source.Hash,
			Priority:         feedPriority(source.Priority),
			SourceStatus:     feedSourceStatus(source.Status),
			Locator:          fmt.Sprintf("%s:%d", source.Path, seg.StartLine),
			SourceSegment:    seg,
			HasSourceSegment: true,
		})
	}
	return out
}

// collectBodyLedgerSegmentIDs returns the row segment id followed by every
// child segment id (in source order). FSQ-05 (#368): a composed table_row
// feed unit subsumes the row segment plus its child table_cell segments,
// so the body-ledger linkage must enumerate all of them. The lookup walks
// the supplied segments slice once; it is O(n).
func collectBodyLedgerSegmentIDs(rowID string, segments []SourceSegment) []string {
	out := []string{rowID}
	for _, s := range segments {
		if s.ParentSegmentID == rowID {
			out = append(out, s.SegmentID)
		}
	}
	return out
}
