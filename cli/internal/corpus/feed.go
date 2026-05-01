package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

const FeedFormat = "nomos.corpus-feed.v1"

// FeedUnit is a unit entry in the consumer feed.
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
}

// FeedContractRef is a simplified contract reference for consumers.
type FeedContractRef struct {
	Path     string `json:"path"`
	ObjectID string `json:"object_id"`
	Status   string `json:"status"`
}

// FeedSource is a source snapshot entry in the consumer feed.
type FeedSource struct {
	ID              string `json:"id"`
	Path            string `json:"path"`
	Domain          string `json:"domain"`
	Type            string `json:"type"`
	Owner           string `json:"owner"`
	Confidentiality string `json:"confidentiality"`
	Hash            string `json:"hash"`
	Status          string `json:"status"`
}

// Feed is the top-level consumer feed artifact.
type Feed struct {
	Format      string       `json:"format"`
	GeneratedAt string       `json:"generated_at"`
	ContentHash string       `json:"content_hash"`
	UnitCount   int          `json:"unit_count"`
	SourceCount int          `json:"source_count"`
	Units       []FeedUnit   `json:"units"`
	Sources     []FeedSource `json:"sources"`
}

// FeedInput provides the raw data for feed generation.
type FeedInput struct {
	MatrixYAML   []byte
	ManifestYAML []byte
	GeneratedAt  time.Time
}

// matrixFile mirrors the canonical-matrix YAML for parsing.
type matrixFile struct {
	SchemaVersion string       `yaml:"schema_version"`
	Units         []matrixUnit `yaml:"units"`
}

type matrixUnit struct {
	UnitID        string            `yaml:"unit_id"`
	UnitType      string            `yaml:"unit_type"`
	Name          string            `yaml:"name"`
	Domain        string            `yaml:"domain"`
	Criticality   string            `yaml:"criticality"`
	Status        string            `yaml:"status"`
	BusinessRule  string            `yaml:"business_rule"`
	SourceRefs    []matrixSourceRef `yaml:"source_refs"`
	TestRefs      []string          `yaml:"test_refs,omitempty"`
	Gaps          []string          `yaml:"gaps,omitempty"`
	CanonicalContract *matrixContract `yaml:"canonical_contract,omitempty"`
}

type matrixSourceRef struct {
	SourceID string `yaml:"source_id"`
}

type matrixContract struct {
	Path     string `yaml:"path"`
	ObjectID string `yaml:"object_id"`
	Status   string `yaml:"status"`
}

// GenerateFeed produces a consumer feed from a canonical matrix and source manifest.
func GenerateFeed(input FeedInput) (Feed, error) {
	var matrix matrixFile
	if err := yaml.Unmarshal(input.MatrixYAML, &matrix); err != nil {
		return Feed{}, fmt.Errorf("parse matrix: %w", err)
	}

	var manifest SidecarManifest
	if err := yaml.Unmarshal(input.ManifestYAML, &manifest); err != nil {
		return Feed{}, fmt.Errorf("parse manifest: %w", err)
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

	sources := make([]FeedSource, 0, len(manifest.Sources))
	for _, s := range manifest.Sources {
		sources = append(sources, FeedSource{
			ID:              s.ID,
			Path:            s.Path,
			Domain:          s.Domain,
			Type:            s.Type,
			Owner:           s.Owner,
			Confidentiality: s.Confidentiality,
			Hash:            s.Hash,
			Status:          s.Status,
		})
	}

	feed := Feed{
		Format:      FeedFormat,
		GeneratedAt: ts.Format(time.RFC3339),
		UnitCount:   len(units),
		SourceCount: len(sources),
		Units:       units,
		Sources:     sources,
	}

	feed.ContentHash = computeFeedHash(feed)

	return feed, nil
}

// MarshalFeed serialises a feed to indented JSON.
func MarshalFeed(feed Feed) ([]byte, error) {
	return json.MarshalIndent(feed, "", "  ")
}

func computeFeedHash(feed Feed) string {
	// Hash over units and sources, excluding the content_hash itself.
	tmp := feed
	tmp.ContentHash = ""
	data, _ := json.Marshal(tmp)
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
