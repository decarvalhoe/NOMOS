package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// FeedAuditSchemaVersion identifies the JSON shape produced by RunAudit.
// Bump when the report contract changes; downstream consumers (FSQ-06 gate,
// dashboards, CUE schema) key off this string.
const FeedAuditSchemaVersion = "fsq-audit-v1"

// FeedAuditConfig captures the inputs required to compute an audit report.
// All paths are reported verbatim in the output for traceability; only
// CorpusRoot is read from disk (when non-empty).
type FeedAuditConfig struct {
	FeedPath    string
	RAGPath     string
	RAGProvided bool
	CorpusRoot  string
	GeneratedAt time.Time
}

// FeedAuditReport is the top-level JSON object emitted by `feed-audit`.
type FeedAuditReport struct {
	SchemaVersion           string                    `json:"schema_version"`
	GeneratedAt             string                    `json:"generated_at"`
	FeedPath                string                    `json:"feed_path"`
	RAGPath                 *string                   `json:"rag_path"`
	CorpusRoot              *string                   `json:"corpus_root"`
	Totals                  FeedAuditTotals           `json:"totals"`
	UnitKindDistribution    map[string]int            `json:"unit_kind_distribution"`
	ChunkKindDistribution   map[string]int            `json:"chunk_kind_distribution"`
	LengthDistribution      LengthDistributionReport  `json:"length_distribution"`
	DuplicateNormalizedText DuplicateNormalizedReport `json:"duplicate_normalized_text"`
	TableCellRatio          TableCellRatioReport      `json:"table_cell_ratio"`
	YAMLRawDecodedMismatch  YAMLMismatchReport        `json:"yaml_raw_decoded_mismatch"`
	SourceCoverage          SourceCoverageReport      `json:"source_coverage"`
	TopOffenders            TopOffendersReport        `json:"top_offenders"`
}

// FeedAuditTotals summarises high-level counts.
type FeedAuditTotals struct {
	FeedUnitCount          int `json:"feed_unit_count"`
	ChunkCount             int `json:"chunk_count"`
	SourceBackedUnitCount  int `json:"source_backed_unit_count"`
	SourceBackedChunkCount int `json:"source_backed_chunk_count"`
	SourcesDeclaredActive  int `json:"sources_declared_active"`
	SourcesWithZeroUnits   int `json:"sources_with_zero_units"`
}

// LengthDistributionReport is the bucketed histogram of unit text lengths.
type LengthDistributionReport struct {
	Tokens     LengthBuckets `json:"tokens"`
	Characters LengthBuckets `json:"characters"`
}

// LengthBuckets enumerates fixed cumulative-style buckets in document order.
type LengthBuckets struct {
	Le2    int `json:"le_2,omitempty"`
	Le10   int `json:"le_10"`
	Le25   int `json:"le_25,omitempty"`
	Le50   int `json:"le_50,omitempty"`
	Le100  int `json:"le_100,omitempty"`
	Le200  int `json:"le_200,omitempty"`
	Le1000 int `json:"le_1000,omitempty"`
	Gt100  int `json:"gt_100,omitempty"`
	Gt1000 int `json:"gt_1000,omitempty"`
}

// DuplicateNormalizedReport summarises units sharing a normalized-text hash.
type DuplicateNormalizedReport struct {
	GroupCount          int                        `json:"group_count"`
	DuplicatedUnitCount int                        `json:"duplicated_unit_count"`
	TopGroups           []DuplicateNormalizedGroup `json:"top_groups"`
}

// DuplicateNormalizedGroup describes one cluster of identical normalized text.
type DuplicateNormalizedGroup struct {
	NormalizedHash string                 `json:"normalized_hash"`
	Occurrences    int                    `json:"occurrences"`
	SampleText     string                 `json:"sample_text"`
	Examples       []DuplicateUnitExample `json:"examples"`
}

// DuplicateUnitExample is a single deterministic pointer back to the source.
type DuplicateUnitExample struct {
	UnitID     string `json:"unit_id"`
	SourcePath string `json:"source_path"`
	Line       int    `json:"line"`
}

