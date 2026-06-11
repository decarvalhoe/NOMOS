package fidelity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VRC-33 (#570, C5) — « capability versionnée avec limites déclarées »
// (doc 14 principe 4): registering an adapter without a COMPLETE kit fails
// closed, and every kit reference resolves to real, executable evidence.

// incompleteAdapter is the adversarial fixture: a working parser that ships
// no kit — exactly what C5 forbids.
type incompleteAdapter struct {
	kit AdapterKit
}

func (incompleteAdapter) Name() string         { return "incomplete" }
func (incompleteAdapter) Extensions() []string { return []string{".inc"} }
func (a incompleteAdapter) Kit() AdapterKit    { return a.kit }
func (incompleteAdapter) Parse(_ []byte, _ string) (ParseResult, error) {
	return ParseResult{Format: "incomplete"}, nil
}
func (incompleteAdapter) Validate(_ []byte, _ string) ValidationResult {
	return ValidationResult{Valid: true}
}
func (incompleteAdapter) Spans(_ []byte, _ string) ([]SpanInfo, error) { return nil, nil }

func completeKit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "test boundary",
		ClaimLevel:       "structural-spans",
		UnsupportedKinds: []string{"everything_else"},
		GateFixtures:     []string{"test://cli/internal/fidelity/adapter_kit_test.go#TestRegisterRefusesIncompleteKits"},
	}
}

func TestRegisterRefusesIncompleteKits(t *testing.T) {
	mutations := map[string]func(*AdapterKit){
		"no claim boundary":      func(k *AdapterKit) { k.ClaimBoundary = "   " },
		"no claim level":         func(k *AdapterKit) { k.ClaimLevel = "" },
		"nothing unsupported":    func(k *AdapterKit) { k.UnsupportedKinds = nil },
		"blank unsupported kind": func(k *AdapterKit) { k.UnsupportedKinds = []string{" "} },
		"no gate fixtures":       func(k *AdapterKit) { k.GateFixtures = nil },
		"blank fixture ref":      func(k *AdapterKit) { k.GateFixtures = []string{""} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			kit := completeKit()
			mutate(&kit)
			err := NewRegistry().Register(incompleteAdapter{kit: kit})
			if err == nil {
				t.Fatalf("an adapter with %s entered the registry", name)
			}
			if !strings.Contains(err.Error(), "incomplete") {
				t.Fatalf("the refusal must name the adapter: %v", err)
			}
		})
	}
	// The control: the complete kit registers fine.
	if err := NewRegistry().Register(incompleteAdapter{kit: completeKit()}); err != nil {
		t.Fatalf("the complete kit must register: %v", err)
	}
}

func TestDefaultRegistryKitsAreCompleteAndResolve(t *testing.T) {
	// DefaultRegistry would already panic-free fail registration on an
	// incomplete kit (Register validates) — here we additionally resolve
	// every gate-fixture reference to REAL evidence in the repo.
	repoRoot := filepath.Join("..", "..", "..")
	r := DefaultRegistry()
	if r.Count() == 0 {
		t.Fatal("default registry is empty")
	}
	for _, name := range r.Names() {
		adapter, _ := r.Lookup(name)
		kit := adapter.Kit()
		if err := validateKit(name, kit); err != nil {
			t.Fatalf("default adapter %q ships an incomplete kit: %v", name, err)
		}
		for _, ref := range kit.GateFixtures {
			t.Run(name+"/"+ref, func(t *testing.T) {
				if strings.HasPrefix(ref, "test://") {
					spec := strings.TrimPrefix(ref, "test://")
					parts := strings.SplitN(spec, "#", 2)
					if len(parts) != 2 {
						t.Fatalf("malformed test:// reference: %s", ref)
					}
					path := filepath.Join(repoRoot, filepath.FromSlash(parts[0]))
					raw, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("gate test file missing: %v", err)
					}
					if !strings.Contains(string(raw), "func "+parts[1]+"(") {
						t.Fatalf("gate test %s not found in %s", parts[1], parts[0])
					}
				} else {
					if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(ref))); err != nil {
						t.Fatalf("gate fixture missing: %v", err)
					}
				}
			})
		}
	}
}

func TestDefaultRegistryRefusalTaxonomiesAreHonest(t *testing.T) {
	// Belt-and-braces on the C5 spirit: even the structural-span adapters
	// declare refusals — « nothing parses everything » is enforced, and the
	// placeholder's level is explicit so nothing upstream mistakes it for a
	// real parser.
	r := DefaultRegistry()
	docx, ok := r.Lookup("docx")
	if !ok {
		t.Fatal("docx placeholder missing from the registry")
	}
	if docx.Kit().ClaimLevel != "declared-placeholder" {
		t.Fatalf("docx must stay a declared placeholder, got %q", docx.Kit().ClaimLevel)
	}
	pdf, _ := r.Lookup("pdf")
	found := false
	for _, kind := range pdf.Kit().UnsupportedKinds {
		if kind == "scanned_image_only_pages" {
			found = true
		}
	}
	if !found {
		t.Fatal("the pdf kit must refuse scanned/image-only pages explicitly (rung 1 honesty)")
	}
}
