package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/bundle"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/ragexport"
)

// ragCommand is `nomos rag`: the interop seam between a governed Nomos corpus
// and any RAG stack. Nomos does not embed, retrieve, or rerank — `rag export`
// hands out chunks a consumer can index and later cite (scoped by a Knowledge
// Lens when asked), `rag manifest` fingerprints what was handed out so a
// stale index is provable rather than assumed, `rag delta` turns two
// manifests into the exact reindexing plan, and `rag verify` gates an index
// against the corpus as it is now.
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
	fmt.Fprintln(w, "  export    Emit indexable chunk records from a corpus feed or a CKM bundle")
	fmt.Fprintln(w, "  manifest  Fingerprint an export (index digest, per-chunk fingerprints, retrieval contract)")
	fmt.Fprintln(w, "  delta     Reindexing plan between two manifests (embed / update_metadata / delete)")
	fmt.Fprintln(w, "  verify    Gate an index manifest against the corpus as it is now (exit 1 when stale)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Input (export / manifest / verify): exactly one of")
	fmt.Fprintln(w, "  --feed <path>              corpus feed JSON produced by `nomos corpus feed` (no facets)")
	fmt.Fprintln(w, "  --bundle <path>            CKM bundle JSON produced by `nomos bundle` (faceted nodes)")
	fmt.Fprintln(w, "Scope (export / manifest / verify):")
	fmt.Fprintln(w, "  --lens <path>              Knowledge Lens YAML enforced on the export; excluded chunks are reported")
	fmt.Fprintln(w, "  --document-facets <path>   pack facets per source path (YAML, `document_facets:` map accepted)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "export / manifest options:")
	fmt.Fprintln(w, "  --format <name>            jsonl (default), langchain, or llamaindex")
	fmt.Fprintln(w, "  --output <path>            write to file (default: stdout)")
	fmt.Fprintln(w, "  --strict                   exit 1 when any chunk is rejected, or when the lens excluded everything")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "delta options:")
	fmt.Fprintln(w, "  --old <path>               manifest the index was built from (required)")
	fmt.Fprintln(w, "  --new <path>               manifest of the corpus as it is now (required)")
	fmt.Fprintln(w, "  --output <path>            write the plan to file (default: stdout)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "verify options:")
	fmt.Fprintln(w, "  --manifest <path>          manifest the index was built from (required)")
	fmt.Fprintln(w, "  --output <path>            write the plan to file (default: stdout)")
	fmt.Fprintln(w, "  --strict                   also exit 1 when any current chunk is rejected")
}

// ragInputs are the shared input/scope flags of export, manifest and verify.
type ragInputs struct {
	feed           *string
	bundle         *string
	lens           *string
	documentFacets *string
}

func addRAGInputFlags(flags *flag.FlagSet) ragInputs {
	return ragInputs{
		feed:           flags.String("feed", "", "corpus feed JSON produced by `nomos corpus feed`"),
		bundle:         flags.String("bundle", "", "CKM bundle JSON produced by `nomos bundle`"),
		lens:           flags.String("lens", "", "Knowledge Lens YAML enforced on the export"),
		documentFacets: flags.String("document-facets", "", "pack document facets YAML keyed by source path"),
	}
}

func (in ragInputs) validate(name string, stderr io.Writer) int {
	feed := strings.TrimSpace(*in.feed) != ""
	bundle := strings.TrimSpace(*in.bundle) != ""
	switch {
	case feed && bundle:
		fmt.Fprintf(stderr, "%s: --feed and --bundle are mutually exclusive\n", name)
		return 2
	case !feed && !bundle:
		fmt.Fprintf(stderr, "%s: one of --feed or --bundle is required\n", name)
		return 2
	}
	return 0
}

// load resolves the inputs and the feed envelope the manifest binds to. For a
// bundle, the envelope is synthesised from the bundle itself: its schema as
// format, the sha256 of its bytes as content hash, its generated_at.
func (in ragInputs) load(prefix string, stderr io.Writer) ([]ragexport.Input, *corpus.Feed, int) {
	if strings.TrimSpace(*in.bundle) != "" {
		raw, err := os.ReadFile(*in.bundle)
		if err != nil {
			fmt.Fprintf(stderr, "%s: read bundle: %v\n", prefix, err)
			return nil, nil, 1
		}
		var b bundle.Bundle
		if err := json.Unmarshal(raw, &b); err != nil {
			fmt.Fprintf(stderr, "%s: decode bundle %s: %v\n", prefix, *in.bundle, err)
			return nil, nil, 1
		}
		if len(b.Feeds) == 0 {
			fmt.Fprintf(stderr, "%s: bundle %s carries no feeds\n", prefix, *in.bundle)
			return nil, nil, 1
		}
		sum := sha256.Sum256(raw)
		envelope := &corpus.Feed{
			Format:      b.SchemaVersion,
			ContentHash: "sha256:" + hex.EncodeToString(sum[:]),
			GeneratedAt: b.GeneratedAt,
		}
		return ragexport.InputsFromBundle(&b), envelope, 0
	}
	feed, code := loadFeedChunks(*in.feed, prefix, stderr)
	if code != 0 {
		return nil, nil, code
	}
	inputs := make([]ragexport.Input, 0, len(feed.RAGMetadata))
	for _, m := range feed.RAGMetadata {
		inputs = append(inputs, ragexport.Input{Chunk: m})
	}
	return inputs, feed, 0
}

