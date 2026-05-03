package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// corpusProfileFeedCommand implements "nomos corpus feed --profile <name>".
// When --profile is set, delegates to RunProfileFeed instead of GenerateFeed.
func corpusProfileFeedCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus feed --profile", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "", "corpus profile (e.g. rbok-lawbook)")
	root := flags.String("root", ".", "corpus root directory")
	format := flags.String("format", "json", "output format: json or text")
	out := flags.String("out", "", "write result to file (default: stdout)")
	outputsRaw := flags.String("outputs", "", "comma-separated output sections (default: all profile outputs)")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *profile == "" {
		fmt.Fprintf(stderr, "corpus feed --profile: --profile is required\nKnown profiles: %s\n",
			strings.Join(corpus.KnownProfiles(), ", "))
		return 2
	}

	if flags.NArg() == 1 && *root == "." {
		*root = flags.Arg(0)
	}

	var outputs []corpus.OutputFlag
	if *outputsRaw != "" {
		for _, s := range strings.Split(*outputsRaw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				outputs = append(outputs, corpus.OutputFlag(s))
			}
		}
	}

	result, err := corpus.RunProfileFeed(corpus.ProfileFeedInput{
		Profile:    *profile,
		CorpusRoot: *root,
		Outputs:    outputs,
	})
	if err != nil {
		fmt.Fprintf(stderr, "corpus feed --profile: %v\n", err)
		return 1
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(stderr, "corpus feed --profile: create output: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	switch strings.ToLower(*format) {
	case "text":
		writeProfileFeedText(w, result)
	default:
		if err := corpus.WriteProfileFeedJSON(w, result); err != nil {
			fmt.Fprintf(stderr, "corpus feed --profile: write: %v\n", err)
			return 1
		}
	}

	if len(result.Errors) > 0 {
		return 1
	}
	return 0
}

// corpusDiagnoseCommand implements "nomos corpus diagnose --profile <name>".
func corpusDiagnoseCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("corpus diagnose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "", "corpus profile (required)")
	root := flags.String("root", ".", "corpus root directory")
	format := flags.String("format", "json", "output format: json or text")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *profile == "" {
		fmt.Fprintf(stderr, "corpus diagnose: --profile is required\nKnown profiles: %s\n",
			strings.Join(corpus.KnownProfiles(), ", "))
		return 2
	}

	if flags.NArg() == 1 && *root == "." {
		*root = flags.Arg(0)
	}

	verdict, err := corpus.DiagnoseProfile(*profile, *root)
	if err != nil {
		fmt.Fprintf(stderr, "corpus diagnose: %v\n", err)
		return 1
	}

	switch strings.ToLower(*format) {
	case "text":
		writeProfileDiagnoseText(stdout, verdict)
	default:
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(verdict); err != nil {
			fmt.Fprintf(stderr, "corpus diagnose: write: %v\n", err)
			return 1
		}
	}

	if verdict.Verdict == "blocked" || verdict.Verdict == "corpus_blocked" {
		return 1
	}
	return 0
}

// corpusProfilesCommand implements "nomos corpus profiles".
func corpusProfilesCommand(_ []string, stdout io.Writer, _ io.Writer) int {
	for _, name := range corpus.KnownProfiles() {
		p, _ := corpus.LookupProfile(name)
		fmt.Fprintf(stdout, "%-20s %s\n", p.Name, p.Description)
	}
	return 0
}

func writeProfileFeedText(w io.Writer, result corpus.ProfileFeedResult) {
	fmt.Fprintf(w, "profile:  %s\n", result.Profile)
	fmt.Fprintf(w, "sources:  %d\n", result.SourceCount)
	fmt.Fprintf(w, "units:    %d\n", result.UnitCount)
	fmt.Fprintf(w, "sections: %d\n", len(result.Sections))
	flags := make([]string, 0, len(result.Sections))
	for flag := range result.Sections {
		flags = append(flags, string(flag))
	}
	sort.Strings(flags)
	for _, flag := range flags {
		data := result.Sections[corpus.OutputFlag(flag)]
		fmt.Fprintf(w, "  %-20s %d entries\n", flag, profileSectionEntryCount(corpus.OutputFlag(flag), data))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintln(w, "\nerrors:")
		for _, e := range result.Errors {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "\nwarnings:")
		for _, warning := range result.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
}

func profileSectionEntryCount(flag corpus.OutputFlag, data json.RawMessage) int {
	switch flag {
	case corpus.OutputFeed:
		var assembly corpus.MultiFeedAssembly
		if json.Unmarshal(data, &assembly) == nil {
			return assembly.TotalNodes
		}
	case corpus.OutputAtomizationReport:
		var report corpus.ProfileAtomizationReport
		if json.Unmarshal(data, &report) == nil {
			return report.TotalNodes
		}
	case corpus.OutputRAGMetadata, corpus.OutputTraceabilityMatrix,
		corpus.OutputIndex, corpus.OutputGovernance, corpus.OutputCitation, corpus.OutputImport:
		var entries []json.RawMessage
		if json.Unmarshal(data, &entries) == nil {
			return len(entries)
		}
	}
	return 0
}

func writeProfileDiagnoseText(w io.Writer, v corpus.DiagnoseVerdict) {
	fmt.Fprintf(w, "profile:    %s\n", v.Profile)
	fmt.Fprintf(w, "verdict:    %s\n", v.Verdict)
	fmt.Fprintf(w, "confidence: %s\n", v.Confidence)
	fmt.Fprintf(w, "summary:    %s\n", v.Summary)
	if len(v.Blockers) > 0 {
		fmt.Fprintln(w, "\nblockers:")
		for _, b := range v.Blockers {
			fmt.Fprintf(w, "  - %s\n", b)
		}
	}
	if len(v.Warnings) > 0 {
		fmt.Fprintln(w, "\nwarnings:")
		for _, wn := range v.Warnings {
			fmt.Fprintf(w, "  - %s\n", wn)
		}
	}
}
