package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"gopkg.in/yaml.v3"
)

// AtomizeCommand is the entry point for `nomos atomize <subcommand>`.
// It dispatches to sub-commands that operate on the atomization pipeline.
func AtomizeCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	subcommands := map[string]commandFunc{
		"parse":      atomizeParse,
		"structure":  atomizeStructure,
		"units":      atomizeUnits,
		"references": atomizeReferences,
		"matrix":     atomizeMatrix,
		"chunks":     atomizeChunks,
		"validate":   atomizeValidate,
		"certify":    atomizeCertify,
		"diff":       atomizeDiff,
	}

	if len(args) == 0 {
		atomizeHelp(stdout)
		return 0
	}

	name := args[0]
	cmd, ok := subcommands[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown atomize subcommand %q\n\n", name)
		atomizeHelp(stderr)
		return 2
	}

	return cmd(args[1:], stdout, stderr)
}

func atomizeHelp(w io.Writer) {
	fmt.Fprintln(w, "nomos atomize <subcommand> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  parse       Parse Markdown into block AST")
	fmt.Fprintln(w, "  structure   Show document structure (heading tree)")
	fmt.Fprintln(w, "  units       Generate atoms from source")
	fmt.Fprintln(w, "  references  Extract canonical references")
	fmt.Fprintln(w, "  matrix      Build traceability matrix from atoms")
	fmt.Fprintln(w, "  chunks      Generate RAG-ready chunks with metadata")
	fmt.Fprintln(w, "  validate    Validate atom set completeness")
	fmt.Fprintln(w, "  certify     Mark atoms as approved with reviewer")
	fmt.Fprintln(w, "  diff        Compare two atom sets for drift")
}

// --- parse: Markdown → block AST ---

func atomizeParse(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize parse", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize parse <file.md>")
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	return writeAtomJSON(stdout, stderr, ast)
}

// --- structure: heading tree ---

func atomizeStructure(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize structure", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize structure <file.md>")
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))

	type StructNode struct {
		ID    string `json:"id"`
		Level int    `json:"level"`
		Title string `json:"title"`
		Line  int    `json:"line"`
	}

	var tree []StructNode
	for _, blk := range ast.Blocks {
		if blk.Type == atomization.BlockHeading {
			tree = append(tree, StructNode{
				ID:    blk.ID,
				Level: blk.Level,
				Title: blk.Text,
				Line:  blk.Span.StartLine,
			})
		}
	}
	return writeAtomJSON(stdout, stderr, tree)
}

// --- units: Markdown → atoms ---

func atomizeUnits(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize units", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	domain := flags.String("domain", "", "domain name")
	state := flags.String("state", "draft", "default review state")
	facets := flags.Bool("facets", false, "emit CKM-01 facets on each atom")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize units [--doc-ref REF] [--domain DOM] [--facets] <file.md>")
		return 2
	}

	filePath := flags.Arg(0)
	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef:  *docRef,
		SourceFile:   filePath,
		Domain:       *domain,
		DefaultState: atomization.ReviewState(*state),
		EmitFacets:   *facets,
	})

	return writeAtomJSON(stdout, stderr, set)
}

// --- references: extract canonical refs ---

func atomizeReferences(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize references", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize references [--doc-ref REF] <file.md>")
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef: *docRef,
		SourceFile:  flags.Arg(0),
	})

	type Ref struct {
		AtomID       string `json:"atom_id"`
		CanonicalRef string `json:"canonical_ref"`
		Type         string `json:"type"`
		Line         int    `json:"line"`
	}
	refs := make([]Ref, 0, len(set.Atoms))
	for _, a := range set.Atoms {
		refs = append(refs, Ref{
			AtomID:       a.ID,
			CanonicalRef: a.CanonicalRef,
			Type:         string(a.Type),
			Line:         a.SourceSpan.StartLine,
		})
	}
	return writeAtomJSON(stdout, stderr, refs)
}

// --- matrix: build traceability matrix ---

