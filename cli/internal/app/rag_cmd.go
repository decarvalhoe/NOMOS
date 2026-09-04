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
// hands out chunks a consumer can index and later cite, and `rag manifest`
// fingerprints what was handed out so a stale index is provable rather than
// assumed.
func ragCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos rag <export|manifest> --feed <feed.json> [options]")
		return 2
	}
	switch args[0] {
	case "export":
		return ragExportCommand(args[1:], stdout, stderr)
	case "manifest":
		return ragManifestCommand(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		ragUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown rag subcommand %q (try: export, manifest)\n", args[0])
		return 2
	}
}

func ragUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: nomos rag <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  export    Emit indexable chunk records from a corpus feed")
	fmt.Fprintln(w, "  manifest  Fingerprint an export so index staleness is provable")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --feed <path>     corpus feed JSON produced by `nomos corpus feed` (required)")
	fmt.Fprintln(w, "  --format <name>   jsonl (default), langchain, or llamaindex")
	fmt.Fprintln(w, "  --output <path>   write to file (default: stdout)")
	fmt.Fprintln(w, "  --strict          exit 1 when any chunk is rejected (default: report and continue)")
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
