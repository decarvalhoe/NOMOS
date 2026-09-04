package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/ragexport"
)

// ragCommand is `nomos rag`: the interop seam between a governed Nomos corpus
// and any RAG stack. Nomos does not embed, retrieve, or rerank — `rag export`
// hands out chunks a consumer can index and later cite, `rag manifest`
// fingerprints what was handed out so a stale index is provable rather than
// assumed, `rag delta` turns two manifests into the exact reindexing plan,
// and `rag verify` gates an index against the corpus as it is now.
func ragCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos rag <export|manifest|delta|verify> [options]")
		return 2
	}
	switch args[0] {
	case "export":
		return ragExportCommand(args[1:], stdout, stderr)
	case "manifest":
		return ragManifestCommand(args[1:], stdout, stderr)
	case "delta":
		return ragDeltaCommand(args[1:], stdout, stderr)
	case "verify":
		return ragVerifyCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		ragUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown rag subcommand %q (try: export, manifest, delta, verify)\n", args[0])
		return 2
	}
}

func ragUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: nomos rag <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  export    Emit indexable chunk records from a corpus feed")
	fmt.Fprintln(w, "  manifest  Fingerprint an export so index staleness is provable")
	fmt.Fprintln(w, "  delta     Reindexing plan between two manifests (embed / update_metadata / delete)")
	fmt.Fprintln(w, "  verify    Gate an index manifest against the corpus feed as it is now (exit 1 when stale)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "export / manifest options:")
	fmt.Fprintln(w, "  --feed <path>       corpus feed JSON produced by `nomos corpus feed` (required)")
	fmt.Fprintln(w, "  --format <name>     jsonl (default), langchain, or llamaindex")
	fmt.Fprintln(w, "  --output <path>     write to file (default: stdout)")
	fmt.Fprintln(w, "  --strict            exit 1 when any chunk is rejected (default: report and continue)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "delta options:")
	fmt.Fprintln(w, "  --old <path>        manifest the index was built from (required)")
	fmt.Fprintln(w, "  --new <path>        manifest of the corpus as it is now (required)")
	fmt.Fprintln(w, "  --output <path>     write the plan to file (default: stdout)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "verify options:")
	fmt.Fprintln(w, "  --manifest <path>   manifest the index was built from (required)")
	fmt.Fprintln(w, "  --feed <path>       corpus feed JSON as it is now (required)")
	fmt.Fprintln(w, "  --output <path>     write the plan to file (default: stdout)")
	fmt.Fprintln(w, "  --strict            also exit 1 when any current chunk is rejected")
}

// loadRAGManifest reads a manifest produced by `nomos rag manifest`, refusing
// anything else: a delta computed against a foreign document proves nothing.
func loadRAGManifest(path string, prefix string, stderr io.Writer) (ragexport.Manifest, int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: read manifest: %v\n", prefix, err)
		return ragexport.Manifest{}, 1
	}
	var m ragexport.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(stderr, "%s: decode manifest %s: %v\n", prefix, path, err)
		return ragexport.Manifest{}, 1
	}
	if m.SchemaVersion != ragexport.ManifestSchemaVersion {
		fmt.Fprintf(stderr, "%s: %s is not a %s document (schema_version %q)\n",
			prefix, path, ragexport.ManifestSchemaVersion, m.SchemaVersion)
		return ragexport.Manifest{}, 1
	}
	return m, 0
}

// writeRAGDelta emits the plan and its calculated summary; with failWhenStale
// the exit code carries the verdict so a pipeline can gate on it.
func writeRAGDelta(delta ragexport.Delta, path string, stdout io.Writer, prefix string, stderr io.Writer, failWhenStale bool) int {
	data, err := json.MarshalIndent(delta, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: encode: %v\n", prefix, err)
		return 1
	}
	data = append(data, '\n')
	if code := writeRAGOutput(data, path, stdout, prefix, stderr); code != 0 {
		return code
	}
	s := delta.Summary
	verdict := "fresh"
	if delta.Stale {
		verdict = "stale"
	}
	fmt.Fprintf(stderr, "%s: index %s — %d to embed, %d metadata update(s), %d to delete, %d unchanged, %d source(s) changed, full_reindex=%v\n",
		prefix, verdict, s.Embed, s.UpdateMetadata, s.Delete, s.Unchanged, s.SourcesChanged, delta.FullReindex)
	for _, reason := range delta.FullReindexReasons {
		fmt.Fprintf(stderr, "%s: full reindex: %s\n", prefix, reason)
	}
	if failWhenStale && delta.Stale {
		return 1
	}
	return 0
}

func ragDeltaCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	const prefix = "rag delta"
	flags := flag.NewFlagSet(prefix, flag.ContinueOnError)
	flags.SetOutput(stderr)
	oldPath := flags.String("old", "", "manifest the index was built from (required)")
	newPath := flags.String("new", "", "manifest of the corpus as it is now (required)")
	output := flags.String("output", "", "write the plan to file (default: stdout)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*oldPath) == "" || strings.TrimSpace(*newPath) == "" {
		fmt.Fprintf(stderr, "%s: --old and --new are required\n", prefix)
		return 2
	}
	oldM, code := loadRAGManifest(*oldPath, prefix, stderr)
	if code != 0 {
		return code
	}
	newM, code := loadRAGManifest(*newPath, prefix, stderr)
	if code != 0 {
		return code
	}
	return writeRAGDelta(ragexport.Diff(oldM, newM), *output, stdout, prefix, stderr, false)
}

func ragVerifyCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	const prefix = "rag verify"
	flags := flag.NewFlagSet(prefix, flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "manifest the index was built from (required)")
	feedPath := flags.String("feed", "", "corpus feed JSON as it is now (required)")
	output := flags.String("output", "", "write the plan to file (default: stdout)")
	strict := flags.Bool("strict", false, "also exit 1 when any current chunk is rejected")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*feedPath) == "" {
		fmt.Fprintf(stderr, "%s: --manifest and --feed are required\n", prefix)
		return 2
	}
	indexed, code := loadRAGManifest(*manifestPath, prefix, stderr)
	if code != 0 {
		return code
	}
	feed, code := loadFeedChunks(*feedPath, prefix, stderr)
	if code != 0 {
		return code
	}

	result := ragexport.Build(feed.RAGMetadata)
	reportRejections(result.Rejections, prefix, stderr)
	format, err := ragexport.ParseFormat(indexed.Format)
	if err != nil {
		format = ragexport.FormatJSONL
	}
	current := ragexport.BuildManifest(feed, result, format)

	code = writeRAGDelta(ragexport.Diff(indexed, current), *output, stdout, prefix, stderr, true)
	if code == 0 && *strict && len(result.Rejections) > 0 {
		return 1
	}
	return code
}

// loadFeedChunks reads a feed JSON and returns its composed RAG chunks.
func loadFeedChunks(path string, prefix string, stderr io.Writer) (*corpus.Feed, int) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: read feed: %v\n", prefix, err)
		return nil, 1
	}
	var feed corpus.Feed
	if err := json.Unmarshal(raw, &feed); err != nil {
		fmt.Fprintf(stderr, "%s: decode feed: %v\n", prefix, err)
		return nil, 1
	}
	return &feed, 0
}

type ragCommonFlags struct {
	feed   string
	format ragexport.Format
	output string
	strict bool
}

func parseRAGFlags(name string, args []string, stderr io.Writer) (ragCommonFlags, int) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	feed := flags.String("feed", "", "corpus feed JSON produced by `nomos corpus feed` (required)")
	format := flags.String("format", "jsonl", "output projection: jsonl, langchain, or llamaindex")
	output := flags.String("output", "", "write to file (default: stdout)")
	strict := flags.Bool("strict", false, "exit 1 when any chunk is rejected")
	if err := flags.Parse(args); err != nil {
		return ragCommonFlags{}, 2
	}
	if strings.TrimSpace(*feed) == "" {
		fmt.Fprintf(stderr, "%s: --feed is required\n", name)
		return ragCommonFlags{}, 2
	}
	parsed, err := ragexport.ParseFormat(*format)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return ragCommonFlags{}, 2
	}
	return ragCommonFlags{feed: *feed, format: parsed, output: *output, strict: *strict}, 0
}

// writeRAGOutput sends bytes to --output or stdout.
func writeRAGOutput(data []byte, path string, stdout io.Writer, prefix string, stderr io.Writer) int {
	if strings.TrimSpace(path) == "" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "%s: write: %v\n", prefix, err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: write %s: %v\n", prefix, path, err)
		return 1
	}
	return 0
}

// reportRejections prints every refused chunk to stderr. Rejections are never
// silent: a chunk missing from an index is a retrieval gap, and a gap that is
// not reported is indistinguishable from a corpus that had nothing to say.
func reportRejections(rejections []ragexport.Rejection, prefix string, stderr io.Writer) {
	for _, r := range rejections {
		fmt.Fprintf(stderr, "%s: rejected %s [%s]: %s\n", prefix, r.ChunkID, r.Code, r.Message)
	}
}

func ragExportCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	const prefix = "rag export"
	opts, code := parseRAGFlags(prefix, args, stderr)
	if code != 0 {
		return code
	}
	feed, code := loadFeedChunks(opts.feed, prefix, stderr)
	if code != 0 {
		return code
	}

	result := ragexport.Build(feed.RAGMetadata)
	reportRejections(result.Rejections, prefix, stderr)

	data, err := ragexport.Encode(result.Records, opts.format)
	if err != nil {
		fmt.Fprintf(stderr, "%s: encode: %v\n", prefix, err)
		return 1
	}
	if code := writeRAGOutput(data, opts.output, stdout, prefix, stderr); code != 0 {
		return code
	}

	fmt.Fprintf(stderr, "%s: %d chunk(s) exported, %d rejected (format %s)\n",
		prefix, len(result.Records), len(result.Rejections), opts.format)
	if opts.strict && len(result.Rejections) > 0 {
		return 1
	}
	return 0
}

func ragManifestCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	const prefix = "rag manifest"
	opts, code := parseRAGFlags(prefix, args, stderr)
	if code != 0 {
		return code
	}
	feed, code := loadFeedChunks(opts.feed, prefix, stderr)
	if code != 0 {
		return code
	}

	result := ragexport.Build(feed.RAGMetadata)
	reportRejections(result.Rejections, prefix, stderr)

	manifest := ragexport.BuildManifest(feed, result, opts.format)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: encode: %v\n", prefix, err)
		return 1
	}
	data = append(data, '\n')
	if code := writeRAGOutput(data, opts.output, stdout, prefix, stderr); code != 0 {
		return code
	}
	if opts.strict && len(result.Rejections) > 0 {
		return 1
	}
	return 0
}