func atomizeMatrix(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize matrix", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	domain := flags.String("domain", "", "domain name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize matrix [--doc-ref REF] [--domain DOM] <file.md>")
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef: *docRef,
		SourceFile:  flags.Arg(0),
		Domain:      *domain,
	})

	type MatrixRow struct {
		AtomID       string `json:"atom_id"`
		CanonicalRef string `json:"canonical_ref"`
		Type         string `json:"type"`
		Depth        int    `json:"depth"`
		ParentID     string `json:"parent_id,omitempty"`
		ContentHash  string `json:"content_hash"`
		ReviewState  string `json:"review_state"`
		SourceSpan   string `json:"source_span"`
	}
	rows := make([]MatrixRow, 0, len(set.Atoms))
	for _, a := range set.Atoms {
		rows = append(rows, MatrixRow{
			AtomID:       a.ID,
			CanonicalRef: a.CanonicalRef,
			Type:         string(a.Type),
			Depth:        a.Depth,
			ParentID:     a.ParentID,
			ContentHash:  a.ContentHash,
			ReviewState:  string(a.ReviewState),
			SourceSpan:   a.SourceSpan.String(),
		})
	}
	return writeAtomJSON(stdout, stderr, rows)
}

// --- chunks: RAG-ready chunks ---

func atomizeChunks(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize chunks", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	domain := flags.String("domain", "", "domain name")
	facets := flags.Bool("facets", false, "emit CKM-01 facets on each chunk")
	lensPath := flags.String("lens", "", "CKM-02 knowledge-lens spec (YAML/JSON); filters chunks by facets, recording excluded units")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize chunks [--doc-ref REF] [--domain DOM] [--facets] [--lens FILE] <file.md>")
		return 2
	}

	// A lens decides retrieval scope by facets, so facets must be derived for the
	// lens to act on. --lens therefore implies faceting (--facets alone just
	// surfaces them on the wire without filtering).
	emitFacets := *facets || *lensPath != ""

	var lens *atomization.KnowledgeLens
	if *lensPath != "" {
		loaded, err := loadKnowledgeLens(*lensPath)
		if err != nil {
			fmt.Fprintf(stderr, "lens: %v\n", err)
			return 2
		}
		lens = loaded
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef: *docRef,
		SourceFile:  flags.Arg(0),
		Domain:      *domain,
		EmitFacets:  emitFacets,
	})

	type Chunk struct {
		ChunkID      string              `json:"chunk_id"`
		AtomID       string              `json:"atom_id"`
		CanonicalRef string              `json:"canonical_ref"`
		Text         string              `json:"text"`
		ContentHash  string              `json:"content_hash"`
		Domain       string              `json:"domain"`
		Type         string              `json:"type"`
		Depth        int                 `json:"depth"`
		ParentChain  []string            `json:"parent_chain"`
		Facets       *atomization.Facets `json:"facets,omitempty"`
	}

	type ExcludedChunk struct {
		ChunkID      string `json:"chunk_id"`
		AtomID       string `json:"atom_id"`
		CanonicalRef string `json:"canonical_ref"`
		Reason       string `json:"reason"`
	}

	atomMap := map[string]*atomization.Atom{}
	for i := range set.Atoms {
		atomMap[set.Atoms[i].ID] = &set.Atoms[i]
	}

	chunks := make([]Chunk, 0, len(set.Atoms))
	excluded := make([]ExcludedChunk, 0)
	for _, a := range set.Atoms {
		if strings.TrimSpace(a.Text) == "" {
			continue
		}
		chunkID := "chunk:" + a.ID

		// CKM-02 lens scoping: a lens drops candidates whose facets fall outside
		// the retrieval scope, recording each drop with the lens's own reason.
		if lens != nil {
			var f atomization.Facets
			if a.Facets != nil {
				f = *a.Facets
			}
			decision := atomization.ApplyLens(f, *lens)
			if !decision.Included {
				excluded = append(excluded, ExcludedChunk{
					ChunkID:      chunkID,
					AtomID:       a.ID,
					CanonicalRef: a.CanonicalRef,
					Reason:       decision.Reason,
				})
				continue
			}
		}

		chain := resolveAtomParentChain(a.ID, atomMap)
		chunks = append(chunks, Chunk{
			ChunkID:      chunkID,
			AtomID:       a.ID,
			CanonicalRef: a.CanonicalRef,
			Text:         a.Text,
			ContentHash:  a.ContentHash,
			Domain:       a.Domain,
			Type:         string(a.Type),
			Depth:        a.Depth,
			ParentChain:  chain,
			Facets:       a.Facets,
		})
	}

	// Without a lens, preserve the exact prior shape (a bare chunk array) so the
	// default path stays byte-for-byte unchanged (zero regression).
	if lens == nil {
		return writeAtomJSON(stdout, stderr, chunks)
	}

	type ScopedChunks struct {
		LensID   string          `json:"lens_id"`
		Mode     string          `json:"mode"`
		Chunks   []Chunk         `json:"chunks"`
		Excluded []ExcludedChunk `json:"excluded"`
	}
	return writeAtomJSON(stdout, stderr, ScopedChunks{
		LensID:   lens.ID,
		Mode:     "lens",
		Chunks:   chunks,
		Excluded: excluded,
	})
}

