package bundle

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile is the engine's loader for an emitted Canonical Knowledge Bundle:
// it decodes the JSON and refuses a bundle whose schema_version is not the
// one this engine writes. `nomos rag` consumes bundles through it.
// It also returns the raw bytes so callers can hash exactly what was read.
func LoadFile(path string) (Bundle, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Bundle{}, nil, fmt.Errorf("read bundle %s: %w", path, err)
	}
	var b Bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bundle{}, nil, fmt.Errorf("decode bundle %s: %w", path, err)
	}
	if b.SchemaVersion != SchemaVersion {
		return Bundle{}, nil, fmt.Errorf("bundle %s: schema_version %q, this engine reads %s", path, b.SchemaVersion, SchemaVersion)
	}
	return b, raw, nil
}