// options resolves the scope flags.
func (in ragInputs) options(prefix string, stderr io.Writer) (ragexport.Options, int) {
	var opts ragexport.Options
	if strings.TrimSpace(*in.lens) != "" {
		lens, err := loadKnowledgeLens(*in.lens)
		if err != nil {
			fmt.Fprintf(stderr, "%s: lens: %v\n", prefix, err)
			return opts, 1
		}
		opts.Lens = lens
	}
	if strings.TrimSpace(*in.documentFacets) != "" {
		facets, err := loadDocumentFacets(*in.documentFacets)
		if err != nil {
			fmt.Fprintf(stderr, "%s: document facets: %v\n", prefix, err)
			return opts, 1
		}
		opts.DocumentFacets = facets
	}
	return opts, 0
}

// loadDocumentFacets reads pack document facets: a YAML map from source path
// to facet values, either bare or under a top-level `document_facets:` key
// (the shape of a pack retrieval harness file).
func loadDocumentFacets(path string) (map[string]atomization.Facets, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m, ok := generic.(map[string]any); ok {
		if inner, ok := m["document_facets"]; ok {
			generic = inner
		}
	}
	bridged, err := json.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("normalize %s: %w", path, err)
	}
	var out map[string]atomization.Facets
	if err := json.Unmarshal(bridged, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no document facets found (expected a map keyed by source path)", path)
	}
	return out, nil
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
	in     ragInputs
	format ragexport.Format
	output string
	strict bool
}

func parseRAGFlags(name string, args []string, stderr io.Writer) (ragCommonFlags, int) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	in := addRAGInputFlags(flags)
	format := flags.String("format", "jsonl", "output projection: jsonl, langchain, or llamaindex")
	output := flags.String("output", "", "write to file (default: stdout)")
	strict := flags.Bool("strict", false, "exit 1 when any chunk is rejected")
	if err := flags.Parse(args); err != nil {
		return ragCommonFlags{}, 2
	}
	if code := in.validate(name, stderr); code != 0 {
		return ragCommonFlags{}, code
	}
	parsed, err := ragexport.ParseFormat(*format)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return ragCommonFlags{}, 2
	}
	return ragCommonFlags{in: in, format: parsed, output: *output, strict: *strict}, 0
}

// buildRAG loads inputs + scope and builds the result, reporting every
// rejection and exclusion on stderr. Neither is ever silent: a chunk missing
// from an index is a retrieval gap, and a gap that is not reported is
// indistinguishable from a corpus that had nothing to say.
func buildRAG(in ragInputs, prefix string, stderr io.Writer) (ragexport.Result, *corpus.Feed, int) {
	inputs, envelope, code := in.load(prefix, stderr)
	if code != 0 {
		return ragexport.Result{}, nil, code
	}
	opts, code := in.options(prefix, stderr)
	if code != 0 {
		return ragexport.Result{}, nil, code
	}
	result := ragexport.BuildInputs(inputs, opts)
	for _, r := range result.Rejections {
		fmt.Fprintf(stderr, "%s: rejected %s [%s]: %s\n", prefix, r.ChunkID, r.Code, r.Message)
	}
	for _, e := range result.Excluded {
		fmt.Fprintf(stderr, "%s: excluded %s [%s]: %s\n", prefix, e.ChunkID, e.Code, e.Reason)
	}
	if result.Lens != nil {
		fmt.Fprintf(stderr, "%s: lens %s enforced: %d chunk(s) excluded\n", prefix, result.Lens.ID, len(result.Excluded))
	}
	return result, envelope, 0
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

func ragExportCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	const prefix = "rag export"
	opts, code := parseRAGFlags(prefix, args, stderr)
	if code != 0 {
		return code
	}
	result, _, code := buildRAG(opts.in, prefix, stderr)
	if code != 0 {
		return code
	}

	data, err := ragexport.Encode(result.Records, opts.format)
	if err != nil {
		fmt.Fprintf(stderr, "%s: encode: %v\n", prefix, err)
		return 1
	}
	if code := writeRAGOutput(data, opts.output, stdout, prefix, stderr); code != 0 {
		return code
	}

	fmt.Fprintf(stderr, "%s: %d chunk(s) exported, %d rejected, %d excluded by lens (format %s)\n",
		prefix, len(result.Records), len(result.Rejections), len(result.Excluded), opts.format)
	if opts.strict && len(result.Rejections) > 0 {
		return 1
	}
	// Fail closed on an empty scope: a lens that excluded everything (for
	// instance over a feed that carries no facets) must not let a pipeline
	// build an empty index and call it done.
	if opts.strict && len(result.Records) == 0 && len(result.Excluded) > 0 {
		fmt.Fprintf(stderr, "%s: nothing exported: every chunk was excluded by the lens\n", prefix)
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
	result, envelope, code := buildRAG(opts.in, prefix, stderr)
	if code != 0 {
		return code
	}

	manifest := ragexport.BuildManifest(envelope, result, opts.format)
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
	in := addRAGInputFlags(flags)
	output := flags.String("output", "", "write the plan to file (default: stdout)")
	strict := flags.Bool("strict", false, "also exit 1 when any current chunk is rejected")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" {
		fmt.Fprintf(stderr, "%s: --manifest is required\n", prefix)
		return 2
	}
	if code := in.validate(prefix, stderr); code != 0 {
		return code
	}
	indexed, code := loadRAGManifest(*manifestPath, prefix, stderr)
	if code != 0 {
		return code
	}
	result, envelope, code := buildRAG(in, prefix, stderr)
	if code != 0 {
		return code
	}
	format, err := ragexport.ParseFormat(indexed.Format)
	if err != nil {
		format = ragexport.FormatJSONL
	}
	current := ragexport.BuildManifest(envelope, result, format)

	code = writeRAGDelta(ragexport.Diff(indexed, current), *output, stdout, prefix, stderr, true)
	if code == 0 && *strict && len(result.Rejections) > 0 {
		return 1
	}
	return code
}