// TableCellRatioReport reports how much of the feed is table-cell-derived.
type TableCellRatioReport struct {
	TableCellUnitCount int     `json:"table_cell_unit_count"`
	TotalUnitCount     int     `json:"total_unit_count"`
	Ratio              float64 `json:"ratio"`
}

// YAMLMismatchReport flags units whose raw source text differs from the
// stored business_rule. The detection is intentionally conservative
// (best-effort) — FSQ-04 (#367) will model this explicitly.
type YAMLMismatchReport struct {
	YAMLUnitCount      int                  `json:"yaml_unit_count"`
	RawDecodedMismatch int                  `json:"raw_decoded_mismatch"`
	Examples           []YAMLMismatchSample `json:"examples"`
}

// YAMLMismatchSample is one example of a divergence.
type YAMLMismatchSample struct {
	UnitID         string `json:"unit_id"`
	SourcePath     string `json:"source_path"`
	Line           int    `json:"line"`
	RawExcerpt     string `json:"raw_excerpt"`
	DecodedExcerpt string `json:"decoded_excerpt"`
}

// SourceCoverageReport summarises corpus-level coverage by file extension.
type SourceCoverageReport struct {
	ByExtension          map[string]ExtensionCoverage `json:"by_extension"`
	SourcesWithZeroUnits []ZeroUnitSource             `json:"sources_with_zero_units"`
}

// ExtensionCoverage is the per-extension fold of source-coverage data.
type ExtensionCoverage struct {
	Sources         int      `json:"sources"`
	WithUnits       int      `json:"with_units"`
	ByteCoveragePct *float64 `json:"byte_coverage_pct"`
}

// ZeroUnitSource is a corpus file present on disk but referenced by no unit.
type ZeroUnitSource struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
}

// TopOffendersReport lists the worst offenders for analyst review.
type TopOffendersReport struct {
	VeryShortUnits  []ShortUnitExample      `json:"very_short_units"`
	DuplicatedUnits []DuplicatedUnitExample `json:"duplicated_units"`
}

// ShortUnitExample names a feed unit whose text is unusually short.
type ShortUnitExample struct {
	UnitID     string `json:"unit_id"`
	SourcePath string `json:"source_path"`
	Line       int    `json:"line"`
	CharCount  int    `json:"char_count"`
	Text       string `json:"text"`
}

// DuplicatedUnitExample names a feed unit that participates in a duplicate
// normalized-text group.
type DuplicatedUnitExample struct {
	UnitID         string `json:"unit_id"`
	SourcePath     string `json:"source_path"`
	Line           int    `json:"line"`
	NormalizedHash string `json:"normalized_hash"`
}

// RunAudit is the deterministic, pure entry point. The cmd/main.go layer
// wires file IO and JSON serialisation around it. Repeated calls with the
// same inputs return byte-identical reports (modulo GeneratedAt, which is
// supplied by the caller).
func RunAudit(feed corpus.Feed, chunks []corpus.ChunkMetadata, cfg FeedAuditConfig) FeedAuditReport {
	report := FeedAuditReport{
		SchemaVersion:         FeedAuditSchemaVersion,
		GeneratedAt:           cfg.GeneratedAt.UTC().Format(time.RFC3339),
		FeedPath:              cfg.FeedPath,
		RAGPath:               nullableString(cfg.RAGPath, cfg.RAGProvided),
		CorpusRoot:            nullableString(cfg.CorpusRoot, cfg.CorpusRoot != ""),
		UnitKindDistribution:  map[string]int{},
		ChunkKindDistribution: map[string]int{},
	}
	report.Totals = computeTotals(feed, chunks)
	report.UnitKindDistribution = computeUnitKindDistribution(feed.Units)
	report.ChunkKindDistribution = computeChunkKindDistribution(chunks)
	report.LengthDistribution = computeLengthDistribution(feed.Units)
	report.DuplicateNormalizedText = computeDuplicates(feed.Units)
	report.TableCellRatio = computeTableCellRatio(feed.Units, report.UnitKindDistribution)
	report.YAMLRawDecodedMismatch = computeYAMLMismatch(feed.Units, cfg.CorpusRoot)
	report.SourceCoverage = computeSourceCoverage(feed.Units, feed.Sources, cfg.CorpusRoot)
	report.TopOffenders = computeTopOffenders(feed.Units, report.DuplicateNormalizedText)
	return report
}

