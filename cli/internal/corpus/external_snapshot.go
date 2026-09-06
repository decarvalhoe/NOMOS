package corpus

// #611 — external, immutable corpus snapshots as NOMOS input.
//
// The operational store (PostgreSQL, a crawler's database, anything live) stays
// external and mutable. NOMOS never reads it. What NOMOS consumes is a SNAPSHOT:
// an envelope naming what was exported, when, by what, and a Merkle root over
// every exported record — plus the records themselves, one JSON object per
// line. Re-hashing the records must reproduce the root, or the snapshot is not
// the one the envelope describes.
//
// Three properties, each refused when it fails:
//
//  1. IMMUTABLE OR REFUSED. The envelope declares `immutable: true` and a
//     `content_hash_root`; the verifier recomputes the root from the records.
//     One changed byte in any record, one record added or dropped, and the
//     root no longer matches. A snapshot that cannot be re-hashed to its own
//     root is a view, not a snapshot, and the strict gate blocks it.
//  2. NOTHING IS LOST ON IMPORT. Every record keeps its source_id, version_id,
//     locator, content_hash and captured_at on the way into a source manifest;
//     web records carry their #610 provenance through untouched. The import is
//     a projection, never a rewrite.
//  3. THE COUNTS ARE CHECKED, NOT TRUSTED. source_count and version_count in
//     the envelope must equal what the records contain. An envelope that
//     claims more than it ships is incomplete; one that claims less is hiding
//     something. Both are refused.
//
// The Merkle machinery is the body ledger's, already proven: buildMerkleProofs
// and VerifyMerkleProof, with the same pair-hash convention. A record's
// inclusion in the snapshot is therefore provable the same way a segment's
// inclusion in a ledger is.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExternalSnapshotFormat identifies the envelope.
const ExternalSnapshotFormat = "nomos.external-snapshot.v1"

// Stable error code substrings — the public contract downstream audits key off.
const (
	ErrCodeSnapshotBadFormat       = "SNAPSHOT_BAD_FORMAT"
	ErrCodeSnapshotNoID            = "SNAPSHOT_NO_ID"
	ErrCodeSnapshotNoGeneratedAt   = "SNAPSHOT_NO_GENERATED_AT"
	ErrCodeSnapshotNoProducer      = "SNAPSHOT_NO_PRODUCER"
	ErrCodeSnapshotNotImmutable    = "SNAPSHOT_NOT_IMMUTABLE"
	ErrCodeSnapshotNoRoot          = "SNAPSHOT_NO_CONTENT_HASH_ROOT"
	ErrCodeSnapshotRootMismatch    = "SNAPSHOT_ROOT_MISMATCH"
	ErrCodeSnapshotCountMismatch   = "SNAPSHOT_COUNT_MISMATCH"
	ErrCodeSnapshotEmpty           = "SNAPSHOT_EMPTY"
	ErrCodeSnapshotRecordMalformed = "SNAPSHOT_RECORD_MALFORMED"
	ErrCodeSnapshotDuplicate       = "SNAPSHOT_DUPLICATE_VERSION"
	ErrCodeSnapshotUnstableHash    = "SNAPSHOT_UNSTABLE_HASH"
	ErrCodeSnapshotBadCapturedAt   = "SNAPSHOT_BAD_CAPTURED_AT"
)

// ExternalSnapshotClaimBoundary travels with every verification and import.
const ExternalSnapshotClaimBoundary = "An immutable export consumed as-is; NOMOS never reads the " +
	"operational store. The root proves these records are the ones exported, not that the " +
	"export was complete relative to the store, nor that any record's content is correct."

// ExternalSnapshot is the envelope. Records live beside it, one JSON object per
// line, so a producer can stream them and a verifier can re-hash them without
// loading a database.
type ExternalSnapshot struct {
	Format          string `json:"format"`
	SnapshotID      string `json:"snapshot_id"`
	GeneratedAt     string `json:"generated_at"`
	Producer        string `json:"producer"`
	DBSchemaVersion string `json:"db_schema_version,omitempty"`
	Immutable       bool   `json:"immutable"`
	SourceCount     int    `json:"source_count"`
	VersionCount    int    `json:"version_count"`
	ContentHashRoot string `json:"content_hash_root"`
	RecordsFile     string `json:"records_file,omitempty"`
	ClaimBoundary   string `json:"claim_boundary,omitempty"`
}

