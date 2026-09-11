package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NRT-031 (#714): the contract (specs/nomos-project.cue #CorpusProject) admits
// a canonical corpus; the validator must read it through the same loader as a
// product project, and refuse the shapes the contract forbids.
func TestCanonicalCorpusProjectValidates(t *testing.T) {
	res := ValidateFile(filepath.Join("..", "..", "..", "specs", "examples", "nomos-project.corpus.yaml"))
	if !res.Valid || res.ManifestType != "nomos-project" {
		t.Fatalf("corpus example must validate: %+v", res)
	}
}

func TestCanonicalCorpusRefusals(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "specs", "examples", "nomos-project.corpus.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	cases := map[string][2]string{
		"missing source_inventory": {"source_inventory:", "source_inventory_gone:"},
		"executable corpus":        {"execution: read_only", "execution: read_write"},
		"unknown mode":             {"mode: canonical_corpus", "mode: library"},
	}
	for name, c := range cases {
		mutated := strings.Replace(src, c[0], c[1], 1)
		if mutated == src {
			t.Fatalf("%s: mutation did not apply", name)
		}
		res := ValidateBytes("corpus.yaml", []byte(mutated))
		if res.Valid {
			t.Errorf("%s: must be refused", name)
		}
	}
	// a product project cannot smuggle corpus fields
	prod, _ := os.ReadFile(filepath.Join("..", "..", "..", "specs", "examples", "nomos-project.minimal.yaml"))
	res := ValidateBytes("p.yaml", append([]byte(string(prod)), []byte("\ncorpus_policy:\n  execution: read_only\n")...))
	if res.Valid {
		t.Fatal("product project with corpus_policy must be refused")
	}
}
