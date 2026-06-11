package app

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/bundle"
)

// packCommand is `nomos pack`: the domain-pack gate (VRC-21 #564, doc 45 §5
// D2). `pack validate` makes the VRC-20 contract (specs/domain-pack.cue)
// EXECUTABLE: it re-checks the declarative shape in-engine, proves every
// declared artifact exists and coheres, RESOLVES the lens presets against the
// pack vocabulary, and runs the golden corpus through the real bundle chain.
// Every rung fails closed and names itself — a mutilated pack can never pass.
func packCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos pack validate --manifest <pack.yaml> [--repo-root <dir>] [--repo owner/name] [--commit sha]")
		return 2
	}
	switch args[0] {
	case "validate":
		return packValidateCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown pack subcommand %q (try: validate)\n", args[0])
		return 2
	}
}

// --- manifest mirror (strict: an out-of-contract field fails the decode) ----

type packManifest struct {
	SchemaVersion string `yaml:"schema_version"`
	PackID        string `yaml:"pack_id"`
	DomainProfile string `yaml:"domain_profile"`
	ProfileRef    string `yaml:"profile_ref"`
	ClaimBoundary string `yaml:"claim_boundary"`
	Vocabularies  struct {
		File string   `yaml:"file"`
		Axes []string `yaml:"axes"`
	} `yaml:"vocabularies"`
	Ontology struct {
		File string `yaml:"file"`
	} `yaml:"ontology"`
	SourceRegister struct {
		File     string `yaml:"file"`
		Contract string `yaml:"contract"`
	} `yaml:"source_register"`
	LensPresets []struct {
		ID   string `yaml:"id"`
		File string `yaml:"file"`
	} `yaml:"lens_presets"`
	GoldenCorpus struct {
		Root      string   `yaml:"root"`
		Documents []string `yaml:"documents"`
	} `yaml:"golden_corpus"`
	Scorecard []struct {
		Area   string `yaml:"area"`
		Status string `yaml:"status"`
		Note   string `yaml:"note"`
	} `yaml:"scorecard"`
}

type packVocabularyFile struct {
	RecordType     string            `yaml:"record_type"`
	SchemaVersion  string            `yaml:"schema_version"`
	DomainProfile  string            `yaml:"domain_profile"`
	Activity       []packVocabTerm   `yaml:"activity"`
	DisciplineRole []packVocabTerm   `yaml:"discipline_role"`
	References     map[string]string `yaml:"references"`
}

type packVocabTerm struct {
	ID      string `yaml:"id"`
	LabelFR string `yaml:"label_fr"`
}

// packOntologyFile mirrors the ckm-facet-ontology-v1 document (VRC-45, D4):
// the BFO→IOF→pack anchoring the gate renders its verdict on.
type packOntologyFile struct {
	SchemaVersion string `yaml:"schema_version"`
	FacetAxes     []struct {
		ID       string `yaml:"id"`
		Root     string `yaml:"root"`
		IOFClass string `yaml:"iof_class"`
		Terms    []struct {
			ID     string `yaml:"id"`
			MapsTo struct {
				BFO     string `yaml:"bfo"`
				IOFCore string `yaml:"iof_core"`
			} `yaml:"maps_to"`
		} `yaml:"terms"`
	} `yaml:"facet_axes"`
	Orthogonality struct {
		OWLConstruct string   `yaml:"owl_construct"`
		DisjointAxes []string `yaml:"disjoint_axes"`
	} `yaml:"orthogonality"`
	ClaimBoundary string `yaml:"claim_boundary"`
}

