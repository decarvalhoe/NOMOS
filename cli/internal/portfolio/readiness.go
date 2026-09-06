package portfolio

// NRT-028 (#681): the v1.0 readiness verdict, COMPUTED from the tree. Each of
// the eight docs/14 "Definition Of v1.0" criteria is mapped to machine checks;
// the verdict is `ready` only when every check of every criterion holds, and
// `not_ready` otherwise with every unmet check named. There is no `released`:
// the release is a human decision recorded on the regulated lane (#561).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/contracts"
)

const (
	ReadinessSchema        = "nomos-release-readiness-v1"
	ReadinessClaimBoundary = "v1.0 readiness computed from committed machine sources against the docs/14 definition. " +
		"`ready` means every mapped check holds on this tree; it is not a release, not an approval, and not a validated-use, " +
		"QMS-effectiveness or regulated-readiness claim (regulated lane, #561)."
	VerdictReady    = "ready"
	VerdictNotReady = "not_ready"
)

// Check is one machine check under a criterion.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Criterion is one docs/14 line with its checks.
type Criterion struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Met    bool    `json:"met"`
	Checks []Check `json:"checks"`
}

// Readiness is the verdict document.
type Readiness struct {
	SchemaVersion   string      `json:"schema_version"`
	GeneratedAt     string      `json:"generated_at"`
	RepoRoot        string      `json:"repo_root"`
	CoreVersion     string      `json:"core_version"`
	Verdict         string      `json:"verdict"`
	StatusDigest    string      `json:"status_digest"`
	Criteria        []Criterion `json:"criteria"`
	Unmet           []string    `json:"unmet"`
	ReadinessDigest string      `json:"readiness_digest"`
	ClaimBoundary   string      `json:"claim_boundary"`
}

// ReadinessOptions parametrise the computation.
type ReadinessOptions struct {
	RepoRoot    string
	Now         time.Time
	CoreVersion string
	// ClaimGuard runs the public-claim guard (default: python3 scripts/claim_boundary_guard.py). Tests inject.
	ClaimGuard func(root string) error
}

// readinessInputs is everything the criteria evaluate — gathered from the tree,
// then judged by evaluate(), which is pure and unit-tested per criterion.
type readinessInputs struct {
	RegistryErr            error
	StableWithoutCompat    []string
	MatrixRows             map[string]matrixRow
	MatrixErr              error
	AdaptersErr            error
	AdapterCount           int
	AdapterFixturesMissing []string
	UnsupportedEngine      bool
	RoadmapErr             error
	ToolsMissingFields     []string
	ClosedWithoutTool      []string
	LedgerErr              error
	LedgerStatus           string
	LedgerSchemaVersion    string
	LedgerGeneratedAt      string
	LedgerStale            bool
	SecurityProcess        string // schema_version found, "" when absent
	SupportModel           string
	ClaimGuardErr          error
	ClaimGuardRan          bool
}

type matrixRow struct {
	Expected string `json:"expected"`
	Computed string `json:"computed"`
	Mismatch bool   `json:"mismatch"`
}

func (in readinessInputs) real(id string) Check {
	r, ok := in.MatrixRows[id]
	switch {
	case in.MatrixErr != nil:
		return Check{Name: "matrix:" + id, Detail: "wiring matrix unavailable: " + in.MatrixErr.Error()}
	case !ok:
		return Check{Name: "matrix:" + id, Detail: "capability " + id + " is not in the wiring matrix"}
	case r.Mismatch || r.Computed != "real":
		return Check{Name: "matrix:" + id, Detail: fmt.Sprintf("capability %s computed %q (expected %q, mismatch=%v); must be real", id, r.Computed, r.Expected, r.Mismatch)}
	}
	return Check{Name: "matrix:" + id, OK: true, Detail: "real"}
}

func okCheck(name, detail string) Check { return Check{Name: name, OK: true, Detail: detail} }
func koCheck(name, detail string) Check { return Check{Name: name, OK: false, Detail: detail} }