// loadKnowledgeLens reads a single CKM-02 #KnowledgeLens document and validates
// that it names at least one predicate. A lens with neither include nor exclude
// would silently keep everything, which is almost certainly an operator mistake.
//
// The canonical lens shape (specs/knowledge-lens.cue) is authored in YAML, while
// the engine type (atomization.KnowledgeLens) is keyed by its JSON tags. We bridge
// the two by decoding YAML to a generic value and re-encoding it as JSON, so the
// file may be either YAML or JSON and the engine type stays free of YAML tags.
func loadKnowledgeLens(path string) (*atomization.KnowledgeLens, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	bridged, err := json.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("normalize %s: %w", path, err)
	}
	var lens atomization.KnowledgeLens
	if err := json.Unmarshal(bridged, &lens); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if lens.ID == "" {
		return nil, fmt.Errorf("%s: lens is missing an id", path)
	}
	if lens.Include == nil && lens.Exclude == nil {
		return nil, fmt.Errorf("%s: lens %q has no include/exclude predicate", path, lens.ID)
	}
	return &lens, nil
}

// --- validate: check atom set completeness ---

func atomizeValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize validate [--doc-ref REF] <file.md>")
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef: *docRef,
		SourceFile:  flags.Arg(0),
	})

	type ValidationResult struct {
		AtomCount   int      `json:"atom_count"`
		IsLossless  bool     `json:"is_lossless"`
		LossRatio   float64  `json:"loss_ratio"`
		UniqueIDs   bool     `json:"unique_ids"`
		OrphanAtoms []string `json:"orphan_atoms,omitempty"`
		Errors      []string `json:"errors,omitempty"`
		Valid       bool     `json:"valid"`
	}

	result := ValidationResult{
		AtomCount:  set.AtomCount,
		IsLossless: ast.LossReport.IsLossless,
		LossRatio:  ast.LossReport.LossRatio,
		UniqueIDs:  true,
		Valid:      true,
	}

	// Check unique IDs.
	seen := map[string]bool{}
	for _, a := range set.Atoms {
		if seen[a.ID] {
			result.UniqueIDs = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate atom ID: %s", a.ID))
			result.Valid = false
		}
		seen[a.ID] = true
	}

	// Check orphan atoms (parent not found).
	atomIDs := map[string]bool{}
	for _, a := range set.Atoms {
		atomIDs[a.ID] = true
	}
	for _, a := range set.Atoms {
		if a.ParentID != "" && !atomIDs[a.ParentID] {
			result.OrphanAtoms = append(result.OrphanAtoms, a.ID)
			result.Errors = append(result.Errors, fmt.Sprintf("orphan atom %s: parent %s not found", a.ID, a.ParentID))
			result.Valid = false
		}
	}

	if !ast.LossReport.IsLossless {
		result.Errors = append(result.Errors, fmt.Sprintf("content loss: %.1f%%", ast.LossReport.LossRatio*100))
		result.Valid = false
	}

	code := writeAtomJSON(stdout, stderr, result)
	if !result.Valid && code == 0 {
		return 1
	}
	return code
}

// --- certify: mark atoms as approved ---

func atomizeCertify(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize certify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	reviewer := flags.String("reviewer", "", "reviewer name (required)")
	state := flags.String("state", "approved", "target review state")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: nomos atomize certify --reviewer NAME [--state STATE] <file.md>")
		return 2
	}
	if *reviewer == "" {
		fmt.Fprintln(stderr, "error: --reviewer is required")
		return 2
	}

	targetState := atomization.ReviewState(*state)
	if !targetState.IsValid() {
		fmt.Fprintf(stderr, "error: invalid review state %q\n", *state)
		return 2
	}

	source, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read: %v\n", err)
		return 1
	}

	ast := atomization.ParseMarkdown(string(source))
	set := atomization.Atomize(ast, atomization.AtomizeOptions{
		DocumentRef:  *docRef,
		SourceFile:   flags.Arg(0),
		DefaultState: targetState,
	})

	type CertifyResult struct {
		AtomCount  int                `json:"atom_count"`
		Reviewer   string             `json:"reviewer"`
		State      string             `json:"state"`
		SourceHash string             `json:"source_hash"`
		Atoms      []atomization.Atom `json:"atoms"`
	}

	result := CertifyResult{
		AtomCount:  set.AtomCount,
		Reviewer:   *reviewer,
		State:      string(targetState),
		SourceHash: set.SourceHash,
		Atoms:      set.Atoms,
	}

	return writeAtomJSON(stdout, stderr, result)
}