// SnapshotRecord is one exported source version. Locator is a path for a file
// export and a canonical URL for a web export; a web record also carries its
// #610 provenance, which the import hands to the manifest untouched.
type SnapshotRecord struct {
	SourceID    string     `json:"source_id"`
	VersionID   string     `json:"version_id"`
	Locator     string     `json:"locator"`
	ContentHash string     `json:"content_hash"`
	SizeBytes   int64      `json:"size_bytes,omitempty"`
	CapturedAt  string     `json:"captured_at"`
	SourceType  string     `json:"source_type,omitempty"`
	WebSource   *WebSource `json:"web_source,omitempty"`
	// ExportPath is where the producer wrote the normalised export of this
	// record (a Markdown file for a web page). A web record has two facts:
	// its identity — the locator, a URL — and its export, a file the pipeline
	// can atomise. Both are kept; import uses ExportPath as the manifest path
	// when present and falls back to the locator otherwise (#612).
	ExportPath string `json:"export_path,omitempty"`
}

// SnapshotVerification is the emitted verdict.
type SnapshotVerification struct {
	SnapshotID      string        `json:"snapshot_id"`
	RecordCount     int           `json:"record_count"`
	SourceCount     int           `json:"source_count"`
	VersionCount    int           `json:"version_count"`
	DeclaredRoot    string        `json:"declared_root"`
	RecomputedRoot  string        `json:"recomputed_root"`
	Proofs          []MerkleProof `json:"proofs,omitempty"`
	ClaimBoundary   string        `json:"claim_boundary"`
	Status          string        `json:"status"`
	Problem         string        `json:"problem,omitempty"`
	DeclaredCounts  [2]int        `json:"declared_counts"`
	RecordsVerified bool          `json:"records_verified"`
}

// snapshotLeafHash binds identity and content. The version id is part of the
// leaf so two versions of one source hash to two leaves; the locator is not,
// so moving a file without changing it does not silently re-root the snapshot
// — that is a different kind of change, visible in the record, not in the root.
func snapshotLeafHash(r SnapshotRecord) string {
	return ComputeRawTextHash([]byte(fmt.Sprintf(
		"snapshot-record\x00%s\x00%s\x00%s", r.SourceID, r.VersionID, strings.ToLower(r.ContentHash),
	)))
}

// snapshotRecordKey orders records deterministically so the root does not
// depend on export order.
func snapshotRecordKey(r SnapshotRecord) string { return r.SourceID + "\x00" + r.VersionID }

// ComputeSnapshotRoot returns the Merkle root and per-record proofs over the
// records sorted by (source_id, version_id). Deterministic under permutation.
func ComputeSnapshotRoot(records []SnapshotRecord) (string, []MerkleProof) {
	sorted := append([]SnapshotRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return snapshotRecordKey(sorted[i]) < snapshotRecordKey(sorted[j])
	})
	leaves := make([]string, len(sorted))
	for i, r := range sorted {
		leaves[i] = snapshotLeafHash(r)
	}
	if len(leaves) == 0 {
		return "", nil
	}
	return buildMerkleProofs(leaves)
}

// ValidateRecord checks one record's own shape. It does not know about the
// envelope; that is Verify's job.
func ValidateRecord(r SnapshotRecord) error {
	if strings.TrimSpace(r.SourceID) == "" || strings.TrimSpace(r.VersionID) == "" {
		return fmt.Errorf("%s: source_id and version_id are required", ErrCodeSnapshotRecordMalformed)
	}
	if strings.TrimSpace(r.Locator) == "" {
		return fmt.Errorf("%s: %s@%s has no locator", ErrCodeSnapshotRecordMalformed, r.SourceID, r.VersionID)
	}
	if !webHashRe.MatchString(strings.TrimSpace(r.ContentHash)) {
		return fmt.Errorf("%s: %s@%s content_hash %q must be algo:hex (sha256|sha384|sha512)",
			ErrCodeSnapshotUnstableHash, r.SourceID, r.VersionID, r.ContentHash)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(r.CapturedAt)); err != nil {
		return fmt.Errorf("%s: %s@%s captured_at %q is not RFC 3339",
			ErrCodeSnapshotBadCapturedAt, r.SourceID, r.VersionID, r.CapturedAt)
	}
	if r.WebSource != nil {
		// A web record's provenance must already stand on its own; admission is
		// decided later, at the manifest, so validate as not-yet-admitted here.
		if err := r.WebSource.Validate(false); err != nil {
			return fmt.Errorf("%s: %s@%s web_source: %v", ErrCodeSnapshotRecordMalformed, r.SourceID, r.VersionID, err)
		}
	}
	return nil
}