// evaluate maps the eight docs/14 criteria to the gathered inputs.
func evaluate(in readinessInputs) []Criterion {
	var cs []Criterion
	add := func(id, title string, checks ...Check) {
		met := true
		for _, c := range checks {
			met = met && c.OK
		}
		cs = append(cs, Criterion{ID: id, Title: title, Met: met, Checks: checks})
	}
	// 1
	reg := okCheck("contract-registry", "every contract registered, hashes at version, fixtures and compatibility reads verified")
	if in.RegistryErr != nil {
		reg = koCheck("contract-registry", in.RegistryErr.Error())
	}
	compat := okCheck("stable-contracts-compat-fixtures", "every stable contract has a compatibility fixture read by its Go reader")
	if len(in.StableWithoutCompat) > 0 {
		compat = koCheck("stable-contracts-compat-fixtures", "stable contracts without a compatibility fixture: "+strings.Join(in.StableWithoutCompat, ", "))
	}
	add("C1", "admitted corpus scopes are explicit and reproducible", reg, compat, in.real("external_snapshot_input"), in.real("recursio_offline_e2e"))
	// 2
	eng := okCheck("unsupported-formats-engine", "cli/internal/fidelity/unsupported_formats.go present")
	if !in.UnsupportedEngine {
		eng = koCheck("unsupported-formats-engine", "cli/internal/fidelity/unsupported_formats.go is missing")
	}
	add("C2", "unsupported source structures become explicit evidence records, not silent gaps", in.real("strict_fidelity_gate"), eng)
	// 3
	add("C3", "source spans and document hierarchy are independently checkable", in.real("body_ledger_merkle_emission"), in.real("body_ledger_merkle_verification"), in.real("cross_reference_graph"))
	// 4
	ad := okCheck("adapters-compatible", fmt.Sprintf("%d adapter manifest(s) inside the declared core and schema ranges", in.AdapterCount))
	if in.AdaptersErr != nil {
		ad = koCheck("adapters-compatible", in.AdaptersErr.Error())
	} else if in.AdapterCount == 0 {
		ad = koCheck("adapters-compatible", "no adapter manifest found under adapters/*/adapter.nomos.yaml")
	}
	fx := okCheck("adapter-manifests-are-fixtures", "every adapter manifest is a registered valid fixture of adapter-manifest")
	if len(in.AdapterFixturesMissing) > 0 {
		fx = koCheck("adapter-manifests-are-fixtures", "adapter manifests not registered as fixtures: "+strings.Join(in.AdapterFixturesMissing, ", "))
	}
	add("C4", "adapters publish compatibility contracts and fixtures", ad, fx, in.real("adapter_capability_kits"), in.real("compatibility_matrix"))
	// 5
	add("C5", "generated chunks, matrices, reports and attestations are reconstructible", in.real("canonical_knowledge_bundle"), in.real("release_candidate_bundle"), in.real("claim_coverage_attestation"))
	// 6
	tools := okCheck("regulated-tools-declared", "every regulated_tool declares intended_use, validation_state and reliance")
	if in.RoadmapErr != nil {
		tools = koCheck("regulated-tools-declared", in.RoadmapErr.Error())
	} else if len(in.ToolsMissingFields) > 0 {
		tools = koCheck("regulated-tools-declared", "incomplete regulated_tool on: "+strings.Join(in.ToolsMissingFields, ", "))
	}
	closed := okCheck("closed-items-have-tool", "every closed roadmap item carries a regulated_tool block")
	if in.RoadmapErr == nil && len(in.ClosedWithoutTool) > 0 {
		closed = koCheck("closed-items-have-tool", "closed items without regulated_tool: "+strings.Join(in.ClosedWithoutTool, ", "))
	}
	add("C6", "evidence-support tooling declares intended use, validation state and reliance boundary", tools, closed)
	// 7
	led := okCheck("evidence-ledger", fmt.Sprintf("schema_version %s, status %s, dated %s", in.LedgerSchemaVersion, in.LedgerStatus, in.LedgerGeneratedAt))
	switch {
	case in.LedgerErr != nil:
		led = koCheck("evidence-ledger", in.LedgerErr.Error())
	case in.LedgerSchemaVersion == "":
		led = koCheck("evidence-ledger", "the evidence ledger carries no schema_version")
	case in.LedgerStatus == "draft" || in.LedgerStatus == "":
		led = koCheck("evidence-ledger", fmt.Sprintf("the evidence ledger status is %q; versioned product evidence requires an effective ledger", in.LedgerStatus))
	case in.LedgerStale:
		led = koCheck("evidence-ledger", "the evidence ledger dated "+in.LedgerGeneratedAt+" is stale under the portfolio freshness policy")
	}
	sec := okCheck("security-process", in.SecurityProcess)
	if in.SecurityProcess == "" {
		sec = koCheck("security-process", "docs/security/security-process.yaml is absent or carries no schema_version")
	}
	sup := okCheck("support-model", in.SupportModel)
	if in.SupportModel == "" {
		sup = koCheck("support-model", "docs/support-model.yaml is absent or carries no schema_version")
	}
	add("C7", "regulated documentation consumes versioned product evidence without becoming a product implementation dependency", led, sec, sup)
	// 8
	cg := okCheck("claim-boundary-guard", "public-claim guard green")
	switch {
	case !in.ClaimGuardRan:
		cg = koCheck("claim-boundary-guard", "the public-claim guard could not be executed here; a verdict without it is not_ready")
	case in.ClaimGuardErr != nil:
		cg = koCheck("claim-boundary-guard", in.ClaimGuardErr.Error())
	}
	add("C8", "public claims never exceed the current evidence level", cg)
	return cs
}

