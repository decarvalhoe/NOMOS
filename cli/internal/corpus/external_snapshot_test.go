package corpus

// #611 — an external snapshot is immutable or it is refused.
//
// Doctrine §2.3: the proof is the failure. The central tests mutate one byte of
// one record after sealing and show the root no longer matches; then they show
// that import loses nothing.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func snapshotRecords() []SnapshotRecord {
	web := validWebSource()
	return []SnapshotRecord{
		{SourceID: "reglement", VersionID: "v3", Locator: "captures/reglement-v3.md",
			ContentHash: "sha256:" + strings.Repeat("11", 32), SizeBytes: 1200, CapturedAt: "2026-09-01T08:00:00Z"},
		{SourceID: "reglement", VersionID: "v4", Locator: "captures/reglement-v4.md",
			ContentHash: "sha256:" + strings.Repeat("22", 32), SizeBytes: 1300, CapturedAt: "2026-09-04T08:00:00Z"},
		{SourceID: "art-7", VersionID: "v1", Locator: web.CanonicalURL,
			ContentHash: web.ContentHash, SizeBytes: 900, CapturedAt: web.FetchedAt, SourceType: "html", WebSource: &web},
	}
}

func sealed(t *testing.T) (ExternalSnapshot, []SnapshotRecord) {
	t.Helper()
	records := snapshotRecords()
	env, err := SealExternalSnapshot("snap-2026-09-05-001", "recursio-pg-export/1.2", "pg-schema-7", "2026-09-05T09:00:00Z", "sources.jsonl", records)
	if err != nil {
		t.Fatalf("sealing valid records failed: %v", err)
	}
	return env, records
}

func TestSnapshot_SealedSnapshotVerifies(t *testing.T) {
	env, records := sealed(t)
	v, err := VerifyExternalSnapshot(env, records)
	if err != nil {
		t.Fatalf("a freshly sealed snapshot does not verify: %v", err)
	}
	if v.Status != "pass" || v.RecomputedRoot != env.ContentHashRoot {
		t.Fatalf("verdict: %+v", v)
	}
	if v.SourceCount != 2 || v.VersionCount != 3 {
		t.Fatalf("counts: %d sources, %d versions", v.SourceCount, v.VersionCount)
	}
	if !env.Immutable || env.ClaimBoundary == "" {
		t.Fatal("envelope must be immutable and carry its claim boundary")
	}
	// Every record's inclusion is provable with the body ledger's own verifier.
	for i, r := range records {
		_ = i
		leaf := snapshotLeafHash(r)
		var proof *MerkleProof
		for j := range v.Proofs {
			if v.Proofs[j].LeafHash == leaf {
				proof = &v.Proofs[j]
			}
		}
		if proof == nil {
			t.Fatalf("no proof for %s@%s", r.SourceID, r.VersionID)
		}
		if err := VerifyMerkleProof(leaf, *proof, v.RecomputedRoot); err != nil {
			t.Fatalf("inclusion proof failed for %s@%s: %v", r.SourceID, r.VersionID, err)
		}
	}
}

func TestSnapshot_OneChangedByteBreaksTheRoot(t *testing.T) {
	// THE point of #611: the store may mutate; the snapshot may not.
	env, records := sealed(t)
	records[1].ContentHash = "sha256:" + strings.Repeat("22", 31) + "23"
	_, err := VerifyExternalSnapshot(env, records)
	if err == nil {
		t.Fatal("a mutated record verified against the sealed root")
	}
	if !strings.Contains(err.Error(), ErrCodeSnapshotRootMismatch) {
		t.Fatalf("expected %s, got %v", ErrCodeSnapshotRootMismatch, err)
	}
}

