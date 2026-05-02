package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testMD = `# Test Document

| Référence | TEST-001 |
| --- | --- |

Introduction text.

## Chapter 1

Chapter content.

- Item one
- Item two

### Section 1.1

Section body.

` + "```go" + `
func example() {}
` + "```" + `

## Chapter 2

Final text.
`

func writeAtomFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.md")
	if err := os.WriteFile(p, []byte(testMD), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAtomizeHelp(t *testing.T) {
	var out, err bytes.Buffer
	code := AtomizeCommand(nil, &out, &err)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("parse")) {
		t.Fatal("help should list subcommands")
	}
}

func TestAtomizeUnknownSubcommand(t *testing.T) {
	var out, err bytes.Buffer
	code := AtomizeCommand([]string{"bogus"}, &out, &err)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestAtomizeParse(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"parse", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("parse failed: %d %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["root"] == nil {
		t.Fatal("expected root in AST output")
	}
}

func TestAtomizeParseNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"parse"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected 2 for missing file, got %d", code)
	}
}

func TestAtomizeStructure(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"structure", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("structure failed: %d %s", code, errBuf.String())
	}
	var nodes []map[string]any
	if err := json.Unmarshal(out.Bytes(), &nodes); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// H1 + H2 + H3 + H2 = 4
	if len(nodes) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(nodes))
	}
}

func TestAtomizeUnits(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"units", "--doc-ref", "test-doc", "--domain", "testing", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("units failed: %d %s", code, errBuf.String())
	}
	var set map[string]any
	if err := json.Unmarshal(out.Bytes(), &set); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if set["document_ref"] != "test-doc" {
		t.Fatalf("expected doc ref test-doc, got %v", set["document_ref"])
	}
	count := set["atom_count"].(float64)
	if count == 0 {
		t.Fatal("expected atoms")
	}
}

func TestAtomizeReferences(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"references", "--doc-ref", "test-doc", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("references failed: %d %s", code, errBuf.String())
	}
	var refs []map[string]any
	if err := json.Unmarshal(out.Bytes(), &refs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected references")
	}
}

func TestAtomizeMatrix(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"matrix", "--doc-ref", "test-doc", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("matrix failed: %d %s", code, errBuf.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected matrix rows")
	}
	for _, r := range rows {
		if r["content_hash"] == nil {
			t.Fatal("matrix row should have content_hash")
		}
		if r["source_span"] == nil {
			t.Fatal("matrix row should have source_span")
		}
	}
}

func TestAtomizeChunks(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"chunks", "--doc-ref", "test-doc", "--domain", "testing", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("chunks failed: %d %s", code, errBuf.String())
	}
	var chunks []map[string]any
	if err := json.Unmarshal(out.Bytes(), &chunks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	for _, c := range chunks {
		if c["chunk_id"] == nil {
			t.Fatal("chunk should have chunk_id")
		}
		if c["text"] == nil || c["text"] == "" {
			t.Fatal("chunk should have text")
		}
	}
}

func TestAtomizeValidate(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"validate", "--doc-ref", "test-doc", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("validate failed: %d %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v, errors: %v", result["valid"], result["errors"])
	}
	if result["unique_ids"] != true {
		t.Fatal("expected unique_ids=true")
	}
}

func TestAtomizeCertify(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"certify", "--reviewer", "Alice", "--state", "approved", "--doc-ref", "test-doc", f}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("certify failed: %d %s", code, errBuf.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["reviewer"] != "Alice" {
		t.Fatalf("expected reviewer Alice, got %v", result["reviewer"])
	}
	if result["state"] != "approved" {
		t.Fatalf("expected state approved, got %v", result["state"])
	}
}

func TestAtomizeCertifyRequiresReviewer(t *testing.T) {
	f := writeAtomFixture(t)
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"certify", f}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected 2 for missing reviewer, got %d", code)
	}
}

func TestAtomizeDiff(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.md")
	newPath := filepath.Join(dir, "new.md")

	os.WriteFile(oldPath, []byte("# Doc\n\nOriginal text.\n"), 0o644)
	os.WriteFile(newPath, []byte("# Doc\n\nModified text.\n\n## New Section\n\nAdded.\n"), 0o644)

	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"diff", "--doc-ref", "test", oldPath, newPath}, &out, &errBuf)
	// diff returns 1 when changes exist
	if code != 1 {
		t.Fatalf("expected 1 (changes detected), got %d %s", code, errBuf.String())
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["added"].(float64)+result["modified"].(float64) == 0 {
		t.Fatal("expected added or modified entries")
	}
}

func TestAtomizeDiffNoChanges(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "same.md")
	os.WriteFile(p, []byte("# Doc\n\nSame text.\n"), 0o644)

	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"diff", "--doc-ref", "test", p, p}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected 0 (no changes), got %d", code)
	}
}

func TestAtomizeDiffMissingArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := AtomizeCommand([]string{"diff", "only-one.md"}, &out, &errBuf)
	if code != 2 {
		t.Fatalf("expected 2 for missing args, got %d", code)
	}
}
