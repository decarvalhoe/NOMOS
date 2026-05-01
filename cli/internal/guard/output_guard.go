package guard

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CheckOutputNotInSource returns an error if outPath resolves to a location
// inside rootPath. This prevents generated files from polluting the source
// corpus that Nomos inspects.
func CheckOutputNotInSource(rootPath string, outPath string) error {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("resolve root path: %w", err)
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	// Normalize with trailing separator so /root does not match /rootExtra.
	normalRoot := filepath.Clean(absRoot) + string(filepath.Separator)
	normalOut := filepath.Clean(absOut)

	if normalOut == filepath.Clean(absRoot) || strings.HasPrefix(normalOut+string(filepath.Separator), normalRoot) {
		return &OutputInsideSourceError{
			Root:   filepath.Clean(absRoot),
			Output: normalOut,
		}
	}

	return nil
}

// OutputInsideSourceError is returned when --out targets a path inside --root.
type OutputInsideSourceError struct {
	Root   string
	Output string
}

func (e *OutputInsideSourceError) Error() string {
	return fmt.Sprintf("output path %q is inside source root %q; writing generated files into the inspected corpus is not allowed", e.Output, e.Root)
}