func TestSnapshot_AddedOrDroppedRecordIsCaught(t *testing.T) {
	env, records := sealed(t)
	// Dropped: counts catch it first, honestly named.
	_, err := VerifyExternalSnapshot(env, records[:2])
	if err == nil || !strings.Contains(err.Error(), ErrCodeSnapshotCountMismatch) {
		t.Fatalf("dropped record: expected %s, got %v", ErrCodeSnapshotCountMismatch, err)
	}
	// Added with the counts forged to match: only the root can catch it.
	extra := append(append([]SnapshotRecord(nil), records...), SnapshotRecord{
		SourceID: "padding", VersionID: "v1", Locator: "x.md",
		ContentHash: "sha256:" + strings.Repeat("ff", 32), CapturedAt: "2026-09-05T00:00:00Z"})
	forged := env
	forged.SourceCount, forged.VersionCount = 3, 4
	_, err = VerifyExternalSnapshot(forged, extra)
	if err == nil || !strings.Contains(err.Error(), ErrCodeSnapshotRootMismatch) {
		t.Fatalf("padded record with forged counts: expected %s, got %v", ErrCodeSnapshotRootMismatch, err)
	}
}

func TestSnapshot_RootIsOrderIndependent(t *testing.T) {
	env, records := sealed(t)
	reversed := []SnapshotRecord{records[2], records[1], records[0]}
	v, err := VerifyExternalSnapshot(env, reversed)
	if err != nil {
		t.Fatalf("export order must not matter: %v", err)
	}
	if v.RecomputedRoot != env.ContentHashRoot {
		t.Fatal("root depends on order")
	}
}

func TestSnapshot_EnvelopeDefectsAreRefusedWithTheirCode(t *testing.T) {
	cases := map[string]struct {
		mutate func(*ExternalSnapshot)
		code   string
	}{
		"wrong format":      {func(e *ExternalSnapshot) { e.Format = "nomos.snapshot.v0" }, ErrCodeSnapshotBadFormat},
		"no id":             {func(e *ExternalSnapshot) { e.SnapshotID = " " }, ErrCodeSnapshotNoID},
		"bad generated_at":  {func(e *ExternalSnapshot) { e.GeneratedAt = "yesterday" }, ErrCodeSnapshotNoGeneratedAt},
		"no producer":       {func(e *ExternalSnapshot) { e.Producer = "" }, ErrCodeSnapshotNoProducer},
		"mutable view":      {func(e *ExternalSnapshot) { e.Immutable = false }, ErrCodeSnapshotNotImmutable},
		"no root":           {func(e *ExternalSnapshot) { e.ContentHashRoot = "" }, ErrCodeSnapshotNoRoot},
		"forged root":       {func(e *ExternalSnapshot) { e.ContentHashRoot = "sha256:" + strings.Repeat("00", 32) }, ErrCodeSnapshotRootMismatch},
		"count inflated":    {func(e *ExternalSnapshot) { e.VersionCount = 9 }, ErrCodeSnapshotCountMismatch},
		"count understated": {func(e *ExternalSnapshot) { e.SourceCount = 1 }, ErrCodeSnapshotCountMismatch},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env, records := sealed(t)
			tc.mutate(&env)
			_, err := VerifyExternalSnapshot(env, records)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("%s: expected %s, got %v", name, tc.code, err)
			}
		})
	}
}

func TestSnapshot_RecordDefectsAreRefusedWithTheirCode(t *testing.T) {
	cases := map[string]struct {
		mutate func(*SnapshotRecord)
		code   string
	}{
		"no source_id":     {func(r *SnapshotRecord) { r.SourceID = "" }, ErrCodeSnapshotRecordMalformed},
		"no version_id":    {func(r *SnapshotRecord) { r.VersionID = "" }, ErrCodeSnapshotRecordMalformed},
		"no locator":       {func(r *SnapshotRecord) { r.Locator = "" }, ErrCodeSnapshotRecordMalformed},
		"placeholder hash": {func(r *SnapshotRecord) { r.ContentHash = "placeholder:not-fetched" }, ErrCodeSnapshotUnstableHash},
		"bare hash":        {func(r *SnapshotRecord) { r.ContentHash = strings.Repeat("11", 32) }, ErrCodeSnapshotUnstableHash},
		"bad captured_at":  {func(r *SnapshotRecord) { r.CapturedAt = "2026-09-01" }, ErrCodeSnapshotBadCapturedAt},
		"web without hash": {func(r *SnapshotRecord) { r.WebSource.ContentHash = "" }, ErrCodeSnapshotRecordMalformed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			records := snapshotRecords()
			idx := 0
			if strings.HasPrefix(name, "web") {
				idx = 2
			}
			tc.mutate(&records[idx])
			// Sealing refuses too: a producer cannot seal bad records.
			if _, err := SealExternalSnapshot("s", "p", "", "2026-09-05T09:00:00Z", "", records); err == nil {
				t.Fatalf("%s: sealed", name)
			}
			env, _ := sealed(t)
			_, err := VerifyExternalSnapshot(env, records)
			if err == nil {
				t.Fatalf("%s: verified", name)
			}
			if !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("%s: expected %s, got %v", name, tc.code, err)
			}
		})
	}
}