// nullableString returns &v when present, else nil — mapped to JSON null.
func nullableString(v string, present bool) *string {
	if !present {
		return nil
	}
	return &v
}

// ----------------------------------------------------------------------------
// Totals.
// ----------------------------------------------------------------------------

func computeTotals(feed corpus.Feed, chunks []corpus.ChunkMetadata) FeedAuditTotals {
	t := FeedAuditTotals{
		FeedUnitCount: len(feed.Units),
		ChunkCount:    len(chunks),
	}
	for _, u := range feed.Units {
		if isSourceBackedUnit(u) {
			t.SourceBackedUnitCount++
		}
	}
	for _, c := range chunks {
		if strings.TrimSpace(c.SourceSegmentID) != "" {
			t.SourceBackedChunkCount++
		}
	}
	activeIDs := map[string]struct{}{}
	for _, s := range feed.Sources {
		if isActiveSource(s.Status) {
			activeIDs[s.ID] = struct{}{}
		}
	}
	t.SourcesDeclaredActive = len(activeIDs)

	usedSources := map[string]struct{}{}
	for _, u := range feed.Units {
		for _, sid := range u.SourceIDs {
			usedSources[sid] = struct{}{}
		}
		if u.SourceID != "" {
			usedSources[u.SourceID] = struct{}{}
		}
	}
	for sid := range activeIDs {
		if _, used := usedSources[sid]; !used {
			t.SourcesWithZeroUnits++
		}
	}
	return t
}

// isActiveSource treats "" as "active" (the GenerateFeed default).
func isActiveSource(status string) bool {
	s := strings.TrimSpace(status)
	return s == "" || s == "active"
}

func isSourceBackedUnit(u corpus.FeedUnit) bool {
	return strings.TrimSpace(u.SourceSegmentID) != "" ||
		strings.TrimSpace(u.SourcePath) != "" ||
		strings.TrimSpace(u.NormalizedTextHash) != "" ||
		u.StartByte != 0 ||
		u.EndByte != 0 ||
		len(u.HeadingPath) > 0
}

// ----------------------------------------------------------------------------
// Kind distributions.
// ----------------------------------------------------------------------------

// computeUnitKindDistribution returns a count keyed by the most informative
// per-unit "kind" identifier. For source-derived units the kind is parsed
// from the SourceSegmentID suffix (markdown_scanner.segmentID encodes the
// scanner kind there). For matrix-derived units we fall back to UnitType.
// Empty values are omitted.
func computeUnitKindDistribution(units []corpus.FeedUnit) map[string]int {
	out := map[string]int{}
	for _, u := range units {
		k := unitKind(u)
		if k == "" {
			continue
		}
		out[k]++
	}
	return out
}

// unitKind picks the best per-unit kind label.
func unitKind(u corpus.FeedUnit) string {
	if k := segmentKindFromID(u.SourceSegmentID); k != "" {
		return k
	}
	return strings.TrimSpace(u.UnitType)
}

// segmentKindFromID returns the trailing kind in a "seg:<src>:<a>-<b>:<kind>"
// segment id, or "" when the id does not match the format.
func segmentKindFromID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "seg:") {
		return ""
	}
	idx := strings.LastIndex(id, ":")
	if idx == -1 || idx == len(id)-1 {
		return ""
	}
	return id[idx+1:]
}

func computeChunkKindDistribution(chunks []corpus.ChunkMetadata) map[string]int {
	out := map[string]int{}
	for _, c := range chunks {
		k := strings.TrimSpace(c.SegmentKind)
		if k == "" {
			continue
		}
		out[k]++
	}
	return out
}

// ----------------------------------------------------------------------------
// Length distribution.
// ----------------------------------------------------------------------------

