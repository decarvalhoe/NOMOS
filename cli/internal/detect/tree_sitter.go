//go:build cgo

package detect

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	tsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

const treeSitterParseTimeout = 250 * time.Millisecond

type treeSitterParseResult struct {
	Language string
	RootType string
	HasError bool
}

type treeSitterGrammar struct {
	name     string
	language *sitter.Language
	parser   *sitter.Parser
}

type treeSitterRegistry struct {
	grammars map[string]*treeSitterGrammar
}

type missingGrammarError struct {
	language   string
	registered []string
}

func (e *missingGrammarError) Error() string {
	return fmt.Sprintf(
		"tree-sitter grammar missing for language %s; registered grammars: %s",
		e.language,
		strings.Join(e.registered, ", "),
	)
}

func newTreeSitterRegistry() *treeSitterRegistry {
	registry := &treeSitterRegistry{grammars: map[string]*treeSitterGrammar{}}
	registry.register("Go", golang.GetLanguage())
	registry.register("Java", java.GetLanguage())
	registry.register("JavaScript", javascript.GetLanguage())
	registry.register("Python", python.GetLanguage())
	registry.register("TSX", tsx.GetLanguage())
	registry.register("TypeScript", typescript.GetLanguage())
	return registry
}

func treeSitterSupported() bool {
	return true
}

func (r *treeSitterRegistry) register(name string, language *sitter.Language) {
	parser := sitter.NewParser()
	parser.SetLanguage(language)
	r.grammars[name] = &treeSitterGrammar{
		name:     name,
		language: language,
		parser:   parser,
	}
}

func (r *treeSitterRegistry) parse(
	rel string,
	language string,
	content []byte,
) (treeSitterParseResult, error) {
	grammar, ok := r.grammars[language]
	if !ok {
		return treeSitterParseResult{}, &missingGrammarError{
			language:   language,
			registered: r.registeredGrammarNames(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), treeSitterParseTimeout)
	defer cancel()

	tree, err := grammar.parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return treeSitterParseResult{}, fmt.Errorf(
			"tree-sitter parse failed for %s as %s: %w",
			rel,
			language,
			err,
		)
	}
	if tree == nil {
		return treeSitterParseResult{}, fmt.Errorf(
			"tree-sitter parse failed for %s as %s: empty syntax tree",
			rel,
			language,
		)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return treeSitterParseResult{}, fmt.Errorf(
			"tree-sitter parse failed for %s as %s: empty root node",
			rel,
			language,
		)
	}

	return treeSitterParseResult{
		Language: grammar.name,
		RootType: root.Type(),
		HasError: root.HasError(),
	}, nil
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