func defaultClaimGuard(root string) error {
	py, err := exec.LookPath("python3")
	if err != nil {
		return fmt.Errorf("python3 not found: %w", err)
	}
	cmd := exec.Command(py, filepath.Join(root, "scripts", "claim_boundary_guard.py"), "--root", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 4 {
			lines = lines[len(lines)-4:]
		}
		return fmt.Errorf("claim guard red: %s", strings.Join(lines, " | "))
	}
	return nil
}

var errGuardNotRun = fmt.Errorf("guard not executed")

// gather reads every input from the tree. It never panics on a missing source:
// the absence becomes the named reason.
func gather(opts ReadinessOptions) readinessInputs {
	root := opts.RepoRoot
	var in readinessInputs
	// contract registry
	reg, err := contracts.Load(root)
	if err != nil {
		in.RegistryErr = err
	} else if _, err := contracts.Verify(root, opts.Now); err != nil {
		in.RegistryErr = err
	}
	if err == nil {
		registered := map[string]bool{}
		for _, c := range reg.Contracts {
			if c.Stability == "stable" && len(c.CompatFixtures) == 0 {
				in.StableWithoutCompat = append(in.StableWithoutCompat, c.ID)
			}
			if c.ID == "adapter-manifest" {
				for _, f := range c.Fixtures.Valid {
					registered[f] = true
				}
			}
		}
		sort.Strings(in.StableWithoutCompat)
		manifests, _ := filepath.Glob(filepath.Join(root, "adapters", "*", "adapter.nomos.yaml"))
		for _, m := range manifests {
			rel := filepath.ToSlash(strings.TrimPrefix(m, filepath.Clean(root)+string(os.PathSeparator)))
			if !registered[rel] {
				in.AdapterFixturesMissing = append(in.AdapterFixturesMissing, rel)
			}
		}
		adapters, aerr := contracts.CheckAdapters(root, opts.CoreVersion, reg)
		in.AdapterCount, in.AdaptersErr = len(adapters), aerr
	} else {
		in.AdaptersErr = fmt.Errorf("contract registry unavailable: %v", err)
	}
	// wiring matrix
	in.MatrixRows = map[string]matrixRow{}
	raw, err := os.ReadFile(filepath.Join(root, ".vrc-wiring-matrix", "wiring-matrix.json"))
	if err != nil {
		in.MatrixErr = err
	} else {
		var doc struct {
			Capabilities []struct {
				ID string `json:"id"`
				matrixRow
			} `json:"capabilities"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			in.MatrixErr = err
		}
		for _, c := range doc.Capabilities {
			in.MatrixRows[c.ID] = c.matrixRow
		}
	}
	_, err = os.Stat(filepath.Join(root, "cli", "internal", "fidelity", "unsupported_formats.go"))
	in.UnsupportedEngine = err == nil
	// roadmap lanes
	raw, err = os.ReadFile(filepath.Join(root, "docs", "roadmap-lanes.yaml"))
	if err != nil {
		in.RoadmapErr = err
	} else {
		var doc struct {
			Items []struct {
				Issue         int    `yaml:"issue"`
				State         string `yaml:"state"`
				Lane          string `yaml:"lane"`
				RegulatedTool *struct {
					IntendedUse     string `yaml:"intended_use"`
					ValidationState string `yaml:"validation_state"`
					Reliance        string `yaml:"reliance"`
				} `yaml:"regulated_tool"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			in.RoadmapErr = err
		}
		for _, it := range doc.Items {
			ref := fmt.Sprintf("#%d", it.Issue)
			if it.RegulatedTool == nil {
				if it.State == "closed" {
					in.ClosedWithoutTool = append(in.ClosedWithoutTool, ref)
				}
				continue
			}
			t := it.RegulatedTool
			if strings.TrimSpace(t.IntendedUse) == "" || strings.TrimSpace(t.ValidationState) == "" || strings.TrimSpace(t.Reliance) == "" {
				in.ToolsMissingFields = append(in.ToolsMissingFields, ref)
			}
		}
	}
	// evidence ledger
	raw, err = os.ReadFile(filepath.Join(root, "docs", "regulated", "evidence-index", "evidence-ledger.yaml"))
	if err != nil {
		in.LedgerErr = err
	} else {
		var doc struct {
			SchemaVersion string `yaml:"schema_version"`
			GeneratedAt   string `yaml:"generated_at"`
			Status        string `yaml:"status"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			in.LedgerErr = err
		}
		in.LedgerSchemaVersion, in.LedgerStatus, in.LedgerGeneratedAt = doc.SchemaVersion, doc.Status, doc.GeneratedAt
		if d, perr := time.Parse("2006-01-02", strings.TrimSpace(doc.GeneratedAt)); perr == nil {
			in.LedgerStale = opts.Now.Sub(d) > time.Duration(DefaultStaleAfterDays)*24*time.Hour
		} else {
			in.LedgerStale = true
		}
	}
	in.SecurityProcess = schemaVersionOf(filepath.Join(root, "docs", "security", "security-process.yaml"))
	in.SupportModel = schemaVersionOf(filepath.Join(root, "docs", "support-model.yaml"))
	// claim guard
	guard := opts.ClaimGuard
	if guard == nil {
		guard = defaultClaimGuard
	}
	err = guard(root)
	if err == errGuardNotRun {
		in.ClaimGuardRan = false
	} else {
		in.ClaimGuardRan, in.ClaimGuardErr = true, err
	}
	return in
}

func schemaVersionOf(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		SchemaVersion string `yaml:"schema_version"`
	}
	if yaml.Unmarshal(raw, &doc) != nil {
		return ""
	}
	return strings.TrimSpace(doc.SchemaVersion)
}

// ComputeReadiness computes the verdict and binds it to the portfolio status digest.
func ComputeReadiness(opts ReadinessOptions) (Readiness, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	st, err := Compute(Options{RepoRoot: opts.RepoRoot, Now: opts.Now})
	if err != nil {
		return Readiness{}, fmt.Errorf("portfolio status: %w", err)
	}
	in := gather(opts)
	r := Readiness{SchemaVersion: ReadinessSchema, GeneratedAt: opts.Now.UTC().Format(time.RFC3339), RepoRoot: opts.RepoRoot, CoreVersion: opts.CoreVersion,
		StatusDigest: st.StatusDigest, Criteria: evaluate(in), ClaimBoundary: ReadinessClaimBoundary}
	r.Verdict, r.Unmet = verdictOf(r.Criteria)
	r.ReadinessDigest = digestOf(r)
	return r, nil
}

func verdictOf(cs []Criterion) (string, []string) {
	unmet := []string{}
	for _, c := range cs {
		for _, ch := range c.Checks {
			if !ch.OK {
				unmet = append(unmet, c.ID+"/"+ch.Name+": "+ch.Detail)
			}
		}
	}
	if len(unmet) == 0 {
		return VerdictReady, unmet
	}
	return VerdictNotReady, unmet
}

// digestOf hashes the document without its own digest and repo_root (path-independent).
func digestOf(r Readiness) string {
	clone := r
	clone.ReadinessDigest, clone.RepoRoot = "", ""
	raw, _ := json.Marshal(clone)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// VerifyReadinessFile re-reads a verdict and refuses what does not hold: an
// unknown schema or verdict, a digest that does not match the content, a
// `ready` whose criteria are not all met (forged), an unmet list that does not
// match the checks, and — when root is given — a status digest that is not the
// one the tree computes now (stale or foreign).
func VerifyReadinessFile(path, root string, now time.Time) (Readiness, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Readiness{}, err
	}
	var r Readiness
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("readiness: %v", err)
	}
	if r.SchemaVersion != ReadinessSchema {
		return r, fmt.Errorf("readiness: schema_version %q, want %s", r.SchemaVersion, ReadinessSchema)
	}
	if r.Verdict != VerdictReady && r.Verdict != VerdictNotReady {
		return r, fmt.Errorf("readiness: verdict %q is not ready|not_ready — there is no other verdict, in particular no `released`", r.Verdict)
	}
	if digestOf(r) != r.ReadinessDigest {
		return r, fmt.Errorf("readiness: readiness_digest does not match the content — the file was edited after it was computed")
	}
	verdict, unmet := verdictOf(r.Criteria)
	if verdict != r.Verdict {
		return r, fmt.Errorf("readiness: verdict %q contradicts its own criteria (%d unmet check(s)) — forged", r.Verdict, len(unmet))
	}
	if strings.Join(unmet, "\n") != strings.Join(r.Unmet, "\n") {
		return r, fmt.Errorf("readiness: the unmet list does not match the checks")
	}
	if len(r.Criteria) != 8 {
		return r, fmt.Errorf("readiness: %d criteria, docs/14 defines 8", len(r.Criteria))
	}
	if root != "" {
		st, err := Compute(Options{RepoRoot: root, Now: now})
		if err != nil {
			return r, fmt.Errorf("readiness: recompute portfolio status: %v", err)
		}
		if st.StatusDigest != r.StatusDigest {
			return r, fmt.Errorf("readiness: bound to portfolio status %s but the tree now computes %s — recompute", r.StatusDigest[:19], st.StatusDigest[:19])
		}
	}
	return r, nil
}

// RenderReadinessMarkdown is a short human view.
func RenderReadinessMarkdown(r Readiness) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# v1.0 readiness — %s\n\nCore %s · generated %s · bound to portfolio status `%s`\n\n", strings.ToUpper(r.Verdict), r.CoreVersion, r.GeneratedAt, r.StatusDigest)
	for _, c := range r.Criteria {
		mark := "MET"
		if !c.Met {
			mark = "UNMET"
		}
		fmt.Fprintf(&b, "- **%s** %s — %s\n", c.ID, mark, c.Title)
		for _, ch := range c.Checks {
			if !ch.OK {
				fmt.Fprintf(&b, "  - %s: %s\n", ch.Name, ch.Detail)
			}
		}
	}
	fmt.Fprintf(&b, "\n%s\n", r.ClaimBoundary)
	return b.String()
}