func computeLengthDistribution(units []corpus.FeedUnit) LengthDistributionReport {
	var out LengthDistributionReport
	for _, u := range units {
		text := u.BusinessRule
		toks := tokenCount(text)
		chars := utf8.RuneCountInString(text)

		switch {
		case toks <= 2:
			out.Tokens.Le2++
			out.Tokens.Le10++
			out.Tokens.Le25++
			out.Tokens.Le100++
		case toks <= 10:
			out.Tokens.Le10++
			out.Tokens.Le25++
			out.Tokens.Le100++
		case toks <= 25:
			out.Tokens.Le25++
			out.Tokens.Le100++
		case toks <= 100:
			out.Tokens.Le100++
		default:
			out.Tokens.Gt100++
		}

		switch {
		case chars <= 10:
			out.Characters.Le10++
			out.Characters.Le50++
			out.Characters.Le200++
			out.Characters.Le1000++
		case chars <= 50:
			out.Characters.Le50++
			out.Characters.Le200++
			out.Characters.Le1000++
		case chars <= 200:
			out.Characters.Le200++
			out.Characters.Le1000++
		case chars <= 1000:
			out.Characters.Le1000++
		default:
			out.Characters.Gt1000++
		}
	}
	return out
}

// tokenCount returns the count of whitespace-separated runs in s. Documented
// in the dispatch as the "tokens" definition for v1 (no tokenizer dep).
func tokenCount(s string) int {
	return len(strings.Fields(s))
}

// ----------------------------------------------------------------------------
// Duplicates.
// ----------------------------------------------------------------------------

func computeDuplicates(units []corpus.FeedUnit) DuplicateNormalizedReport {
	groups := map[string][]int{}
	order := []string{}
	for i, u := range units {
		h := strings.TrimSpace(u.NormalizedTextHash)
		if h == "" {
			continue
		}
		if _, seen := groups[h]; !seen {
			order = append(order, h)
		}
		groups[h] = append(groups[h], i)
	}

	type groupSummary struct {
		hash      string
		indices   []int
		count     int
		firstPath string
		firstUnit string
	}
	var dupGroups []groupSummary
	dupUnitTotal := 0
	for _, h := range order {
		idxs := groups[h]
		if len(idxs) < 2 {
			continue
		}
		dupUnitTotal += len(idxs)
		first := units[idxs[0]]
		dupGroups = append(dupGroups, groupSummary{
			hash:      h,
			indices:   idxs,
			count:     len(idxs),
			firstPath: first.SourcePath,
			firstUnit: first.UnitID,
		})
	}

	sort.SliceStable(dupGroups, func(i, j int) bool {
		if dupGroups[i].count != dupGroups[j].count {
			return dupGroups[i].count > dupGroups[j].count
		}
		if dupGroups[i].firstPath != dupGroups[j].firstPath {
			return dupGroups[i].firstPath < dupGroups[j].firstPath
		}
		return dupGroups[i].firstUnit < dupGroups[j].firstUnit
	})

	const maxTopGroups = 10
	const maxExamplesPerGroup = 5
	report := DuplicateNormalizedReport{
		GroupCount:          len(dupGroups),
		DuplicatedUnitCount: dupUnitTotal,
		TopGroups:           []DuplicateNormalizedGroup{},
	}
	for i, g := range dupGroups {
		if i >= maxTopGroups {
			break
		}
		examples := make([]DuplicateUnitExample, 0, len(g.indices))
		for j, idx := range g.indices {
			if j >= maxExamplesPerGroup {
				break
			}
			u := units[idx]
			examples = append(examples, DuplicateUnitExample{
				UnitID:     u.UnitID,
				SourcePath: u.SourcePath,
				Line:       u.StartLine,
			})
		}
		sort.SliceStable(examples, func(a, b int) bool {
			if examples[a].SourcePath != examples[b].SourcePath {
				return examples[a].SourcePath < examples[b].SourcePath
			}
			return examples[a].UnitID < examples[b].UnitID
		})
		report.TopGroups = append(report.TopGroups, DuplicateNormalizedGroup{
			NormalizedHash: g.hash,
			Occurrences:    g.count,
			SampleText:     truncateForReport(units[g.indices[0]].BusinessRule, 200),
			Examples:       examples,
		})
	}
	return report
}

func truncateForReport(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	out := make([]rune, 0, maxRunes)
	for _, r := range s {
		if len(out) >= maxRunes {
			break
		}
		out = append(out, r)
	}
	return string(out) + "..."
}

// ----------------------------------------------------------------------------
// Table-cell ratio.
// ----------------------------------------------------------------------------

