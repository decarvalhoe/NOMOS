package corpus

import (
	"strings"
	"testing"
)

func validCanonicalSegment() SourceSegment {
	raw := []byte("Article 1. The example.")
	return SourceSegment{
		SegmentID:          "seg-0001",
		SourceID:           "src-001",
		SourcePath:         "docs/example.md",
		Kind:               "paragraph",
		Disposition:        DispositionCanonicalAtom,
		StartByte:          0,
		EndByte:            len(raw),
		StartLine:          1,
		StartColumn:        1,
		EndLine:            1,
		EndColumn:          len(raw) + 1,
		RawTextHash:        ComputeRawTextHash(raw),
		NormalizedTextHash: ComputeNormalizedTextHash(string(raw)),
		IncludeInFeed:      true,
		IncludeInRAG:       true,
	}
}

func TestSourceSegment_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*SourceSegment)
		wantErr string // substring match; empty means must succeed
	}{
		{
			name:   "valid canonical atom",
			mutate: func(*SourceSegment) {},
		},
		{
			name:    "missing segment id",
			mutate:  func(s *SourceSegment) { s.SegmentID = "" },
			wantErr: "segment_id",
		},
		{
			name:    "whitespace segment id",
			mutate:  func(s *SourceSegment) { s.SegmentID = "   " },
			wantErr: "segment_id",
		},
		{
			name:    "missing source id",
			mutate:  func(s *SourceSegment) { s.SourceID = "" },
			wantErr: "source_id",
		},
		{
			name:    "missing source path",
			mutate:  func(s *SourceSegment) { s.SourcePath = "" },
			wantErr: "source_path",
		},
		{
			name:    "missing kind",
			mutate:  func(s *SourceSegment) { s.Kind = "" },
			wantErr: "kind",
		},
		{
			name:    "unknown disposition",
			mutate:  func(s *SourceSegment) { s.Disposition = Disposition("nonsense") },
			wantErr: "disposition",
		},
		{
			name:    "empty disposition",
			mutate:  func(s *SourceSegment) { s.Disposition = "" },
			wantErr: "disposition",
		},
		{
			name:    "start byte after end byte",
			mutate:  func(s *SourceSegment) { s.StartByte = 10; s.EndByte = 5 },
			wantErr: "start_byte",
		},
		{
			name:    "start line after end line",
			mutate:  func(s *SourceSegment) { s.StartLine = 5; s.EndLine = 2 },
			wantErr: "start_line",
		},
		{
			name: "same line columns inverted",
			mutate: func(s *SourceSegment) {
				s.StartLine = 3
				s.EndLine = 3
				s.StartColumn = 20
				s.EndColumn = 5
			},
			wantErr: "start_column",
		},
		{
			name: "same line equal columns ok",
			mutate: func(s *SourceSegment) {
				s.StartLine = 3
				s.EndLine = 3
				s.StartColumn = 5
				s.EndColumn = 5
			},
		},
		{
			name: "unsupported blocking requires reason",
			mutate: func(s *SourceSegment) {
				s.Disposition = DispositionUnsupportedBlocking
				s.UnsupportedReason = ""
			},
			wantErr: "unsupported_reason",
		},
		{
			name: "unsupported blocking with reason ok",
			mutate: func(s *SourceSegment) {
				s.Disposition = DispositionUnsupportedBlocking
				s.UnsupportedReason = "html block not yet supported"
				s.RawTextHash = ""
				s.NormalizedTextHash = ""
			},
		},
		{
			name: "canonical atom missing raw hash",
			mutate: func(s *SourceSegment) {
				s.RawTextHash = ""
			},
			wantErr: "raw_text_hash",
		},
		{
			name: "canonical atom missing normalized hash",
			mutate: func(s *SourceSegment) {
				s.NormalizedTextHash = ""
			},
			wantErr: "normalized_text_hash",
		},
		{
			name: "structure only without hashes ok",
			mutate: func(s *SourceSegment) {
				s.Disposition = DispositionStructureOnly
				s.RawTextHash = ""
				s.NormalizedTextHash = ""
			},
		},
		{
			name: "coverage only without hashes ok",
			mutate: func(s *SourceSegment) {
				s.Disposition = DispositionCoverageOnly
				s.RawTextHash = ""
				s.NormalizedTextHash = ""
			},
		},
		{
			name: "excluded by policy without hashes ok",
			mutate: func(s *SourceSegment) {
				s.Disposition = DispositionExcludedByPolicy
				s.RawTextHash = ""
				s.NormalizedTextHash = ""
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seg := validCanonicalSegment()
			tc.mutate(&seg)
			err := seg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestComputeRawTextHash_Deterministic(t *testing.T) {
	t.Parallel()

	in := []byte("alpha\nbeta\n")
	got1 := ComputeRawTextHash(in)
	got2 := ComputeRawTextHash(in)
	if got1 != got2 {
		t.Fatalf("raw hash not deterministic: %s vs %s", got1, got2)
	}
	if len(got1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d (%s)", len(got1), got1)
	}
}

func TestComputeRawTextHash_ByteSensitive(t *testing.T) {
	t.Parallel()

	a := ComputeRawTextHash([]byte("hello world"))
	b := ComputeRawTextHash([]byte("hello  world")) // two spaces
	if a == b {
		t.Fatalf("raw hash must differ for different bytes, got identical %s", a)
	}

	c := ComputeRawTextHash([]byte("hello world\n"))
	if a == c {
		t.Fatalf("raw hash must differ when trailing newline added, got identical %s", a)
	}
}

func TestComputeNormalizedTextHash_Stability(t *testing.T) {
	t.Parallel()

	base := "Article 1.\nThe quick brown fox."
	cases := []struct {
		name string
		in   string
	}{
		{"identity", base},
		{"trailing whitespace per line", "Article 1.   \nThe quick brown fox.\t  "},
		{"leading blank lines", "\n\n\nArticle 1.\nThe quick brown fox."},
		{"trailing blank lines", "Article 1.\nThe quick brown fox.\n\n\n"},
		{"surrounding blank lines", "\n\nArticle 1.\nThe quick brown fox.\n\n"},
		{"run-length whitespace inside line", "Article    1.\nThe   quick    brown   fox."},
		{"mixed tabs and spaces inside line", "Article\t\t1.\nThe\tquick \t brown  fox."},
	}

	want := ComputeNormalizedTextHash(base)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeNormalizedTextHash(tc.in)
			if got != want {
				t.Fatalf("normalized hash mismatch for %s: got %s want %s", tc.name, got, want)
			}
		})
	}
}

func TestComputeNormalizedTextHash_DifferentSemantics(t *testing.T) {
	t.Parallel()

	a := ComputeNormalizedTextHash("Article 1.\nThe quick brown fox.")
	b := ComputeNormalizedTextHash("Article 2.\nThe quick brown fox.")
	if a == b {
		t.Fatalf("normalized hash must differ for different semantic content, got identical %s", a)
	}

	// Inserting a non-blank line in the middle is a real content change
	// and must produce a different hash.
	c := ComputeNormalizedTextHash("Article 1.\nINSERTED\nThe quick brown fox.")
	if a == c {
		t.Fatalf("normalized hash must differ when an extra non-blank line is inserted, got identical %s", a)
	}
}

func TestComputeNormalizedTextHash_EmptyInputs(t *testing.T) {
	t.Parallel()

	a := ComputeNormalizedTextHash("")
	b := ComputeNormalizedTextHash("\n\n\n")
	c := ComputeNormalizedTextHash("   \n\t\n  \n")
	if a != b || a != c {
		t.Fatalf("expected blank-only inputs to share the same normalized hash, got %s / %s / %s", a, b, c)
	}
}