// --- diff: compare two atom sets ---

func atomizeDiff(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("atomize diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	docRef := flags.String("doc-ref", "", "document reference slug")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: nomos atomize diff [--doc-ref REF] <old.md> <new.md>")
		return 2
	}

	oldSource, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read old: %v\n", err)
		return 1
	}
	newSource, err := os.ReadFile(flags.Arg(1))
	if err != nil {
		fmt.Fprintf(stderr, "read new: %v\n", err)
		return 1
	}

	oldAST := atomization.ParseMarkdown(string(oldSource))
	newAST := atomization.ParseMarkdown(string(newSource))

	oldSet := atomization.Atomize(oldAST, atomization.AtomizeOptions{DocumentRef: *docRef, SourceFile: flags.Arg(0)})
	newSet := atomization.Atomize(newAST, atomization.AtomizeOptions{DocumentRef: *docRef, SourceFile: flags.Arg(1)})

	type DiffEntry struct {
		AtomID       string `json:"atom_id"`
		CanonicalRef string `json:"canonical_ref"`
		Change       string `json:"change"` // added, removed, modified, unchanged
		OldHash      string `json:"old_hash,omitempty"`
		NewHash      string `json:"new_hash,omitempty"`
	}

	oldMap := map[string]*atomization.Atom{}
	for i := range oldSet.Atoms {
		oldMap[oldSet.Atoms[i].CanonicalRef] = &oldSet.Atoms[i]
	}
	newMap := map[string]*atomization.Atom{}
	for i := range newSet.Atoms {
		newMap[newSet.Atoms[i].CanonicalRef] = &newSet.Atoms[i]
	}

	var entries []DiffEntry

	for ref, newAtom := range newMap {
		oldAtom, existed := oldMap[ref]
		if !existed {
			entries = append(entries, DiffEntry{
				AtomID: newAtom.ID, CanonicalRef: ref, Change: "added",
				NewHash: newAtom.ContentHash,
			})
		} else if oldAtom.ContentHash != newAtom.ContentHash {
			entries = append(entries, DiffEntry{
				AtomID: newAtom.ID, CanonicalRef: ref, Change: "modified",
				OldHash: oldAtom.ContentHash, NewHash: newAtom.ContentHash,
			})
		} else {
			entries = append(entries, DiffEntry{
				AtomID: newAtom.ID, CanonicalRef: ref, Change: "unchanged",
			})
		}
	}
	for ref, oldAtom := range oldMap {
		if _, exists := newMap[ref]; !exists {
			entries = append(entries, DiffEntry{
				AtomID: oldAtom.ID, CanonicalRef: ref, Change: "removed",
				OldHash: oldAtom.ContentHash,
			})
		}
	}

	type DiffResult struct {
		OldFile   string      `json:"old_file"`
		NewFile   string      `json:"new_file"`
		Added     int         `json:"added"`
		Removed   int         `json:"removed"`
		Modified  int         `json:"modified"`
		Unchanged int         `json:"unchanged"`
		Entries   []DiffEntry `json:"entries"`
	}

	result := DiffResult{OldFile: flags.Arg(0), NewFile: flags.Arg(1)}
	for _, e := range entries {
		switch e.Change {
		case "added":
			result.Added++
		case "removed":
			result.Removed++
		case "modified":
			result.Modified++
		case "unchanged":
			result.Unchanged++
		}
	}
	result.Entries = entries

	code := writeAtomJSON(stdout, stderr, result)
	if (result.Added > 0 || result.Removed > 0 || result.Modified > 0) && code == 0 {
		return 1
	}
	return code
}

// --- helpers ---

func writeAtomJSON(stdout io.Writer, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "write json: %v\n", err)
		return 1
	}
	return 0
}

func resolveAtomParentChain(atomID string, atomMap map[string]*atomization.Atom) []string {
	var chain []string
	visited := map[string]bool{}
	current := atomID
	for {
		a, ok := atomMap[current]
		if !ok || a.ParentID == "" {
			break
		}
		if visited[a.ParentID] {
			break
		}
		visited[a.ParentID] = true
		chain = append([]string{a.ParentID}, chain...)
		current = a.ParentID
	}
	return chain
}
