package pointintime

import (
	"fmt"

	"github.com/RBOKproject/Nomos/cli/internal/docload"
)

// AtomSetSchema is the temporal atom set contract version (specs/point-in-time.cue).
const AtomSetSchema = "0.1.0"

// AtomSet is a temporal atom set document (#TemporalAtomSet).
type AtomSet struct {
	SchemaVersion string           `json:"schema_version"`
	Atoms         []Atom           `json:"atoms"`
	Events        []map[string]any `json:"events"`
}

// LoadAtomSet is the engine's loader for a temporal atom set, YAML or JSON.
// `nomos pointintime resolve` and the contract registry read through it.
func LoadAtomSet(path string) (AtomSet, error) {
	var doc AtomSet
	if err := docload.Load(path, &doc); err != nil {
		return AtomSet{}, err
	}
	if doc.SchemaVersion != "" && doc.SchemaVersion != AtomSetSchema {
		return AtomSet{}, fmt.Errorf("%s: schema_version %q, this engine reads %s", path, doc.SchemaVersion, AtomSetSchema)
	}
	if len(doc.Atoms) == 0 {
		return AtomSet{}, fmt.Errorf("%s: no atoms", path)
	}
	return doc, nil
}