func TestSnapshot_DuplicateVersionIsRefused(t *testing.T) {
	records := snapshotRecords()
	records = append(records, records[0])
	if _, err := SealExternalSnapshot("s", "p", "", "2026-09-05T09:00:00Z", "", records); err == nil ||
		!strings.Contains(err.Error(), ErrCodeSnapshotDuplicate) {
		t.Fatalf("expected %s, got %v", ErrCodeSnapshotDuplicate, err)
	}
}

func TestSnapshot_EmptySnapshotProvesNothing(t *testing.T) {
	if _, err := SealExternalSnapshot("s", "p", "", "2026-09-05T09:00:00Z", "", nil); err == nil {
		t.Fatal("an empty snapshot was sealed")
	}
	env, _ := sealed(t)
	if _, err := VerifyExternalSnapshot(env, nil); err == nil || !strings.Contains(err.Error(), ErrCodeSnapshotEmpty) {
		t.Fatalf("expected %s, got %v", ErrCodeSnapshotEmpty, err)
	}
}

func TestSnapshot_ReadJSONLRefusesGarbage(t *testing.T) {
	good := `{"source_id":"a","version_id":"v1","locator":"a.md","content_hash":"sha256:` + strings.Repeat("aa", 32) + `","captured_at":"2026-09-05T00:00:00Z"}`
	records, err := ReadSnapshotRecords(strings.NewReader(good + "\n\n" + good + "\n"))
	if err != nil || len(records) != 2 {
		t.Fatalf("clean JSONL with a blank line should read: %v (%d)", err, len(records))
	}
	if _, err := ReadSnapshotRecords(strings.NewReader(good + "\nnot json\n")); err == nil ||
		!strings.Contains(err.Error(), ErrCodeSnapshotRecordMalformed) {
		t.Fatalf("garbage line accepted: %v", err)
	}
}

// --- import: nothing lost -----------------------------------------------

func TestSnapshot_ImportLosesNothing(t *testing.T) {
	env, records := sealed(t)
	if _, err := VerifyExternalSnapshot(env, records); err != nil {
		t.Fatal(err)
	}
	manifest := ImportSnapshotToManifest(env, records, SnapshotImportOptions{
		Domain: "demo", Owner: "owner@example.invalid", License: "internal", Confidentiality: "internal",
	})
	if len(manifest.Sources) != len(records) {
		t.Fatalf("import dropped records: %d -> %d", len(records), len(manifest.Sources))
	}
	byHash := map[string]ManifestSource{}
	for _, m := range manifest.Sources {
		byHash[m.Hash] = m
		if err := m.Validate(); err != nil {
			t.Fatalf("imported source %s does not validate: %v", m.ID, err)
		}
	}
	for _, r := range records {
		m, ok := byHash[strings.ToLower(r.ContentHash)]
		if !ok {
			t.Fatalf("record %s@%s lost its hash on import", r.SourceID, r.VersionID)
		}
		if m.Path != r.Locator {
			t.Fatalf("locator lost: %q vs %q", m.Path, r.Locator)
		}
		for _, must := range []string{"version_id=" + r.VersionID, "captured_at=" + r.CapturedAt, "snapshot=" + env.SnapshotID} {
			if !strings.Contains(m.Notes, must) {
				t.Fatalf("%s@%s lost %q on import: %q", r.SourceID, r.VersionID, must, m.Notes)
			}
		}
		if r.WebSource != nil {
			if m.WebSource == nil || m.WebSource.CanonicalURL != r.WebSource.CanonicalURL {
				t.Fatal("web provenance did not survive import")
			}
			if m.Type != "html" || m.AdmissionStatus != AdmissionAdmitted {
				t.Fatalf("admissible web record imported as %s/%s", m.Type, m.AdmissionStatus)
			}
		}
	}
	// Two versions of one source are two distinct manifest ids.
	ids := map[string]bool{}
	for _, m := range manifest.Sources {
		if ids[m.ID] {
			t.Fatalf("duplicate manifest id %s", m.ID)
		}
		ids[m.ID] = true
	}
}