func computeTableCellRatio(units []corpus.FeedUnit, kindDist map[string]int) TableCellRatioReport {
	total := len(units)
	tc := kindDist[corpus.KindTableCell]
	ratio := 0.0
	if total > 0 {
		ratio = float64(tc) / float64(total)
	}
	return TableCellRatioReport{
		TableCellUnitCount: tc,
		TotalUnitCount:     total,
		Ratio:              ratio,
	}
}

// ----------------------------------------------------------------------------
// YAML raw vs decoded mismatch (best-effort, FSQ-04 will model this fully).
// ----------------------------------------------------------------------------

func computeYAMLMismatch(units []corpus.FeedUnit, corpusRoot string) YAMLMismatchReport {
	report := YAMLMismatchReport{Examples: []YAMLMismatchSample{}}
	const maxExamples = 5

	var yamlUnits []corpus.FeedUnit
	for _, u := range units {
		if isYAMLPath(u.SourcePath) {
			yamlUnits = append(yamlUnits, u)
		}
	}
	report.YAMLUnitCount = len(yamlUnits)

	if corpusRoot == "" {
		return report
	}
	for _, u := range yamlUnits {
		if u.StartByte == u.EndByte {
			continue
		}
		raw, err := readSourceSlice(corpusRoot, u.SourcePath, u.StartByte, u.EndByte)
		if err != nil {
			continue
		}
		rawTrim := strings.TrimSpace(raw)
		decTrim := strings.TrimSpace(u.BusinessRule)
		if rawTrim == decTrim {
			continue
		}
		report.RawDecodedMismatch++
		if len(report.Examples) < maxExamples {
			report.Examples = append(report.Examples, YAMLMismatchSample{
				UnitID:         u.UnitID,
				SourcePath:     u.SourcePath,
				Line:           u.StartLine,
				RawExcerpt:     truncateForReport(rawTrim, 120),
				DecodedExcerpt: truncateForReport(decTrim, 120),
			})
		}
	}
	sort.SliceStable(report.Examples, func(i, j int) bool {
		if report.Examples[i].SourcePath != report.Examples[j].SourcePath {
			return report.Examples[i].SourcePath < report.Examples[j].SourcePath
		}
		return report.Examples[i].UnitID < report.Examples[j].UnitID
	})
	return report
}

func isYAMLPath(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".yaml" || ext == ".yml"
}

func readSourceSlice(root, relPath string, start, end int) (string, error) {
	if start < 0 || end < start {
		return "", fmt.Errorf("invalid span [%d,%d)", start, end)
	}
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if end > len(data) {
		end = len(data)
	}
	if start > len(data) {
		start = len(data)
	}
	return string(data[start:end]), nil
}

// ----------------------------------------------------------------------------
// Source coverage by extension.
// ----------------------------------------------------------------------------

func computeSourceCoverage(units []corpus.FeedUnit, sources []corpus.FeedSource, corpusRoot string) SourceCoverageReport {
	report := SourceCoverageReport{
		ByExtension:          map[string]ExtensionCoverage{},
		SourcesWithZeroUnits: []ZeroUnitSource{},
	}
	if corpusRoot == "" {
		return report
	}

	type fileInfo struct {
		relPath string
		size    int64
	}
	filesByExt := map[string][]fileInfo{}

	walkErr := filepath.WalkDir(corpusRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore unreadable entries; report still useful
		}
		if d.IsDir() {
			if filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(corpusRoot, path)
		if err != nil {
			rel = path
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == "" {
			ext = "<none>"
		}
		filesByExt[ext] = append(filesByExt[ext], fileInfo{
			relPath: filepath.ToSlash(rel),
			size:    info.Size(),
		})
		return nil
	})
	_ = walkErr // partial trees are still informative

	pathsWithUnits := map[string]struct{}{}
	bytesByPath := map[string]int{}
	for _, u := range units {
		if p := strings.TrimSpace(u.SourcePath); p != "" {
			pathsWithUnits[p] = struct{}{}
			if u.EndByte > u.StartByte {
				bytesByPath[p] += u.EndByte - u.StartByte
			}
		}
	}

	exts := make([]string, 0, len(filesByExt))
	for ext := range filesByExt {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	for _, ext := range exts {
		files := filesByExt[ext]
		sort.SliceStable(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })
		coverage := ExtensionCoverage{Sources: len(files)}
		var totalSize int64
		var coveredBytes int64
		for _, f := range files {
			totalSize += f.size
			if _, has := pathsWithUnits[f.relPath]; has {
				coverage.WithUnits++
			}
			coveredBytes += int64(bytesByPath[f.relPath])
		}
		if totalSize > 0 && coveredBytes >= 0 {
			pct := float64(coveredBytes) / float64(totalSize) * 100.0
			coverage.ByteCoveragePct = &pct
		}
		report.ByExtension[ext] = coverage
	}

	type zeroEntry struct {
		path string
		size int64
	}
	var zeros []zeroEntry
	for _, files := range filesByExt {
		for _, f := range files {
			if _, has := pathsWithUnits[f.relPath]; !has {
				zeros = append(zeros, zeroEntry{path: f.relPath, size: f.size})
			}
		}
	}
	sort.SliceStable(zeros, func(i, j int) bool { return zeros[i].path < zeros[j].path })
	const maxZero = 50
	for i, z := range zeros {
		if i >= maxZero {
			break
		}
		report.SourcesWithZeroUnits = append(report.SourcesWithZeroUnits, ZeroUnitSource{
			Path:      z.path,
			SizeBytes: z.size,
		})
	}
	_ = sources // declared sources are surfaced via totals, not the by-extension index
	return report
}

