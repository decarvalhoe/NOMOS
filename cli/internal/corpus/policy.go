package corpus

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy defines allow/ignore glob rules for corpus file filtering.
type Policy struct {
	Allow  []string `yaml:"allow" json:"allow"`
	Ignore []string `yaml:"ignore" json:"ignore"`
}

// PolicyConfig is the on-disk representation of .nomos-corpus.yaml.
type PolicyConfig struct {
	SchemaVersion string `yaml:"schema_version"`
	Corpus        Policy `yaml:"corpus"`
}

// DefaultPolicy returns the built-in policy that ignores common
// non-source directories and binary files.
func DefaultPolicy() Policy {
	return Policy{
		Allow: []string{"**/*"},
		Ignore: []string{
			".git/**",
			".hg/**",
			".svn/**",
			"node_modules/**",
			"vendor/**",
			".venv/**",
			"venv/**",
			"__pycache__/**",
			"dist/**",
			"build/**",
			".next/**",
			"target/**",
			"coverage/**",
			".terraform/**",
			".tools/**",
			".cache/**",
			"**/*.exe",
			"**/*.dll",
			"**/*.so",
			"**/*.dylib",
			"**/*.bin",
			"**/*.o",
			"**/*.a",
			"**/*.class",
			"**/*.jar",
			"**/*.war",
			"**/*.pyc",
			"**/*.pyo",
			"**/*.wasm",
		},
	}
}

// LoadPolicy reads a .nomos-corpus.yaml from the given directory.
// If the file doesn't exist, returns the default policy.
func LoadPolicy(dir string) (Policy, error) {
	configPath := filepath.Join(dir, ".nomos-corpus.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return Policy{}, fmt.Errorf("reading corpus policy: %w", err)
	}
	return ParsePolicy(data)
}

// ParsePolicy parses policy YAML bytes into a Policy.
func ParsePolicy(data []byte) (Policy, error) {
	var config PolicyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Policy{}, fmt.Errorf("parsing corpus policy: %w", err)
	}
	policy := config.Corpus
	if len(policy.Allow) == 0 {
		policy.Allow = []string{"**/*"}
	}
	return policy, nil
}

// Merge combines a base policy with overrides. Override ignore patterns
// are appended; override allow patterns replace if non-empty.
func (p Policy) Merge(override Policy) Policy {
	merged := Policy{
		Allow:  p.Allow,
		Ignore: append(append([]string{}, p.Ignore...), override.Ignore...),
	}
	if len(override.Allow) > 0 {
		merged.Allow = override.Allow
	}
	return merged
}

// Match returns true if the given relative path is allowed by the policy
// (matches at least one allow pattern and no ignore pattern).
func (p Policy) Match(rel string) bool {
	rel = filepath.ToSlash(rel)
	if matchesAny(rel, p.Ignore) {
		return false
	}
	return matchesAny(rel, p.Allow)
}

// Filter returns only the paths that pass the policy.
func (p Policy) Filter(paths []string) []string {
	var result []string
	for _, rel := range paths {
		if p.Match(rel) {
			result = append(result, rel)
		}
	}
	return result
}

// matchesAny checks if rel matches any of the glob patterns.
func matchesAny(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, rel) {
			return true
		}
	}
	return false
}

// globMatch supports ** for recursive directory matching and delegates
// single-segment matching to path.Match.
func globMatch(pattern, name string) bool {
	// Fast path: no ** means standard match.
	if !strings.Contains(pattern, "**") {
		matched, _ := path.Match(pattern, name)
		return matched
	}

	parts := strings.Split(pattern, "**")
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := strings.TrimPrefix(parts[1], "/")

		// Pattern like "dir/**" — matches anything under dir/
		if suffix == "" || suffix == "*" {
			if prefix == "" {
				return true
			}
			cleanPrefix := strings.TrimSuffix(prefix, "/")
			return strings.HasPrefix(name, cleanPrefix+"/") || name == cleanPrefix
		}

		// Pattern like "**/*.ext" — matches suffix anywhere in path.
		if prefix == "" {
			// Match suffix against the basename or any tail segment.
			segments := strings.Split(name, "/")
			for i := range segments {
				tail := strings.Join(segments[i:], "/")
				matched, _ := path.Match(suffix, tail)
				if matched {
					return true
				}
				// Also try matching just the base.
				if i == len(segments)-1 {
					matched, _ = path.Match(suffix, segments[i])
					if matched {
						return true
					}
				}
			}
			return false
		}

		// Pattern like "dir/**/file" — prefix must match start, suffix the rest.
		cleanPrefix := strings.TrimSuffix(prefix, "/")
		if !strings.HasPrefix(name, cleanPrefix+"/") {
			return false
		}
		remainder := strings.TrimPrefix(name, cleanPrefix+"/")
		// Suffix can match at any depth.
		segments := strings.Split(remainder, "/")
		for i := range segments {
			tail := strings.Join(segments[i:], "/")
			matched, _ := path.Match(suffix, tail)
			if matched {
				return true
			}
		}
		return false
	}

	// Fallback for multiple ** segments: simple prefix/suffix heuristic.
	matched, _ := path.Match(pattern, name)
	return matched
}
