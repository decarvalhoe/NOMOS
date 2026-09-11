// Package domainpack holds the domain-pack manifest contract
// (specs/domain-pack.cue) and its loader: a strict parse (unknown fields are
// mechanics smuggled around the contract) and the exact schema version.
package domainpack

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Schema is the domain-pack contract version this engine reads.
const Schema = "nomos-domain-pack-v1"

// Manifest is a domain pack manifest (pack.yaml).
type Manifest struct {
	SchemaVersion string `yaml:"schema_version"`
	PackID        string `yaml:"pack_id"`
	DomainProfile string `yaml:"domain_profile"`
	ProfileRef    string `yaml:"profile_ref"`
	ClaimBoundary string `yaml:"claim_boundary"`
	Vocabularies  struct {
		File string   `yaml:"file"`
		Axes []string `yaml:"axes"`
	} `yaml:"vocabularies"`
	Ontology struct {
		File string `yaml:"file"`
	} `yaml:"ontology"`
	SourceRegister struct {
		File     string `yaml:"file"`
		Contract string `yaml:"contract"`
	} `yaml:"source_register"`
	LensPresets []struct {
		ID   string `yaml:"id"`
		File string `yaml:"file"`
	} `yaml:"lens_presets"`
	GoldenCorpus struct {
		Root      string   `yaml:"root"`
		Documents []string `yaml:"documents"`
	} `yaml:"golden_corpus"`
	Scorecard []struct {
		Area   string `yaml:"area"`
		Status string `yaml:"status"`
		Note   string `yaml:"note"`
	} `yaml:"scorecard"`
}

// LoadManifest is the engine's loader for a pack manifest: strict fields,
// exact schema_version. `nomos pack validate` and the contract registry read
// through it.
func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("strict parse %s: %w", path, err)
	}
	if m.SchemaVersion != Schema {
		return Manifest{}, fmt.Errorf("%s: schema_version %q is not %s", path, m.SchemaVersion, Schema)
	}
	return m, nil
}