// ReadSnapshotRecords parses a JSONL stream. A blank line is skipped; anything
// else that is not one JSON object per line is refused — a producer that cannot
// emit clean JSONL has not produced a snapshot.
func ReadSnapshotRecords(r io.Reader) ([]SnapshotRecord, error) {
	var records []SnapshotRecord
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var rec SnapshotRecord
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, fmt.Errorf("%s: line %d is not a JSON record: %v", ErrCodeSnapshotRecordMalformed, line, err)
		}
		records = append(records, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %v", ErrCodeSnapshotRecordMalformed, err)
	}
	return records, nil
}

// VerifyExternalSnapshot is the fail-closed contract: envelope shape, every
// record's shape, uniqueness, counts, and the root — recomputed, never read
// back. It returns the verification AND an error, so a caller writing evidence
// gets the verdict with its problem named even on failure.
func VerifyExternalSnapshot(env ExternalSnapshot, records []SnapshotRecord) (SnapshotVerification, error) {
	v := SnapshotVerification{
		SnapshotID:     env.SnapshotID,
		RecordCount:    len(records),
		DeclaredRoot:   normalizeRoot(env.ContentHashRoot),
		DeclaredCounts: [2]int{env.SourceCount, env.VersionCount},
		ClaimBoundary:  ExternalSnapshotClaimBoundary,
		Status:         "fail",
	}
	fail := func(err error) (SnapshotVerification, error) {
		v.Problem = err.Error()
		return v, err
	}

	if env.Format != ExternalSnapshotFormat {
		return fail(fmt.Errorf("%s: format %q, expected %q", ErrCodeSnapshotBadFormat, env.Format, ExternalSnapshotFormat))
	}
	if strings.TrimSpace(env.SnapshotID) == "" {
		return fail(fmt.Errorf("%s: snapshot_id required", ErrCodeSnapshotNoID))
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(env.GeneratedAt)); err != nil {
		return fail(fmt.Errorf("%s: generated_at %q is not RFC 3339", ErrCodeSnapshotNoGeneratedAt, env.GeneratedAt))
	}
	if strings.TrimSpace(env.Producer) == "" {
		return fail(fmt.Errorf("%s: producer required — a snapshot names what exported it", ErrCodeSnapshotNoProducer))
	}
	if !env.Immutable {
		return fail(fmt.Errorf("%s: immutable must be true — NOMOS consumes snapshots, never mutable views", ErrCodeSnapshotNotImmutable))
	}
	if v.DeclaredRoot == "" {
		return fail(fmt.Errorf("%s: content_hash_root required", ErrCodeSnapshotNoRoot))
	}
	if len(records) == 0 {
		return fail(fmt.Errorf("%s: no record — an empty snapshot proves nothing", ErrCodeSnapshotEmpty))
	}

	seen := map[string]bool{}
	sources := map[string]bool{}
	for _, r := range records {
		if err := ValidateRecord(r); err != nil {
			return fail(err)
		}
		key := snapshotRecordKey(r)
		if seen[key] {
			return fail(fmt.Errorf("%s: %s@%s appears twice", ErrCodeSnapshotDuplicate, r.SourceID, r.VersionID))
		}
		seen[key] = true
		sources[r.SourceID] = true
	}
	v.SourceCount = len(sources)
	v.VersionCount = len(records)
	v.RecordsVerified = true

	if env.SourceCount != v.SourceCount || env.VersionCount != v.VersionCount {
		return fail(fmt.Errorf("%s: envelope declares %d source(s)/%d version(s), records hold %d/%d — "+
			"an incomplete or padded export", ErrCodeSnapshotCountMismatch,
			env.SourceCount, env.VersionCount, v.SourceCount, v.VersionCount))
	}

	root, proofs := ComputeSnapshotRoot(records)
	v.RecomputedRoot = root
	v.Proofs = proofs
	if root != v.DeclaredRoot {
		return fail(fmt.Errorf("%s: declared %s, records hash to %s — a record changed, was added or dropped since sealing",
			ErrCodeSnapshotRootMismatch, v.DeclaredRoot, root))
	}
	v.Status = "pass"
	return v, nil
}

