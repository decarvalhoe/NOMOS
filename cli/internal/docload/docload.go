// Package docload decodes a YAML or JSON document into a json-tagged engine
// type: YAML is parsed generically, normalised (yaml.v3 map[any]any → map
// [string]any) and re-encoded as JSON, so engine types keep one tag set and a
// contract authored in YAML is read by the same struct as its JSON form.
package docload

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads path and decodes it into dst (a pointer to a json-tagged struct).
func Load(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return Decode(raw, path, dst)
}

// Decode decodes YAML-or-JSON bytes into dst; name is used in errors.
func Decode(raw []byte, name string, dst any) error {
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	bridged, err := json.Marshal(Normalize(generic))
	if err != nil {
		return fmt.Errorf("normalize %s: %w", name, err)
	}
	if err := json.Unmarshal(bridged, dst); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

// Normalize converts yaml.v3 map[any]any trees into json-encodable map[string]any trees.
func Normalize(v any) any {
	switch t := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprintf("%v", k)] = Normalize(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = Normalize(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = Normalize(t[i])
		}
		return t
	default:
		return v
	}
}
