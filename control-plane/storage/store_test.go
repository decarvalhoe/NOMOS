package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *FSStore {
	t.Helper()
	s, err := NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return s
}

var baseTime = time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)

func makeReport(id, projectID, runID, commit, status string, storedAt time.Time, retentionDays int) StoredReport {
	return StoredReport{
		ID:        id,
		ProjectID: projectID,
		RunID:     runID,
		Version:   VersionRef{Commit: commit, Branch: "main"},
		Status:    status,
		StoredAt:  storedAt,
		ExpiresAt: storedAt.Add(time.Duration(retentionDays) * 24 * time.Hour),
		ReportData: json.RawMessage(`{
			"schema_version": "0.1.0",
			"report_type": "nomos-report"
		}`),
	}
}

func makeAttestation(id, projectID, runID, commit, attType string, storedAt time.Time) StoredAttestation {
	return StoredAttestation{
		ID:        id,
		ProjectID: projectID,
		RunID:     runID,
		Type:      attType,
		Version:   VersionRef{Commit: commit, Branch: "main"},
		StoredAt:  storedAt,
		ExpiresAt: storedAt.Add(90 * 24 * time.Hour),
		Payload:   json.RawMessage(`{"type": "` + attType + `"}`),
	}
}

func TestStoreAndGetReport(t *testing.T) {
	s := newTestStore(t)
	r := makeReport("RPT-001", "proj-a", "run-1", "abc123", "pass", baseTime, 90)

	if err := s.StoreReport(r); err != nil {
		t.Fatalf("store report: %v", err)
	}

	got, err := s.GetReport("RPT-001")
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if got.ID != "RPT-001" {
		t.Fatalf("expected ID RPT-001, got %q", got.ID)
	}
	if got.ProjectID != "proj-a" {
		t.Fatalf("expected project proj-a, got %q", got.ProjectID)
	}
	if got.Version.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", got.Version.Commit)
	}
	if got.Status != "pass" {
		t.Fatalf("expected status pass, got %q", got.Status)
	}
}

func TestGetReportNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetReport("NONEXISTENT")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreReportInvalidID(t *testing.T) {
	s := newTestStore(t)
	r := makeReport("", "proj-a", "run-1", "abc", "pass", baseTime, 90)

	err := s.StoreReport(r)
	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestStoreAndGetAttestation(t *testing.T) {
	s := newTestStore(t)
	a := makeAttestation("ATT-001", "proj-a", "run-1", "abc123", "in-toto", baseTime)

	if err := s.StoreAttestation(a); err != nil {
		t.Fatalf("store attestation: %v", err)
	}

	got, err := s.GetAttestation("ATT-001")
	if err != nil {
		t.Fatalf("get attestation: %v", err)
	}
	if got.ID != "ATT-001" {
		t.Fatalf("expected ID ATT-001, got %q", got.ID)
	}
	if got.Type != "in-toto" {
		t.Fatalf("expected type in-toto, got %q", got.Type)
	}
	if got.Version.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", got.Version.Commit)
	}
}

func TestListReportsFilterByProject(t *testing.T) {
	s := newTestStore(t)
	_ = s.StoreReport(makeReport("RPT-001", "proj-a", "run-1", "aaa", "pass", baseTime, 90))
	_ = s.StoreReport(makeReport("RPT-002", "proj-b", "run-2", "bbb", "fail", baseTime.Add(time.Hour), 90))
	_ = s.StoreReport(makeReport("RPT-003", "proj-a", "run-3", "ccc", "pass", baseTime.Add(2*time.Hour), 90))

	results, err := s.ListReports(ListFilter{ProjectID: "proj-a"})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 reports for proj-a, got %d", len(results))
	}
	// Should be sorted newest first.
	if results[0].ID != "RPT-003" {
		t.Fatalf("expected first result RPT-003, got %q", results[0].ID)
	}
}

func TestListReportsFilterByCommit(t *testing.T) {
	s := newTestStore(t)
	_ = s.StoreReport(makeReport("RPT-001", "proj-a", "run-1", "abc123", "pass", baseTime, 90))
	_ = s.StoreReport(makeReport("RPT-002", "proj-a", "run-2", "def456", "fail", baseTime.Add(time.Hour), 90))

	results, err := s.ListReports(ListFilter{Commit: "abc123"})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 report for commit abc123, got %d", len(results))
	}
	if results[0].Version.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", results[0].Version.Commit)
	}
}

func TestListReportsWithLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("RPT-%03d", i)
		_ = s.StoreReport(makeReport(id, "proj-a", "run-"+id, "commit", "pass", baseTime.Add(time.Duration(i)*time.Hour), 90))
	}

	results, err := s.ListReports(ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(results))
	}
}