func TestSnapshot_ImportRecordsAnInadmissibleWebPageAsExcluded(t *testing.T) {
	// A web record whose provenance says "undecided" is imported, not dropped —
	// as excluded, with the reason, exactly as #610 prescribes.
	env, records := sealed(t)
	records[2].WebSource.RobotsDecision = WebDecisionUndecided
	// Re-seal: the record itself is still valid (undecided is recordable).
	env, err := SealExternalSnapshot(env.SnapshotID, env.Producer, "", env.GeneratedAt, "", records)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ImportSnapshotToManifest(env, records, SnapshotImportOptions{Domain: "d", Owner: "o", License: "l", Confidentiality: "internal"})
	var web *ManifestSource
	for i := range manifest.Sources {
		if manifest.Sources[i].WebSource != nil {
			web = &manifest.Sources[i]
		}
	}
	if web == nil {
		t.Fatal("web record dropped")
	}
	if web.AdmissionStatus != AdmissionExcluded || !strings.Contains(web.ExclusionReason, "excluded_by_policy") {
		t.Fatalf("undecided page should be excluded with a reason, got %s / %q", web.AdmissionStatus, web.ExclusionReason)
	}
	if err := web.Validate(); err != nil {
		t.Fatalf("excluded import must still validate: %v", err)
	}
}

// --- the committed fixture, sealed by the real CLI ---------------------------

func TestSnapshot_CommittedFixtureVerifiesAndOneByteBreaksIt(t *testing.T) {
	// The fixture under testdata/external-snapshot was sealed by
	// `nomos corpus snapshot seal`, not written by hand: its root is whatever the
	// engine computed. This pins that the committed files still agree with the
	// engine, and that the smallest possible edit is caught.
	dir := filepath.Join("testdata", "external-snapshot")
	env, records, err := LoadExternalSnapshot(filepath.Join(dir, "snapshot.json"), "")
	if err != nil {
		t.Fatalf("fixture unreadable: %v", err)
	}
	if _, err := VerifyExternalSnapshot(env, records); err != nil {
		t.Fatalf("committed fixture no longer verifies — regenerate it with the CLI or explain why: %v", err)
	}
	if env.SourceCount != 2 || env.VersionCount != 3 {
		t.Fatalf("fixture shape drifted: %d/%d", env.SourceCount, env.VersionCount)
	}
	// One byte of one record.
	raw, err := os.ReadFile(filepath.Join(dir, "sources.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(raw), "2222222222222222222222222222222222222222222222222222222222222222",
		"2222222222222222222222222222222222222222222222222222222222222223", 1)
	if mutated == string(raw) {
		t.Fatal("mutation did not apply; fixture changed?")
	}
	recs, err := ReadSnapshotRecords(strings.NewReader(mutated))
	if err != nil {
		t.Fatal(err)
	}
	_, err = VerifyExternalSnapshot(env, recs)
	if err == nil || !strings.Contains(err.Error(), ErrCodeSnapshotRootMismatch) {
		t.Fatalf("one changed byte should break the root: %v", err)
	}
}

func TestSnapshot_VerifyNamesADuplicateVersionEvenWithForgedCounts(t *testing.T) {
	// A duplicated record would also break the root, so the duplicate rule in
	// Verify is defence in depth for the DIAGNOSIS: "appears twice" tells a
	// reader more than "root mismatch". Counts are forged so only that rule — or
	// the root — can catch it; the rule must come first and name it.
	env, records := sealed(t)
	dup := append(append([]SnapshotRecord(nil), records...), records[0])
	forged := env
	forged.VersionCount = len(dup)
	_, err := VerifyExternalSnapshot(forged, dup)
	if err == nil {
		t.Fatal("a duplicated record verified")
	}
	if !strings.Contains(err.Error(), ErrCodeSnapshotDuplicate) {
		t.Fatalf("expected %s to be named, got %v", ErrCodeSnapshotDuplicate, err)
	}
}