var (
	packIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// The positive declarative allowlist — the Go mirror of #DeclarativePath.
	// Code cannot match it; there is no negative list to bypass.
	packDeclarativeRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+\.(yaml|yml|md|json)$`)
	packLocalRe       = regexp.MustCompile(`^docs/regulated/domain-packs/`)
	packLensFileRe    = regexp.MustCompile(`\.lens\.(yaml|yml)$`)
	packCorpusRootRe  = regexp.MustCompile(`^(docs/regulated/domain-packs|cli/internal/corpus/testdata)/[A-Za-z0-9._/-]+$`)
	packCorpusDocRe   = regexp.MustCompile(`^[A-Za-z0-9._-]+\.(md|xml|html|pdf|yaml|json|txt)$`)
	packLensIDRe      = regexp.MustCompile(`^LENS-[A-Z0-9-]+$`)
	packTermRe        = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
)

var packOpenAxes = map[string]bool{"activity": true, "discipline_role": true}

var packScorecardStatuses = map[string]bool{
	"applicable": true, "partial": true, "out_of_scope": true, "blocked": true,
}

func packFail(stderr io.Writer, rung, format string, a ...any) int {
	fmt.Fprintf(stderr, "pack validate: FAIL [%s]: %s\n", rung, fmt.Sprintf(format, a...))
	return 1
}

func packValidateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("pack validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "pack manifest path (required)")
	repoRoot := flags.String("repo-root", ".", "repository root the manifest paths are relative to")
	repo := flags.String("repo", "", "override trace corpus repo (owner/name); default: git origin of --repo-root")
	commit := flags.String("commit", "", "override trace corpus commit sha; default: git HEAD of --repo-root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" {
		fmt.Fprintln(stderr, "pack validate: --manifest is required")
		return 2
	}

	// Rung 1 — manifest: strict parse (unknown fields = mechanics smuggled
	// around the contract → fail), exact schema, well-formed identifiers.
	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		return packFail(stderr, "manifest", "read %s: %v", *manifestPath, err)
	}
	var m packManifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return packFail(stderr, "manifest", "strict parse %s: %v", *manifestPath, err)
	}
	if m.SchemaVersion != "nomos-domain-pack-v1" {
		return packFail(stderr, "manifest", "schema_version %q is not nomos-domain-pack-v1", m.SchemaVersion)
	}
	if !packIDRe.MatchString(m.PackID) || !packIDRe.MatchString(m.DomainProfile) {
		return packFail(stderr, "manifest", "pack_id %q / domain_profile %q must match %s", m.PackID, m.DomainProfile, packIDRe)
	}

	// Rung 2 — declarative: every declared path matches the positive
	// allowlist (the Go mirror of #DeclarativePath — code cannot be named).
	declared := map[string]string{
		"profile_ref":          m.ProfileRef,
		"vocabularies.file":    m.Vocabularies.File,
		"source_register.file": m.SourceRegister.File,
		"ontology.file":        m.Ontology.File,
	}
	for field, p := range declared {
		if !packDeclarativeRe.MatchString(p) || strings.Contains(p, "..") {
			return packFail(stderr, "declarative", "%s %q is not a declarative artifact path (yaml|yml|md|json)", field, p)
		}
	}
	for _, ref := range []string{m.Vocabularies.File, m.SourceRegister.File, m.Ontology.File} {
		if !packLocalRe.MatchString(ref) {
			return packFail(stderr, "declarative", "%q must live under docs/regulated/domain-packs/", ref)
		}
	}
	if len(m.Vocabularies.Axes) == 0 {
		return packFail(stderr, "declarative", "vocabularies.axes is empty — a pack provides at least one open axis")
	}
	for _, axis := range m.Vocabularies.Axes {
		if !packOpenAxes[axis] {
			return packFail(stderr, "declarative", "axis %q is not an open-term axis (packs own TERMS; core owns AXES)", axis)
		}
	}
	if len(m.LensPresets) == 0 {
		return packFail(stderr, "declarative", "lens_presets is empty — a pack ships at least one activable view")
	}
	for _, preset := range m.LensPresets {
		if !packLensIDRe.MatchString(preset.ID) {
			return packFail(stderr, "declarative", "preset id %q must match %s", preset.ID, packLensIDRe)
		}
		if !packDeclarativeRe.MatchString(preset.File) || !packLocalRe.MatchString(preset.File) || !packLensFileRe.MatchString(preset.File) {
			return packFail(stderr, "declarative", "preset file %q is not a pack-local .lens.yaml", preset.File)
		}
	}
	if !packCorpusRootRe.MatchString(m.GoldenCorpus.Root) || strings.Contains(m.GoldenCorpus.Root, "..") {
		return packFail(stderr, "declarative", "golden_corpus.root %q is outside the allowed trees", m.GoldenCorpus.Root)
	}
	if len(m.GoldenCorpus.Documents) == 0 {
		return packFail(stderr, "declarative", "golden_corpus.documents is empty — the proof material is not optional")
	}
	for _, doc := range m.GoldenCorpus.Documents {
		if !packCorpusDocRe.MatchString(doc) {
			return packFail(stderr, "declarative", "golden corpus document %q is not a declarative data file", doc)
		}
	}
	if len(m.Scorecard) == 0 {
		return packFail(stderr, "declarative", "scorecard is empty — the honest perimeter is not optional")
	}
	for i, row := range m.Scorecard {
		if strings.TrimSpace(row.Area) == "" || strings.TrimSpace(row.Note) == "" || !packScorecardStatuses[row.Status] {
			return packFail(stderr, "declarative", "scorecard row %d is incomplete or has status %q", i, row.Status)
		}
	}

	// Rung 3 — claim boundary: the honest statement is present, not a blank.
	if strings.TrimSpace(m.ClaimBoundary) == "" {
		return packFail(stderr, "claim_boundary", "claim_boundary is absent — a pack without a boundary claims everything")
	}

	// Rung 4 — artifacts: everything the manifest names exists on disk.
	abs := func(rel string) string { return filepath.Join(*repoRoot, filepath.FromSlash(rel)) }
	for field, p := range declared {
		if _, err := os.Stat(abs(p)); err != nil {
			return packFail(stderr, "artifacts", "%s: %s does not exist: %v", field, p, err)
		}
	}
	for _, preset := range m.LensPresets {
		if _, err := os.Stat(abs(preset.File)); err != nil {
			return packFail(stderr, "artifacts", "preset %s: %s does not exist: %v", preset.ID, preset.File, err)
		}
	}
	corpusRoot := abs(m.GoldenCorpus.Root)
	if info, err := os.Stat(corpusRoot); err != nil || !info.IsDir() {
		return packFail(stderr, "artifacts", "golden_corpus.root %s is not a directory", m.GoldenCorpus.Root)
	}

	// Rung 5 — vocabulary: declared axes carry at least one namespaced term.
	vocabRaw, err := os.ReadFile(abs(m.Vocabularies.File))
	if err != nil {
		return packFail(stderr, "vocabulary", "read %s: %v", m.Vocabularies.File, err)
	}
	var vocab packVocabularyFile
	if err := yaml.Unmarshal(vocabRaw, &vocab); err != nil {
		return packFail(stderr, "vocabulary", "parse %s: %v", m.Vocabularies.File, err)
	}
	vocabTerms := map[string]map[string]bool{
		"activity":        {},
		"discipline_role": {},
	}
	for _, t := range vocab.Activity {
		vocabTerms["activity"][t.ID] = true
	}
	for _, t := range vocab.DisciplineRole {
		vocabTerms["discipline_role"][t.ID] = true
	}
	totalTerms := 0
	for _, axis := range m.Vocabularies.Axes {
		terms := vocabTerms[axis]
		if len(terms) == 0 {
			return packFail(stderr, "vocabulary", "axis %q is declared but %s carries no terms for it", axis, m.Vocabularies.File)
		}
		for id := range terms {
			if !packTermRe.MatchString(id) {
				return packFail(stderr, "vocabulary", "term %q on axis %q is not pack-namespaced (want e.g. aec.conception)", id, axis)
			}
		}
		totalTerms += len(terms)
	}

	// Rung 5b — ontology (VRC-45, D4): the gate renders the verdict on the
	// pack's BFO→IOF→pack alignment. Three failure modes, each rejected:
	// an open axis the pack declares but the ontology never registers, a
	// vocabulary term with no (bfo, iof_core) mapping, and a term appearing
	// on two owl:disjointUnionOf axes.
	ontoRaw, err := os.ReadFile(abs(m.Ontology.File))
	if err != nil {
		return packFail(stderr, "ontology", "read %s: %v", m.Ontology.File, err)
	}
	var onto packOntologyFile
	if err := yaml.Unmarshal(ontoRaw, &onto); err != nil {
		return packFail(stderr, "ontology", "parse %s: %v", m.Ontology.File, err)
	}
	if onto.SchemaVersion != "ckm-facet-ontology-v1" {
		return packFail(stderr, "ontology", "schema_version %q is not ckm-facet-ontology-v1", onto.SchemaVersion)
	}
	if onto.Orthogonality.OWLConstruct != "owl:disjointUnionOf" {
		return packFail(stderr, "ontology", "orthogonality.owl_construct must be owl:disjointUnionOf, got %q", onto.Orthogonality.OWLConstruct)
	}
	ontoAxes := map[string]map[string]bool{}
	for _, axis := range onto.FacetAxes {
		if strings.TrimSpace(axis.Root) == "" || strings.TrimSpace(axis.IOFClass) == "" {
			return packFail(stderr, "ontology", "axis %q has no BFO root or IOF class — not aligned", axis.ID)
		}
		terms := map[string]bool{}
		for _, term := range axis.Terms {
			if strings.TrimSpace(term.MapsTo.BFO) == "" || strings.TrimSpace(term.MapsTo.IOFCore) == "" {
				return packFail(stderr, "ontology", "term %q on axis %q lacks a (bfo, iof_core) mapping", term.ID, axis.ID)
			}
			terms[term.ID] = true
		}
		ontoAxes[axis.ID] = terms
	}
	for _, axis := range m.Vocabularies.Axes {
		registered, ok := ontoAxes[axis]
		if !ok {
			return packFail(stderr, "ontology", "pack axis %q is not registered in %s — axe non aligné", axis, m.Ontology.File)
		}
		for id := range vocabTerms[axis] {
			if !registered[id] {
				return packFail(stderr, "ontology", "vocabulary term %q (axis %q) has no ontology mapping — terme non aligné", id, axis)
			}
		}
	}
	seenOnDisjoint := map[string]string{}
	for _, axis := range onto.Orthogonality.DisjointAxes {
		for id := range ontoAxes[axis] {
			if other, dup := seenOnDisjoint[id]; dup {
				return packFail(stderr, "ontology", "term %q sits on disjoint axes %q and %q — owl:disjointUnionOf violated", id, other, axis)
			}
			seenOnDisjoint[id] = axis
		}
	}

	// Rung 6 — source register: the authority register names this pack.
	var register struct {
		DomainProfile string `yaml:"domain_profile"`
	}
	registerRaw, err := os.ReadFile(abs(m.SourceRegister.File))
	if err != nil {
		return packFail(stderr, "source_register", "read %s: %v", m.SourceRegister.File, err)
	}
	if err := yaml.Unmarshal(registerRaw, &register); err != nil {
		return packFail(stderr, "source_register", "parse %s: %v", m.SourceRegister.File, err)
	}
	if register.DomainProfile != m.DomainProfile {
		return packFail(stderr, "source_register", "register domain_profile %q does not match pack %q", register.DomainProfile, m.DomainProfile)
	}

	// Rung 7 — lens presets: each loads as a usable KnowledgeLens, its id
	// matches the manifest, and every open-axis term it references RESOLVES
	// in the pack vocabulary (a preset selecting a term the pack never
	// defined would silently select nothing — that is a broken preset).
	for _, preset := range m.LensPresets {
		lens, err := loadPackLensFile(abs(preset.File))
		if err != nil {
			return packFail(stderr, "lens_presets", "%s: %v", preset.File, err)
		}
		if lens.ID != preset.ID {
			return packFail(stderr, "lens_presets", "%s declares id %q but the manifest says %q", preset.File, lens.ID, preset.ID)
		}
		if lens.Include == nil && lens.Exclude == nil {
			return packFail(stderr, "lens_presets", "%s has neither include nor exclude — not a usable lens", preset.File)
		}
		for axis, terms := range lensOpenAxisTerms(lens) {
			for _, term := range terms {
				if !vocabTerms[axis][term] {
					return packFail(stderr, "lens_presets", "%s references %s term %q absent from the pack vocabulary", preset.File, axis, term)
				}
			}
		}
	}

	// Rung 8 — golden corpus: the declared documents ride the REAL bundle
	// chain (atomization → faceted nodes → emitted bundle). Every document
	// must contribute at least one node; a document the chain cannot turn
	// into citable knowledge is a red corpus.
	sources := make([]bundle.SourceFile, 0, len(m.GoldenCorpus.Documents))
	for _, doc := range m.GoldenCorpus.Documents {
		content, err := os.ReadFile(filepath.Join(corpusRoot, doc))
		if err != nil {
			return packFail(stderr, "golden_corpus", "read %s/%s: %v", m.GoldenCorpus.Root, doc, err)
		}
		sources = append(sources, bundle.SourceFile{RelPath: doc, Content: content})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].RelPath < sources[j].RelPath })

	gitCtx := bundle.TraceGitContext{
		Repo:   firstNonEmptyStr(*repo, bundle.ParseRepoFromRemote(gitValue(*repoRoot, "config", "--get", "remote.origin.url"))),
		Branch: gitValue(*repoRoot, "rev-parse", "--abbrev-ref", "HEAD"),
		Commit: firstNonEmptyStr(*commit, gitValue(*repoRoot, "rev-parse", "HEAD")),
	}
	now := time.Now().UTC()
	trace, err := bundle.NewTraceManifest(gitCtx, now.Format(time.RFC3339), m.PackID+"-golden", "", "", nil)
	if err != nil {
		return packFail(stderr, "golden_corpus", "trace context: %v", err)
	}
	b, err := bundle.Build(bundle.BuildInput{
		BundleID:    m.PackID + "-golden",
		Producer:    "nomos-pack-validate",
		Domain:      m.DomainProfile,
		GeneratedAt: now,
		Sources:     sources,
		Trace:       trace,
	})
	if err != nil {
		return packFail(stderr, "golden_corpus", "bundle chain failed on the golden corpus: %v", err)
	}
	nodesBySource := map[string]int{}
	totalNodes := 0
	for _, feed := range b.Feeds {
		for _, node := range feed.Nodes {
			nodesBySource[node.SourcePath]++
			totalNodes++
		}
	}
	for _, doc := range m.GoldenCorpus.Documents {
		if nodesBySource[doc] == 0 {
			return packFail(stderr, "golden_corpus", "document %s produced zero nodes — red corpus", doc)
		}
	}
	if _, err := b.Marshal(); err != nil {
		return packFail(stderr, "golden_corpus", "bundle marshal: %v", err)
	}

	fmt.Fprintf(stdout,
		"pack validate: OK — %s: %d axe(s)/%d terme(s) alignés BFO→IOF, %d preset(s) résolus, corpus doré %d doc(s) → %d node(s), scorecard %d ligne(s)\n",
		m.PackID, len(m.Vocabularies.Axes), totalTerms, len(m.LensPresets),
		len(m.GoldenCorpus.Documents), totalNodes, len(m.Scorecard))
	return 0
}

// loadPackLensFile bridges a YAML lens preset into the engine's KnowledgeLens
// (the same yaml→json normalization the atomize command applies).
func loadPackLensFile(path string) (atomization.KnowledgeLens, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return atomization.KnowledgeLens{}, fmt.Errorf("read: %w", err)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return atomization.KnowledgeLens{}, fmt.Errorf("parse: %w", err)
	}
	bridged, err := json.Marshal(normalizeYAML(generic))
	if err != nil {
		return atomization.KnowledgeLens{}, fmt.Errorf("normalize: %w", err)
	}
	var lens atomization.KnowledgeLens
	if err := json.Unmarshal(bridged, &lens); err != nil {
		return atomization.KnowledgeLens{}, fmt.Errorf("decode: %w", err)
	}
	if lens.ID == "" {
		return atomization.KnowledgeLens{}, fmt.Errorf("lens has no id")
	}
	return lens, nil
}

// normalizeYAML converts yaml.v3's map[any]any trees into json-encodable
// map[string]any trees.
func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprintf("%v", k)] = normalizeYAML(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeYAML(val)
		}
		return out
	case []any:
		for i := range t {
			t[i] = normalizeYAML(t[i])
		}
		return t
	default:
		return v
	}
}

// lensOpenAxisTerms collects every open-axis term a lens references across
// its include/exclude predicates, keyed by axis.
func lensOpenAxisTerms(lens atomization.KnowledgeLens) map[string][]string {
	out := map[string][]string{}
	collect := func(p *atomization.LensPredicate) {
		if p == nil {
			return
		}
		for _, group := range [][]atomization.LensFacetSelection{p.AllOf, p.AnyOf, p.NoneOf} {
			for _, sel := range group {
				out["activity"] = append(out["activity"], sel.Activity...)
				out["discipline_role"] = append(out["discipline_role"], sel.DisciplineRole...)
			}
		}
	}
	collect(lens.Include)
	collect(lens.Exclude)
	return out
}
