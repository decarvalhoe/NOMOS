package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/bundle"
	"github.com/RBOKproject/Nomos/cli/internal/guard"
)

// bundleCommand is `nomos bundle`: it emits a Canonical Knowledge Bundle
// (specs/canonical-knowledge-bundle.cue) from a real corpus run so a downstream
// consumer imports a NOMOS-produced artifact instead of a hand-crafted fixture
// (CKM-H4 / #522). Source ingestion is read-only.
func bundleCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("bundle", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "source corpus root to atomize (required)")
	bundleID := flags.String("bundle-id", "nomos-ckm-bundle", "bundle identifier ([a-z0-9][a-z0-9._-]*)")
	producer := flags.String("producer", "nomos", "bundle producer identifier")
	domain := flags.String("domain", "", "domain tag recorded on emitted atoms")
	feedID := flags.String("feed-id", "", "feed identifier (default: <bundle-id>-feed)")
	feedVersion := flags.String("feed-version", "", "feed content version (default: deterministic <bundle-id>@<generated_at>)")
	country := flags.String("country", "", "feed jurisdiction country (omitted when empty)")
	canton := flags.String("canton", "", "feed jurisdiction canton (omitted when empty)")
	commune := flags.String("commune", "", "feed jurisdiction commune (omitted when empty)")
	out := flags.String("out", "", "bundle output path outside the source root (default: stdout)")
	repo := flags.String("repo", "", "override trace corpus repo (owner/name); default: git origin")
	branch := flags.String("branch", "", "override trace corpus branch; default: git HEAD ref")
	commit := flags.String("commit", "", "override trace corpus commit sha; default: git HEAD")
	workflowRunID := flags.String("workflow-run-id", "", "trace run identifier (default: local-<shortsha>)")
	event := flags.String("event", "", "trace run event (default: workflow_dispatch)")
	var exts listFlag
	flags.Var(&exts, "ext", "source extension to include; repeatable (default: .md)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*root) == "" {
		fmt.Fprintln(stderr, "bundle: --root is required")
		return 2
	}
	if len(exts) == 0 {
		exts = listFlag{".md"}
	}
	if err := validateOutputPath(*root, *out); err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 2
	}
	before, hadSnapshot, err := readOnlyBefore(*root)
	if err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 2
	}

	sources, err := collectBundleSources(*root, exts)
	if err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 1
	}
	if len(sources) == 0 {
		fmt.Fprintf(stderr, "bundle: no source files with extensions %v under %s\n", []string(exts), *root)
		return 1
	}

	gitCtx := bundle.TraceGitContext{
		Repo:   firstNonEmptyStr(*repo, bundle.ParseRepoFromRemote(gitValue(*root, "config", "--get", "remote.origin.url"))),
		Branch: firstNonEmptyStr(*branch, gitValue(*root, "rev-parse", "--abbrev-ref", "HEAD")),
		Commit: firstNonEmptyStr(*commit, gitValue(*root, "rev-parse", "HEAD")),
	}

	now := time.Now().UTC()
	trace, err := bundle.NewTraceManifest(gitCtx, now.Format(time.RFC3339), *bundleID, *workflowRunID, *event, scopeGlobs(exts))
	if err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 1
	}

	var jurisdiction *bundle.Jurisdiction
	if j := (bundle.Jurisdiction{Country: *country, Canton: *canton, Commune: *commune}); !j.IsZero() {
		jurisdiction = &j
	}

	b, err := bundle.Build(bundle.BuildInput{
		BundleID:     *bundleID,
		Producer:     *producer,
		Domain:       *domain,
		FeedID:       *feedID,
		FeedVersion:  *feedVersion,
		Jurisdiction: jurisdiction,
		GeneratedAt:  now,
		Sources:      sources,
		Trace:        trace,
	})
	if err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 1
	}

	data, err := b.Marshal()
	if err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 1
	}
	if *out == "" {
		if _, err := stdout.Write(append(data, '\n')); err != nil {
			fmt.Fprintf(stderr, "bundle: %v\n", err)
			return 1
		}
	} else if err := writeFile(*out, func(w io.Writer) error { _, e := w.Write(data); return e }); err != nil {
		fmt.Fprintf(stderr, "bundle: %v\n", err)
		return 1
	}

	if hadSnapshot {
		if err := guard.GuardReadOnly(before); err != nil {
			fmt.Fprintf(stderr, "bundle: %v\n", err)
			return 1
		}
	}
	if *out != "" {
		fmt.Fprintf(stdout, "wrote bundle %s (%d node(s)) into %s\n", *bundleID, bundleNodeCount(b), filepath.Clean(*out))
	}
	return 0
}

func collectBundleSources(root string, exts listFlag) ([]bundle.SourceFile, error) {
	extSet := map[string]bool{}
	for _, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		extSet[strings.ToLower(e)] = true
	}
	var sources []bundle.SourceFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !extSet[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		sources = append(sources, bundle.SourceFile{
			RelPath: filepath.ToSlash(rel),
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].RelPath < sources[j].RelPath })
	return sources, nil
}

func scopeGlobs(exts listFlag) []string {
	globs := make([]string, 0, len(exts))
	for _, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		globs = append(globs, "**/*"+e)
	}
	return globs
}

func bundleNodeCount(b bundle.Bundle) int {
	n := 0
	for _, f := range b.Feeds {
		n += len(f.Nodes)
	}
	return n
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