func TestGetExecution(t *testing.T) {
	s := newTestStore(t)
	_ = s.StoreReport(makeReport("RPT-001", "proj-a", "run-1", "abc123", "pass", baseTime, 90))
	_ = s.StoreAttestation(makeAttestation("ATT-001", "proj-a", "run-1", "abc123", "in-toto", baseTime))
	_ = s.StoreAttestation(makeAttestation("ATT-002", "proj-a", "run-1", "abc123", "slsa-provenance", baseTime))
	// Different run — should not be included.
	_ = s.StoreAttestation(makeAttestation("ATT-003", "proj-a", "run-2", "def456", "in-toto", baseTime))

	view, err := s.GetExecution("run-1")
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if view.RunID != "run-1" {
		t.Fatalf("expected run-1, got %q", view.RunID)
	}
	if view.Report == nil {
		t.Fatalf("expected report in execution view")
	}
	if view.Report.ID != "RPT-001" {
		t.Fatalf("expected report RPT-001, got %q", view.Report.ID)
	}
	if len(view.Attestations) != 2 {
		t.Fatalf("expected 2 attestations, got %d", len(view.Attestations))
	}
	if view.Version.Commit != "abc123" {
		t.Fatalf("expected version commit abc123, got %q", view.Version.Commit)
	}
}

func TestGetExecutionNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetExecution("nonexistent-run")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestApplyRetentionByAge(t *testing.T) {
	s := newTestStore(t)
	old := baseTime.Add(-100 * 24 * time.Hour)
	recent := baseTime.Add(-10 * 24 * time.Hour)

	_ = s.StoreReport(makeReport("RPT-OLD", "proj-a", "run-old", "old", "pass", old, 90))
	_ = s.StoreReport(makeReport("RPT-NEW", "proj-a", "run-new", "new", "pass", recent, 90))
	_ = s.StoreAttestation(makeAttestation("ATT-OLD", "proj-a", "run-old", "old", "in-toto", old))
	_ = s.StoreAttestation(makeAttestation("ATT-NEW", "proj-a", "run-new", "new", "in-toto", recent))

	policy := RetentionPolicy{MaxAge: 90 * 24 * time.Hour}
	removed, err := s.ApplyRetention(policy, baseTime)
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	// Old should be gone.
	_, err = s.GetReport("RPT-OLD")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected old report to be purged")
	}

	// New should still exist.
	_, err = s.GetReport("RPT-NEW")
	if err != nil {
		t.Fatalf("expected new report to survive retention: %v", err)
	}
}

func TestApplyRetentionByCount(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("RPT-%03d", i)
		_ = s.StoreReport(makeReport(id, "proj-a", "run-"+id, "c", "pass",
			baseTime.Add(time.Duration(i)*time.Hour), 90))
	}

	policy := RetentionPolicy{MaxCount: 3}
	removed, err := s.ApplyRetention(policy, baseTime)
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	remaining, err := s.ListReports(ListFilter{})
	if err != nil {
		t.Fatalf("list after retention: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining, got %d", len(remaining))
	}
}

func TestEvidenceLinkedToVersion(t *testing.T) {
	s := newTestStore(t)
	r := makeReport("RPT-V", "proj-a", "run-v", "sha-version-1", "pass", baseTime, 90)
	r.Version = VersionRef{
		Commit:     "sha-version-1",
		Branch:     "release/2026-04",
		Repository: "https://example.com/proj-a.git",
		Dirty:      false,
	}
	_ = s.StoreReport(r)

	got, err := s.GetReport("RPT-V")
	if err != nil {
		t.Fatalf("get report: %v", err)
	}
	if got.Version.Commit != "sha-version-1" {
		t.Fatalf("expected commit sha-version-1, got %q", got.Version.Commit)
	}
	if got.Version.Branch != "release/2026-04" {
		t.Fatalf("expected branch release/2026-04, got %q", got.Version.Branch)
	}
	if got.Version.Repository != "https://example.com/proj-a.git" {
		t.Fatalf("expected repo url, got %q", got.Version.Repository)
	}

	// Filter by commit version.
	results, err := s.ListReports(ListFilter{Commit: "sha-version-1"})
	if err != nil {
		t.Fatalf("list by commit: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 report for commit, got %d", len(results))
	}
}

func TestDefaultRetentionPolicy(t *testing.T) {
	p := DefaultRetentionPolicy()
	if p.MaxAge != 90*24*time.Hour {
		t.Fatalf("expected 90 days, got %v", p.MaxAge)
	}
	if p.MaxCount != 500 {
		t.Fatalf("expected 500, got %d", p.MaxCount)
	}
}
