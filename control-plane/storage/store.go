package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Well-known errors.
var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidID      = errors.New("invalid identifier")
	ErrRetentionLimit = errors.New("retention limit exceeded")
)

// RetentionPolicy defines how long reports and attestations are kept.
type RetentionPolicy struct {
	MaxAge   time.Duration `json:"max_age"`
	MaxCount int           `json:"max_count"`
}

// DefaultRetentionPolicy returns a 90-day / 500-entry retention policy.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MaxAge:   90 * 24 * time.Hour,
		MaxCount: 500,
	}
}

// VersionRef ties evidence to a specific code version.
type VersionRef struct {
	Commit     string `json:"commit"`
	Branch     string `json:"branch,omitempty"`
	Repository string `json:"repository,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
}

// StoredReport is the envelope persisted for each execution report.
type StoredReport struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"project_id"`
	RunID      string          `json:"run_id"`
	Version    VersionRef      `json:"version"`
	Status     string          `json:"status"`
	StoredAt   time.Time       `json:"stored_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
	ReportData json.RawMessage `json:"report_data"`
}

// StoredAttestation is the envelope persisted for each attestation.
type StoredAttestation struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"project_id"`
	RunID      string          `json:"run_id"`
	Type       string          `json:"type"`
	Version    VersionRef      `json:"version"`
	StoredAt   time.Time       `json:"stored_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
	Payload    json.RawMessage `json:"payload"`
}

// ExecutionView is the complete view for a single run, combining
// report and attestations.
type ExecutionView struct {
	RunID        string              `json:"run_id"`
	ProjectID    string              `json:"project_id"`
	Version      VersionRef          `json:"version"`
	Report       *StoredReport       `json:"report,omitempty"`
	Attestations []StoredAttestation `json:"attestations"`
}

// ListFilter constrains which entries are returned by list operations.
type ListFilter struct {
	ProjectID string
	Commit    string
	Since     time.Time
	Limit     int
}

// Store is the storage interface for reports and attestations.
type Store interface {
	StoreReport(report StoredReport) error
	GetReport(id string) (StoredReport, error)
	ListReports(filter ListFilter) ([]StoredReport, error)

	StoreAttestation(att StoredAttestation) error
	GetAttestation(id string) (StoredAttestation, error)
	ListAttestations(filter ListFilter) ([]StoredAttestation, error)

	GetExecution(runID string) (ExecutionView, error)

	ApplyRetention(policy RetentionPolicy, now time.Time) (removed int, err error)
}

// FSStore implements Store using the local filesystem.
// Layout:
//
//	<root>/reports/<id>.json
//	<root>/attestations/<id>.json
type FSStore struct {
	root string
}

// NewFSStore creates a filesystem-backed store at the given root directory.
func NewFSStore(root string) (*FSStore, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"reports", "attestations"} {
		if err := os.MkdirAll(filepath.Join(absRoot, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return &FSStore{root: absRoot}, nil
}

func (s *FSStore) StoreReport(report StoredReport) error {
	if err := validateID(report.ID); err != nil {
		return err
	}
	return writeJSON(s.reportPath(report.ID), report)
}

func (s *FSStore) GetReport(id string) (StoredReport, error) {
	var r StoredReport
	if err := readJSON(s.reportPath(id), &r); err != nil {
		return r, err
	}
	return r, nil
}

func (s *FSStore) ListReports(filter ListFilter) ([]StoredReport, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "reports"))
	if err != nil {
		return nil, err
	}

	var results []StoredReport
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var r StoredReport
		if err := readJSON(filepath.Join(s.root, "reports", e.Name()), &r); err != nil {
			continue
		}
		if matchesFilter(r.ProjectID, r.Version.Commit, r.StoredAt, filter) {
			results = append(results, r)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StoredAt.After(results[j].StoredAt)
	})

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return results, nil
}

func (s *FSStore) StoreAttestation(att StoredAttestation) error {
	if err := validateID(att.ID); err != nil {
		return err
	}
	return writeJSON(s.attestationPath(att.ID), att)
}

func (s *FSStore) GetAttestation(id string) (StoredAttestation, error) {
	var a StoredAttestation
	if err := readJSON(s.attestationPath(id), &a); err != nil {
		return a, err
	}
	return a, nil
}

func (s *FSStore) ListAttestations(filter ListFilter) ([]StoredAttestation, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "attestations"))
	if err != nil {
		return nil, err
	}

	var results []StoredAttestation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var a StoredAttestation
		if err := readJSON(filepath.Join(s.root, "attestations", e.Name()), &a); err != nil {
			continue
		}
		if matchesFilter(a.ProjectID, a.Version.Commit, a.StoredAt, filter) {
			results = append(results, a)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StoredAt.After(results[j].StoredAt)
	})

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return results, nil
}

// GetExecution returns the complete view of a run: its report and all
// associated attestations.
func (s *FSStore) GetExecution(runID string) (ExecutionView, error) {
	view := ExecutionView{RunID: runID}

	reports, err := s.ListReports(ListFilter{})
	if err != nil {
		return view, err
	}
	for i := range reports {
		if reports[i].RunID == runID {
			view.Report = &reports[i]
			view.ProjectID = reports[i].ProjectID
			view.Version = reports[i].Version
			break
		}
	}

	atts, err := s.ListAttestations(ListFilter{})
	if err != nil {
		return view, err
	}
	for _, a := range atts {
		if a.RunID == runID {
			view.Attestations = append(view.Attestations, a)
		}
	}

	if view.Report == nil && len(view.Attestations) == 0 {
		return view, fmt.Errorf("run %q: %w", runID, ErrNotFound)
	}

	return view, nil
}

// ApplyRetention removes reports and attestations that exceed the
// retention policy (by age or count).
func (s *FSStore) ApplyRetention(policy RetentionPolicy, now time.Time) (int, error) {
	removed := 0

	r, err := s.purgeDir(filepath.Join(s.root, "reports"), policy, now)
	if err != nil {
		return removed, err
	}
	removed += r

	r, err = s.purgeDir(filepath.Join(s.root, "attestations"), policy, now)
	if err != nil {
		return removed, err
	}
	removed += r

	return removed, nil
}

func (s *FSStore) purgeDir(dir string, policy RetentionPolicy, now time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	type fileEntry struct {
		name     string
		storedAt time.Time
	}

	var files []fileEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var envelope struct {
			StoredAt time.Time `json:"stored_at"`
		}
		if err := readJSON(filepath.Join(dir, e.Name()), &envelope); err != nil {
			continue
		}
		files = append(files, fileEntry{name: e.Name(), storedAt: envelope.StoredAt})
	}

	// Sort oldest first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].storedAt.Before(files[j].storedAt)
	})

	removed := 0

	// Remove by age.
	if policy.MaxAge > 0 {
		cutoff := now.Add(-policy.MaxAge)
		for _, f := range files {
			if f.storedAt.Before(cutoff) {
				if err := os.Remove(filepath.Join(dir, f.name)); err == nil {
					removed++
				}
			}
		}
	}

	// Re-read after age purge for count enforcement.
	remaining, err := os.ReadDir(dir)
	if err != nil {
		return removed, err
	}
	var jsonCount int
	for _, e := range remaining {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonCount++
		}
	}

	if policy.MaxCount > 0 && jsonCount > policy.MaxCount {
		// Re-sort to remove oldest first.
		var current []fileEntry
		for _, e := range remaining {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			var envelope struct {
				StoredAt time.Time `json:"stored_at"`
			}
			if err := readJSON(filepath.Join(dir, e.Name()), &envelope); err != nil {
				continue
			}
			current = append(current, fileEntry{name: e.Name(), storedAt: envelope.StoredAt})
		}
		sort.Slice(current, func(i, j int) bool {
			return current[i].storedAt.Before(current[j].storedAt)
		})
		excess := len(current) - policy.MaxCount
		for i := 0; i < excess; i++ {
			if err := os.Remove(filepath.Join(dir, current[i].name)); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

func (s *FSStore) reportPath(id string) string {
	return filepath.Join(s.root, "reports", id+".json")
}

func (s *FSStore) attestationPath(id string) string {
	return filepath.Join(s.root, "attestations", id+".json")
}

// --- helpers ---

func validateID(id string) error {
	if id == "" || strings.ContainsAny(id, "/\\..") {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return nil
}

func matchesFilter(projectID, commit string, storedAt time.Time, f ListFilter) bool {
	if f.ProjectID != "" && f.ProjectID != projectID {
		return false
	}
	if f.Commit != "" && f.Commit != commit {
		return false
	}
	if !f.Since.IsZero() && storedAt.Before(f.Since) {
		return false
	}
	return true
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", filepath.Base(path), ErrNotFound)
		}
		return err
	}
	return json.Unmarshal(data, v)
}