// SealExternalSnapshot builds the envelope a PRODUCER ships: counts and root
// computed from the records it is about to export. It refuses malformed or
// duplicate records for the same reasons Verify does — a producer that seals
// bad records only defers the refusal.
func SealExternalSnapshot(snapshotID, producer, dbSchemaVersion, generatedAt, recordsFile string, records []SnapshotRecord) (ExternalSnapshot, error) {
	if len(records) == 0 {
		return ExternalSnapshot{}, fmt.Errorf("%s: nothing to seal", ErrCodeSnapshotEmpty)
	}
	seen := map[string]bool{}
	sources := map[string]bool{}
	for _, r := range records {
		if err := ValidateRecord(r); err != nil {
			return ExternalSnapshot{}, err
		}
		key := snapshotRecordKey(r)
		if seen[key] {
			return ExternalSnapshot{}, fmt.Errorf("%s: %s@%s appears twice", ErrCodeSnapshotDuplicate, r.SourceID, r.VersionID)
		}
		seen[key] = true
		sources[r.SourceID] = true
	}
	if _, err := time.Parse(time.RFC3339, generatedAt); err != nil {
		return ExternalSnapshot{}, fmt.Errorf("%s: generated_at %q is not RFC 3339", ErrCodeSnapshotNoGeneratedAt, generatedAt)
	}
	root, _ := ComputeSnapshotRoot(records)
	return ExternalSnapshot{
		Format:          ExternalSnapshotFormat,
		SnapshotID:      snapshotID,
		GeneratedAt:     generatedAt,
		Producer:        producer,
		DBSchemaVersion: dbSchemaVersion,
		Immutable:       true,
		SourceCount:     len(sources),
		VersionCount:    len(records),
		ContentHashRoot: "sha256:" + root,
		RecordsFile:     recordsFile,
		ClaimBoundary:   ExternalSnapshotClaimBoundary,
	}, nil
}

// SnapshotImportOptions carries the manifest-level values a snapshot does not
// know about itself.
type SnapshotImportOptions struct {
	Domain          string
	Owner           string
	License         string
	Confidentiality string
}

