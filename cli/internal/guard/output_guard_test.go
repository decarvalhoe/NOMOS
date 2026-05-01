package guard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckOutputNotInSource_OutsideRoot(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()

	if err := CheckOutputNotInSource(root, out); err != nil {
		t.Fatalf("expected no error for disjoint paths, got: %v", err)
	}
}

func TestCheckOutputNotInSource_SiblingDir(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "corpus")
	out := filepath.Join(parent, "output")
	os.MkdirAll(root, 0o755)
	os.MkdirAll(out, 0o755)

	if err := CheckOutputNotInSource(root, out); err != nil {
		t.Fatalf("expected no error for sibling dirs, got: %v", err)
	}
}

func TestCheckOutputNotInSource_ExactMatch(t *testing.T) {
	root := t.TempDir()

	err := CheckOutputNotInSource(root, root)
	if err == nil {
		t.Fatal("expected error when output equals root")
	}
	var guardErr *OutputInsideSourceError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected OutputInsideSourceError, got %T: %v", err, err)
	}
}

func TestCheckOutputNotInSource_SubdirOfRoot(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "reports", "generated")

	err := CheckOutputNotInSource(root, out)
	if err == nil {
		t.Fatal("expected error when output is subdirectory of root")
	}
	var guardErr *OutputInsideSourceError
	if !errors.As(err, &guardErr) {
		t.Fatalf("expected OutputInsideSourceError, got %T: %v", err, err)
	}
}

func TestCheckOutputNotInSource_DeepNestedSubdir(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "a", "b", "c", "d")

	err := CheckOutputNotInSource(root, out)
	if err == nil {
		t.Fatal("expected error for deeply nested output inside root")
	}
}

func TestCheckOutputNotInSource_PrefixNotSubdir(t *testing.T) {
	// /tmp/corpus vs /tmp/corpus-extra — must NOT trigger.
	parent := t.TempDir()
	root := filepath.Join(parent, "corpus")
	out := filepath.Join(parent, "corpus-extra")
	os.MkdirAll(root, 0o755)
	os.MkdirAll(out, 0o755)

	if err := CheckOutputNotInSource(root, out); err != nil {
		t.Fatalf("expected no error for prefix-only match, got: %v", err)
	}
}

func TestCheckOutputNotInSource_RelativePaths(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()

	// Use relative path for root.
	cwd, _ := os.Getwd()
	relRoot, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Skipf("cannot compute relative path: %v", err)
	}

	if err := CheckOutputNotInSource(relRoot, out); err != nil {
		t.Fatalf("expected no error with relative root outside, got: %v", err)
	}
}

func TestCheckOutputNotInSource_RelativeInsideRoot(t *testing.T) {
	root := t.TempDir()
	// Create a subdir and use relative notation.
	sub := filepath.Join(root, "gen")
	os.MkdirAll(sub, 0o755)

	cwd, _ := os.Getwd()
	relRoot, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Skipf("cannot compute relative path: %v", err)
	}
	relOut, err := filepath.Rel(cwd, sub)
	if err != nil {
		t.Skipf("cannot compute relative path: %v", err)
	}

	if err := CheckOutputNotInSource(relRoot, relOut); err == nil {
		t.Fatal("expected error for relative path inside root")
	}
}

func TestOutputInsideSourceError_Message(t *testing.T) {
	err := &OutputInsideSourceError{Root: "/project", Output: "/project/out"}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	if !contains(msg, "/project/out") || !contains(msg, "/project") {
		t.Fatalf("error message should reference both paths: %s", msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
