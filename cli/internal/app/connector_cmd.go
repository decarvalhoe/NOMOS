package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/connector"
)

// connectorCommand is `nomos connector`: real, read-only fetch of an open Swiss
// source → real content hash → line-span atoms → body-ledger byte coverage
// (CKM-H5 / #523). It replaces the synthetic-hash placeholder with a live
// pipeline whose evidence is self-describing and carries no full text.
func connectorCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		connectorUsage(stdout)
		return 0
	}
	switch args[0] {
	case "fetch":
		return connectorFetch(args[1:], stdout, stderr)
	case "sources":
		return connectorSources(stdout)
	case "help", "-h", "--help":
		connectorUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown connector subcommand %q\n\n", args[0])
		connectorUsage(stderr)
		return 2
	}
}

func connectorUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos connector fetch --url <https url> [--connector-id <id>] [--accept <media type>] [--out <evidence.json>] [--sample N] [--max-bytes N] [--timeout 30s]")
	fmt.Fprintln(w, "  nomos connector sources")
}

func connectorSources(stdout io.Writer) int {
	ids := make([]string, 0, len(connector.KnownSources))
	for id := range connector.KnownSources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(stdout, "Known open Swiss sources (paid norms such as SIA are intentionally excluded):")
	for _, id := range ids {
		d := connector.KnownSources[id]
		fmt.Fprintf(stdout, "  %-26s %s — %s (%s)\n", d.ID, d.Authority, d.Description, d.Access)
	}
	return 0
}

func connectorFetch(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("connector fetch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	url := flags.String("url", "", "https URL of an open source to fetch (required)")
	connectorID := flags.String("connector-id", "ch-ofs-commune-register", "known source id (see `nomos connector sources`)")
	accept := flags.String("accept", "", "Accept header for ELI content negotiation (e.g. application/rdf+xml); empty = none")
	out := flags.String("out", "", "evidence output path (default: stdout)")
	sample := flags.Int("sample", 5, "number of atom previews to record in the evidence")
	maxBytes := flags.Int64("max-bytes", connector.DefaultMaxBytes, "maximum bytes to fetch")
	timeout := flags.Duration("timeout", 30*time.Second, "fetch timeout")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*url) == "" {
		fmt.Fprintln(stderr, "connector fetch: --url is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	content, result, err := connector.Fetch(ctx, *url, connector.FetchOptions{
		Client:   &http.Client{Timeout: *timeout},
		MaxBytes: *maxBytes,
		Now:      time.Now().UTC(),
		Accept:   *accept,
	})
	if err != nil {
		fmt.Fprintf(stderr, "connector fetch: %v\n", err)
		return 1
	}

	ledger := connector.BuildLineLedger(content)
	evidence := connector.BuildEvidence(*connectorID, content, result, ledger, *sample)
	if err := evidence.Validate(); err != nil {
		fmt.Fprintf(stderr, "connector fetch: %v\n", err)
		return 1
	}

	data, err := evidence.Marshal()
	if err != nil {
		fmt.Fprintf(stderr, "connector fetch: %v\n", err)
		return 1
	}
	if *out == "" {
		if _, err := stdout.Write(append(data, '\n')); err != nil {
			fmt.Fprintf(stderr, "connector fetch: %v\n", err)
			return 1
		}
		return 0
	}
	if err := writeFile(*out, func(w io.Writer) error { _, e := w.Write(append(data, '\n')); return e }); err != nil {
		fmt.Fprintf(stderr, "connector fetch: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "fetched %d bytes (%s), %d atoms, 0 uncovered → %s\n",
		result.ByteCount, result.SHA256, evidence.AtomCount, filepath.Clean(*out))
	return 0
}