// ImportSnapshotToManifest projects verified records into a source manifest.
// It is called ONLY after VerifyExternalSnapshot passed; it does not re-verify,
// and it does not decide admission — every imported source is `admitted` with
// atomization `not_atomized` and role `canonical` pending the pipeline, except
// web records whose provenance says they cannot be admitted, which are imported
// `excluded` with the reason recorded. Nothing is dropped, nothing is renamed.
func ImportSnapshotToManifest(env ExternalSnapshot, records []SnapshotRecord, opts SnapshotImportOptions) SidecarManifest {
	sorted := append([]SnapshotRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return snapshotRecordKey(sorted[i]) < snapshotRecordKey(sorted[j])
	})
	manifest := SidecarManifest{SchemaVersion: "0.1.0"}
	for _, r := range sorted {
		id := strings.ToUpper(strings.NewReplacer("_", "-", ".", "-", "/", "-", ":", "-", " ", "-").Replace(r.SourceID + "-" + r.VersionID))
		srcType := r.SourceType
		if srcType == "" {
			if r.WebSource != nil {
				srcType = "html"
			} else {
				srcType = "markdown"
			}
		}
		path := r.Locator
		if strings.TrimSpace(r.ExportPath) != "" {
			path = r.ExportPath
		}
		m := ManifestSource{
			ID:              id,
			Path:            path,
			Type:            srcType,
			Domain:          opts.Domain,
			Priority:        "primary",
			Status:          "active",
			Hash:            strings.ToLower(r.ContentHash),
			Owner:           opts.Owner,
			License:         opts.License,
			Confidentiality: opts.Confidentiality,
			AllowedUses:     []string{"citation_internal"},
			// Version identity and the snapshot it came from are not optional
			// provenance; they are what makes a later re-hash comparable.
			Notes:             fmt.Sprintf("snapshot=%s version_id=%s captured_at=%s", env.SnapshotID, r.VersionID, r.CapturedAt),
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationNotAtomized,
			// FSQ-02 requires a reason for not_atomized, and there is one: the
			// pipeline has not run yet. Saying so is more exact than borrowing a
			// status that would imply it had.
			ExclusionReason: fmt.Sprintf("not_atomized: imported from snapshot %s, awaiting the pipeline", env.SnapshotID),
			SourceRole:      AdmissionRoleCanonical,
			FormatSupport:   FormatSupported,
			WebSource:       r.WebSource,
		}
		if r.WebSource != nil {
			r.WebSource.Normalize()
			if err := r.WebSource.Validate(true); err != nil {
				// Recordable, not admissible: the provenance says so.
				m.AdmissionStatus = AdmissionExcluded
				m.AtomizationStatus = ""
				m.ExclusionReason = "excluded_by_policy: " + err.Error()
				m.SourceRole = AdmissionRoleReference
			}
		}
		manifest.Sources = append(manifest.Sources, m)
	}
	return manifest
}

// LoadExternalSnapshot reads an envelope and its records from disk. The records
// file defaults to `sources.jsonl` beside the envelope, or to env.RecordsFile.
func LoadExternalSnapshot(envelopePath, recordsPath string) (ExternalSnapshot, []SnapshotRecord, error) {
	raw, err := os.ReadFile(envelopePath)
	if err != nil {
		return ExternalSnapshot{}, nil, fmt.Errorf("read envelope: %w", err)
	}
	var env ExternalSnapshot
	if err := json.Unmarshal(raw, &env); err != nil {
		return ExternalSnapshot{}, nil, fmt.Errorf("%s: envelope is not JSON: %v", ErrCodeSnapshotBadFormat, err)
	}
	if recordsPath == "" {
		// Default: the envelope's declared records file, else sources.jsonl,
		// resolved beside the envelope.
		name := env.RecordsFile
		if name == "" {
			name = "sources.jsonl"
		}
		recordsPath = filepath.Join(filepath.Dir(envelopePath), name)
	}
	f, err := os.Open(recordsPath)
	if err != nil {
		return env, nil, fmt.Errorf("read records: %w", err)
	}
	defer f.Close()
	records, err := ReadSnapshotRecords(f)
	if err != nil {
		return env, nil, err
	}
	return env, records, nil
}

// SnapshotCoverageMetadata is what an attestation binds when it covers an
// external snapshot (#612): the snapshot's identity and root, its counts, and
// how many records are web captures carrying #610 provenance. It says which
// snapshot the attestation is about and which kinds of source it holds; it is
// not a statement that the snapshot covered the producer's operational store.
func SnapshotCoverageMetadata(env ExternalSnapshot, records []SnapshotRecord) map[string]any {
	web := 0
	types := map[string]int{}
	for _, r := range records {
		if r.WebSource != nil {
			web++
		}
		t := r.SourceType
		if t == "" {
			t = "unspecified"
		}
		types[t]++
	}
	return map[string]any{
		"format":            env.Format,
		"snapshot_id":       env.SnapshotID,
		"producer":          env.Producer,
		"generated_at":      env.GeneratedAt,
		"content_hash_root": env.ContentHashRoot,
		"source_count":      env.SourceCount,
		"version_count":     env.VersionCount,
		"records":           len(records),
		"web_sources":       web,
		"source_types":      types,
		"claim_boundary":    ExternalSnapshotClaimBoundary,
	}
}

// normalizeRoot accepts the contract form ("sha256:<hex>", #StableHash in
// specs/external-snapshot.cue) and the bare-hex form older envelopes carried;
// the comparison is always on the hex.
func normalizeRoot(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.TrimPrefix(v, "sha256:")
}
