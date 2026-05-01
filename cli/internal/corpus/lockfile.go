package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

const LockfileVersion = "1"

var (
	ErrSnapshotNotApproved = errors.New("snapshot not approved in lockfile")
	ErrLockfileCorrupt     = errors.New("lockfile is corrupt or unreadable")
	ErrDuplicateEntry      = errors.New("duplicate lockfile entry")
)

// ReviewStatus indicates the approval state of a corpus snapshot.
type ReviewStatus string

const (
	ReviewApproved ReviewStatus = "approved"
	ReviewPending  ReviewStatus = "pending"
	ReviewRejected ReviewStatus = "rejected"
)

// LockEntry records one accepted corpus snapshot.
type LockEntry struct {
	Path       string       `json:"path"`
	Hash       string       `json:"hash"`
	ReviewedAt time.Time    `json:"reviewed_at"`
	ReviewedBy string       `json:"reviewed_by"`
	Status     ReviewStatus `json:"status"`
	Comment    string       `json:"comment,omitempty"`
}

// Lockfile holds the set of accepted corpus snapshots.
type Lockfile struct {
	Version   string      `json:"version"`
	UpdatedAt time.Time   `json:"updated_at"`
	Entries   []LockEntry `json:"entries"`
}

// NewLockfile creates an empty lockfile.
func NewLockfile() *Lockfile {
	return &Lockfile{
		Version:   LockfileVersion,
		UpdatedAt: time.Now().UTC(),
		Entries:   []LockEntry{},
	}
}

// ReadLockfile loads a lockfile from disk.
func ReadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLockfileCorrupt, err)
	}
	if lf.Version == "" {
		return nil, fmt.Errorf("%w: missing version field", ErrLockfileCorrupt)
	}
	return &lf, nil
}

// Write persists the lockfile to disk.
func (lf *Lockfile) Write(path string) error {
	lf.UpdatedAt = time.Now().UTC()
	sort.Slice(lf.Entries, func(i, j int) bool {
		return lf.Entries[i].Path < lf.Entries[j].Path
	})
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Add registers a new approved snapshot in the lockfile.
func (lf *Lockfile) Add(path, hash, reviewer, comment string) error {
	for _, entry := range lf.Entries {
		if entry.Path == path && entry.Hash == hash {
			return fmt.Errorf("%w: %s@%s", ErrDuplicateEntry, path, hash)
		}
	}
	lf.Entries = append(lf.Entries, LockEntry{
		Path:       path,
		Hash:       hash,
		ReviewedAt: time.Now().UTC(),
		ReviewedBy: reviewer,
		Status:     ReviewApproved,
		Comment:    comment,
	})
	return nil
}

// Approve updates an existing entry's status to approved.
func (lf *Lockfile) Approve(path, hash, reviewer string) error {
	for i, entry := range lf.Entries {
		if entry.Path == path && entry.Hash == hash {
			lf.Entries[i].Status = ReviewApproved
			lf.Entries[i].ReviewedBy = reviewer
			lf.Entries[i].ReviewedAt = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("%w: %s@%s", ErrSnapshotNotApproved, path, hash)
}

// Reject marks an entry as rejected.
func (lf *Lockfile) Reject(path, hash, reviewer, reason string) error {
	for i, entry := range lf.Entries {
		if entry.Path == path && entry.Hash == hash {
			lf.Entries[i].Status = ReviewRejected
			lf.Entries[i].ReviewedBy = reviewer
			lf.Entries[i].ReviewedAt = time.Now().UTC()
			lf.Entries[i].Comment = reason
			return nil
		}
	}
	return fmt.Errorf("%w: %s@%s", ErrSnapshotNotApproved, path, hash)
}

// IsApproved checks whether a specific path+hash combination is approved.
func (lf *Lockfile) IsApproved(path, hash string) bool {
	for _, entry := range lf.Entries {
		if entry.Path == path && entry.Hash == hash && entry.Status == ReviewApproved {
			return true
		}
	}
	return false
}

// Verify checks that all entries in a snapshot are approved in the lockfile.
// Returns a list of unapproved file entries.
func (lf *Lockfile) Verify(snapshot Snapshot) []SourceEntry {
	var unapproved []SourceEntry
	for _, fe := range snapshot.Sources {
		if !lf.IsApproved(fe.Path, fe.Hash) {
			unapproved = append(unapproved, fe)
		}
	}
	return unapproved
}

// Guard returns an error if any file in the snapshot is not approved.
func (lf *Lockfile) Guard(snapshot Snapshot) error {
	unapproved := lf.Verify(snapshot)
	if len(unapproved) == 0 {
		return nil
	}
	paths := make([]string, len(unapproved))
	for i, fe := range unapproved {
		paths[i] = fe.Path
	}
	return fmt.Errorf("%w: %d file(s) not approved: %v", ErrSnapshotNotApproved, len(unapproved), paths)
}

// ApprovedEntries returns only entries with approved status.
func (lf *Lockfile) ApprovedEntries() []LockEntry {
	var approved []LockEntry
	for _, entry := range lf.Entries {
		if entry.Status == ReviewApproved {
			approved = append(approved, entry)
		}
	}
	return approved
}
