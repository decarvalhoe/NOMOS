// Command body-ledger emits a CorpusBodyLedger JSON artifact (FSQ-05
// #368) for the source manifest at --manifest, scanning markdown and
// supported structured sources under --corpus-root with typed scanners
// and leaving other non-text sources unscanned (their bytes go to BinaryBytes /
// UnsupportedBytes per FSQ-02 admission). Output is written to --out.
//
// This is the body-ledger generator the FSQ-08 (#371) RBOK POC runner
// invokes; it has no top-level `nomos` subcommand by design (the
// dispatch forbids adding new CLI flags to the `nomos` binary in this
// PR). Treat it as the moral equivalent of `feed-audit/`: a small
// standalone tool wired by scripts/rbok-poc-integrity.sh.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, _, stderr *os.File) int {
	flags := flag.NewFlagSet("body-ledger", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "path to source-manifest.yaml (required)")
	corpusRoot := flags.String("corpus-root", "", "path to the corpus root directory (required)")
	outPath := flags.String("out", "", "path to write the corpus-body-ledger.json (required)")
	frozenTime := flags.String("frozen-time", "", "override generated_at with this RFC3339 timestamp (test-only)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *manifestPath == "" || *corpusRoot == "" || *outPath == "" {
		fmt.Fprintln(stderr, "usage: body-ledger --manifest M --corpus-root R --out OUT [--frozen-time TS]")
		return 2
	}

	manifestBytes, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest %s: %v\n", *manifestPath, err)
		return 1
	}
	var manifest corpus.SidecarManifest
	if err := yaml.Unmarshal(manifestBytes, &manifest); err != nil {
		fmt.Fprintf(stderr, "parse manifest %s: %v\n", *manifestPath, err)
		return 1
	}

	inputs := make([]corpus.BodyLedgerSourceInput, 0, len(manifest.Sources))
	for _, src := range manifest.Sources {
		// FSQ-02 (#365) defaults; operator declarations win.
		adm := src.Admission()
		corpus.BackfillAdmission(&adm, src.Path)
		src.AdmissionStatus = adm.AdmissionStatus
		src.AtomizationStatus = adm.AtomizationStatus
		src.ExclusionReason = adm.ExclusionReason
		src.SourceRole = adm.SourceRole
		src.FormatSupport = adm.FormatSupport
		src.DerivativeOf = adm.DerivativeOf

		in := corpus.BodyLedgerSourceInput{Source: src}
		abs := filepath.Join(*corpusRoot, filepath.FromSlash(src.Path))

		ext := strings.ToLower(filepath.Ext(src.Path))
		if ext == ".md" || ext == ".mdx" {
			content, err := os.ReadFile(abs)
			if err != nil {
				fmt.Fprintf(stderr, "read source %s: %v\n", abs, err)
				return 1
			}
			segs, err := corpus.ScanMarkdown(src.ID, src.Path, content)
			if err != nil {
				fmt.Fprintf(stderr, "scan %s: %v\n", abs, err)
				return 1
			}
			in.Content = content
			in.Segments = segs
			in.SizeBytes = int64(len(content))
		} else if format, ok := corpus.StructuredFormatForPath(src.Path); ok {
			content, err := os.ReadFile(abs)
			if err != nil {
				fmt.Fprintf(stderr, "read source %s: %v\n", abs, err)
				return 1
			}
			scan, err := corpus.ScanStructuredScalars(src, content, format)
			if err != nil {
				fmt.Fprintf(stderr, "scan structured %s: %v\n", abs, err)
				return 1
			}
			in.Content = content
			in.Segments = scan.Segments
			in.SizeBytes = int64(len(content))
		} else {
			info, err := os.Stat(abs)
			switch {
			case err == nil:
				in.SizeBytes = info.Size()
			case os.IsNotExist(err):
				// Manifest references a file that is no longer on disk.
				// Record SizeBytes=0 rather than aborting; the body
				// ledger remains a best-effort coverage record.
				in.SizeBytes = 0
			default:
				fmt.Fprintf(stderr, "stat source %s: %v\n", abs, err)
				return 1
			}
		}
		inputs = append(inputs, in)
	}

	ledger, err := corpus.BuildCorpusBodyLedger(corpus.BodyLedgerInput{
		CorpusRoot:  *corpusRoot,
		GeneratedAt: *frozenTime,
		Sources:     inputs,
	})
	if err != nil {
		fmt.Fprintf(stderr, "build body ledger: %v\n", err)
		return 1
	}

	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "marshal body ledger: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*outPath, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "write %s: %v\n", *outPath, err)
		return 1
	}
	return 0
}
