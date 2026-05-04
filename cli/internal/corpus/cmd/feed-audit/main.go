package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

const usageText = `usage: feed-audit --feed FEED.json --out feed-audit.json [--rag RAG.json] [--corpus DIR] [--format json|text] [--frozen-time TIMESTAMP]

Computes a deterministic, evidence-grade audit of a corpus feed (and optional RAG
metadata, and optional source corpus directory). Exit code 0 on success regardless
of audit findings — this is measurement, not a gate.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. It returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("feed-audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, usageText) }

	feedPath := flags.String("feed", "", "path to the corpus feed JSON (required)")
	outPath := flags.String("out", "", "path to write feed-audit.json (required)")
	ragPath := flags.String("rag", "", "path to RAG metadata JSON (optional)")
	corpusRoot := flags.String("corpus", "", "path to the source corpus root (optional)")
	format := flags.String("format", "json", "output format: json|text")
	frozenTime := flags.String("frozen-time", "", "override generated_at with this RFC3339 timestamp (test-only)")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*feedPath) == "" || strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stderr, usageText)
		return 2
	}
	if *format != "json" && *format != "text" {
		fmt.Fprintf(stderr, "unsupported --format %q; expected json or text\n", *format)
		return 2
	}

	feedBytes, err := os.ReadFile(*feedPath)
	if err != nil {
		fmt.Fprintf(stderr, "read feed %s: %v\n", *feedPath, err)
		return 1
	}
	var feed corpus.Feed
	if err := json.Unmarshal(feedBytes, &feed); err != nil {
		fmt.Fprintf(stderr, "parse feed %s: %v\n", *feedPath, err)
		return 1
	}

	chunks, ragProvided, err := loadChunks(*ragPath, feed)
	if err != nil {
		fmt.Fprintf(stderr, "parse rag %s: %v\n", *ragPath, err)
		return 1
	}

	now, err := resolveGeneratedAt(*frozenTime)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --frozen-time %q: %v\n", *frozenTime, err)
		return 2
	}

	cfg := FeedAuditConfig{
		FeedPath:    *feedPath,
		RAGPath:     *ragPath,
		RAGProvided: ragProvided,
		CorpusRoot:  *corpusRoot,
		GeneratedAt: now,
	}
	report := RunAudit(feed, chunks, cfg)

	if err := writeOutput(*outPath, *format, report); err != nil {
		fmt.Fprintf(stderr, "write %s: %v\n", *outPath, err)
		return 1
	}
	return 0
}

// loadChunks resolves the RAG metadata input. If --rag is provided we read
// it from disk; otherwise we fall back to the inline RAGMetadata embedded in
// the feed envelope. ragProvided reflects whether --rag was supplied (and is
// surfaced as the report's rag_path field — present vs null).
func loadChunks(ragPath string, feed corpus.Feed) ([]corpus.ChunkMetadata, bool, error) {
	if strings.TrimSpace(ragPath) == "" {
		return feed.RAGMetadata, false, nil
	}
	data, err := os.ReadFile(ragPath)
	if err != nil {
		return nil, true, err
	}
	chunks, err := parseChunkBytes(data)
	if err != nil {
		return nil, true, err
	}
	return chunks, true, nil
}

// parseChunkBytes accepts three on-disk RAG formats:
//  1. a top-level []ChunkMetadata array,
//  2. an envelope object with {"rag_metadata": [...]} (matches Feed),
//  3. an envelope object with {"chunks": [...]}.
func parseChunkBytes(data []byte) ([]corpus.ChunkMetadata, error) {
	var arr []corpus.ChunkMetadata
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	var env struct {
		RAGMetadata []corpus.ChunkMetadata `json:"rag_metadata"`
		Chunks      []corpus.ChunkMetadata `json:"chunks"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if len(env.RAGMetadata) > 0 {
		return env.RAGMetadata, nil
	}
	return env.Chunks, nil
}

func resolveGeneratedAt(frozen string) (time.Time, error) {
	if strings.TrimSpace(frozen) == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, frozen)
}

func writeOutput(path, format string, report FeedAuditReport) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(data, '\n'), 0o644)
	case "text":
		return os.WriteFile(path, []byte(renderText(report)), 0o644)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// renderText is a deterministic plain-text rendering for human review. It is
// not the canonical artifact (the JSON is) — this is a convenience.
func renderText(r FeedAuditReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "feed-audit %s\n", r.SchemaVersion)
	fmt.Fprintf(&b, "generated_at: %s\n", r.GeneratedAt)
	fmt.Fprintf(&b, "feed_path: %s\n", r.FeedPath)
	if r.RAGPath != nil {
		fmt.Fprintf(&b, "rag_path: %s\n", *r.RAGPath)
	}
	if r.CorpusRoot != nil {
		fmt.Fprintf(&b, "corpus_root: %s\n", *r.CorpusRoot)
	}
	fmt.Fprintln(&b, "")
	fmt.Fprintf(&b, "TOTALS:\n")
	fmt.Fprintf(&b, "  feed_unit_count            %d\n", r.Totals.FeedUnitCount)
	fmt.Fprintf(&b, "  chunk_count                %d\n", r.Totals.ChunkCount)
	fmt.Fprintf(&b, "  source_backed_unit_count   %d\n", r.Totals.SourceBackedUnitCount)
	fmt.Fprintf(&b, "  source_backed_chunk_count  %d\n", r.Totals.SourceBackedChunkCount)
	fmt.Fprintf(&b, "  sources_declared_active    %d\n", r.Totals.SourcesDeclaredActive)
	fmt.Fprintf(&b, "  sources_with_zero_units    %d\n", r.Totals.SourcesWithZeroUnits)
	fmt.Fprintf(&b, "  table_cell_ratio           %.4f (%d/%d)\n",
		r.TableCellRatio.Ratio, r.TableCellRatio.TableCellUnitCount, r.TableCellRatio.TotalUnitCount)
	return b.String()
}