// ----------------------------------------------------------------------------
// Top offenders.
// ----------------------------------------------------------------------------

func computeTopOffenders(units []corpus.FeedUnit, dup DuplicateNormalizedReport) TopOffendersReport {
	const maxOffenders = 20
	out := TopOffendersReport{
		VeryShortUnits:  []ShortUnitExample{},
		DuplicatedUnits: []DuplicatedUnitExample{},
	}

	type shortRow struct {
		idx       int
		charCount int
	}
	var shortRows []shortRow
	for i, u := range units {
		shortRows = append(shortRows, shortRow{idx: i, charCount: utf8.RuneCountInString(u.BusinessRule)})
	}
	sort.SliceStable(shortRows, func(i, j int) bool {
		if shortRows[i].charCount != shortRows[j].charCount {
			return shortRows[i].charCount < shortRows[j].charCount
		}
		ui := units[shortRows[i].idx]
		uj := units[shortRows[j].idx]
		if ui.SourcePath != uj.SourcePath {
			return ui.SourcePath < uj.SourcePath
		}
		return ui.UnitID < uj.UnitID
	})
	for i, r := range shortRows {
		if i >= maxOffenders {
			break
		}
		u := units[r.idx]
		out.VeryShortUnits = append(out.VeryShortUnits, ShortUnitExample{
			UnitID:     u.UnitID,
			SourcePath: u.SourcePath,
			Line:       u.StartLine,
			CharCount:  r.charCount,
			Text:       truncateForReport(u.BusinessRule, 120),
		})
	}

	hashToCount := map[string]int{}
	for _, g := range dup.TopGroups {
		hashToCount[g.NormalizedHash] = g.Occurrences
	}
	type dupRow struct {
		idx   int
		hash  string
		count int
	}
	var dupRows []dupRow
	for i, u := range units {
		h := strings.TrimSpace(u.NormalizedTextHash)
		if h == "" {
			continue
		}
		if c, ok := hashToCount[h]; ok && c >= 2 {
			dupRows = append(dupRows, dupRow{idx: i, hash: h, count: c})
		}
	}
	sort.SliceStable(dupRows, func(i, j int) bool {
		if dupRows[i].count != dupRows[j].count {
			return dupRows[i].count > dupRows[j].count
		}
		ui := units[dupRows[i].idx]
		uj := units[dupRows[j].idx]
		if ui.SourcePath != uj.SourcePath {
			return ui.SourcePath < uj.SourcePath
		}
		return ui.UnitID < uj.UnitID
	})
	for i, r := range dupRows {
		if i >= maxOffenders {
			break
		}
		u := units[r.idx]
		out.DuplicatedUnits = append(out.DuplicatedUnits, DuplicatedUnitExample{
			UnitID:         u.UnitID,
			SourcePath:     u.SourcePath,
			Line:           u.StartLine,
			NormalizedHash: r.hash,
		})
	}
	return out
}
