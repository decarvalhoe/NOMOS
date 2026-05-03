//go:build !cgo

package detect

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type treeSitterParseResult struct {
	Language string
	RootType string
	HasError bool
}

type treeSitterRegistry struct {
	grammars map[string]bool
}

type missingGrammarError struct {
	language   string
	registered []string
}

func (e *missingGrammarError) Error() string {
	return fmt.Sprintf(
		"tree-sitter disabled for this build; language %s cannot be parsed; registered grammars: %s",
		e.language,
		strings.Join(e.registered, ", "),
	)
}

func newTreeSitterRegistry() *treeSitterRegistry {
	return &treeSitterRegistry{grammars: map[string]bool{
		"Go": true, "Java": true, "JavaScript": true, "Python": true, "TSX": true, "TypeScript": true,
	}}
}

func (r *treeSitterRegistry) parse(rel string, language string, _ []byte) (treeSitterParseResult, error) {
	return treeSitterParseResult{}, &missingGrammarError{
		language:   language,
		registered: r.registeredGrammarNames(),
	}
}

func (r *treeSitterRegistry) registeredGrammarNames() []string {
	names := make([]string, 0, len(r.grammars))
	for name := range r.grammars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func treeSitterLanguageForPath(rel string) string {
	ext := strings.ToLower(path.Ext(rel))
	switch ext {
	case ".go":
		return "Go"
	case ".java":
		return "Java"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JavaScript"
	case ".py":
		return "Python"
	case ".ts":
		return "TypeScript"
	case ".tsx":
		return "TSX"
	case ".cs":
		return "C#"
	default:
		return ""
	}
}

func treeSitterSupported() bool {
	return false
}
