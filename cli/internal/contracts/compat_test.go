package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemverCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.0-ALPHA", "0.1.0", 1}, {"0.2.0-ALPHA", "0.2.0", -1}, {"0.2.0", "0.2.0", 0}, {"1.0.0", "0.9.9", 1},
		{"0.2.0-ALPHA", "0.2.0-BETA", -1}, {"v0.3.0", "0.3.0", 0}, {"0.2.1", "0.2.0-ALPHA", 1},
		// NRT-036 (#719): the beta label orders above every alpha and below the final 1.0.0.
		{"1.0.0-BETA.1", "0.2.0-ALPHA", 1}, {"1.0.0-BETA.1", "1.0.0", -1}, {"1.0.0-BETA.1", "1.0.0-BETA.1", 0}, {"1.0.0-BETA.2", "1.0.0-BETA.1", 1}, {"1.0.0-BETA.1", "0.9.9", 1},
	}
	for _, c := range cases {
		a, err := parseSemver(c.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := parseSemver(c.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := a.compare(b); got != c.want {
			t.Errorf("%s vs %s: got %d want %d", c.a, c.b, got, c.want)
		}
	}
	if _, err := parseSemver("0.2"); err == nil {
		t.Fatal("0.2 must not parse")
	}
}

func adapterRoot(t *testing.T, manifest string) (string, Registry) {
	t.Helper()
	root := mini(t, nil)
	dir := filepath.Join(root, "adapters", "x")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "adapter.nomos.yaml"), []byte(manifest), 0o644)
	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// the mini registry has contract a (1.0.0); give it an adapter-manifest and a nomos-project line
	reg.Contracts = append(reg.Contracts, Contract{ID: adapterContractID, SchemaVersion: "0.1.0"}, Contract{ID: "nomos-project", SchemaVersion: "0.1.0"})
	return root, reg
}

const okManifest = `schema_version: "0.1.0"
adapter: {id: x, version: "0.1.0", status: experimental}
compatibility:
  nomos_core: {min_version: "0.1.0", max_version: "0.9.0"}
  manifest_contract: {version: "0.1.0"}
  schemas:
    nomos_project: {min_version: "0.1.0"}
`

func TestAdapterInsideRangeIsCompatible(t *testing.T) {
	root, reg := adapterRoot(t, okManifest)
	got, err := CheckAdapters(root, "0.2.0-ALPHA", reg)
	if err != nil || len(got) != 1 || got[0].Verdict != "compatible" || got[0].Schemas["nomos_project"] != "0.1.0" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestAdapterOutsideRangeIsRed(t *testing.T) {
	root, reg := adapterRoot(t, strings.Replace(okManifest, `min_version: "0.1.0", max_version: "0.9.0"`, `min_version: "0.3.0"`, 1))
	got, err := CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, "x (adapters/x/adapter.nomos.yaml): requires core >= 0.3.0, current core is 0.2.0-ALPHA")
	if len(got) != 1 || got[0].Verdict != "incompatible" {
		t.Fatalf("verdict must be reported too: %+v", got)
	}
	root, reg = adapterRoot(t, strings.Replace(okManifest, `max_version: "0.9.0"`, `max_version: "0.1.5"`, 1))
	_, err = CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, "supports core <= 0.1.5, current core is 0.2.0-ALPHA")
}

func TestAdapterManifestContractAndSchemasAreChecked(t *testing.T) {
	root, reg := adapterRoot(t, strings.Replace(okManifest, `manifest_contract: {version: "0.1.0"}`, `manifest_contract: {version: "0.2.0"}`, 1))
	_, err := CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, `manifest_contract.version "0.2.0" is newer than the adapter-manifest 0.1.0 the core ships`)
	root, reg = adapterRoot(t, strings.Replace(okManifest, `manifest_contract: {version: "0.1.0"}`, `manifest_contract: {version: "1.0.0"}`, 1))
	for i := range reg.Contracts {
		if reg.Contracts[i].ID == adapterContractID {
			reg.Contracts[i].SchemaVersion = "2.1.0"
		}
	}
	_, err = CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, `manifest_contract.version "1.0.0" is another MAJOR than the adapter-manifest 2.1.0 the core ships`)
	root, reg = adapterRoot(t, strings.Replace(okManifest, `manifest_contract: {version: "0.1.0"}`, `manifest_contract: {version: "0.0.9"}`, 1))
	if _, err := CheckAdapters(root, "0.2.0-ALPHA", reg); err != nil {
		t.Fatalf("an older MINOR/PATCH of the same MAJOR is compatible: %v", err)
	}

	root, reg = adapterRoot(t, strings.Replace(okManifest, `nomos_project: {min_version: "0.1.0"}`, `nomos_project: {min_version: "0.4.0"}`, 1))
	_, err = CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, "schemas.nomos_project requires >= 0.4.0, the core ships 0.1.0")

	root, reg = adapterRoot(t, strings.Replace(okManifest, `nomos_project: {min_version: "0.1.0"}`, `mystery: {min_version: "0.1.0"}`, 1))
	_, err = CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, "schemas.mystery is not a schema the core knows")

	root, reg = adapterRoot(t, "adapter: [not, a, mapping\n")
	_, err = CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterUnreadable, "adapters/x/adapter.nomos.yaml")

	root, reg = adapterRoot(t, okManifest)
	_, err = CheckAdapters(root, "banana", reg)
	wantCode(t, err, CodeAdapterIncompatible, "core version")
}

func TestRealAdaptersAreCompatibleWithTheRealCore(t *testing.T) {
	root := repoRoot(t)
	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// the version the CLI ships; read from app.go so this test cannot drift from it
	raw, err := os.ReadFile(filepath.Join(root, "cli", "internal", "app", "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(raw), `const Version = "`)
	core := string(raw)[i+len(`const Version = "`):]
	core = core[:strings.Index(core, `"`)]
	got, err := CheckAdapters(root, core, reg)
	if err != nil || len(got) < 3 {
		t.Fatalf("%v %+v", err, got)
	}
	ann, err := Announce(root, core)
	if err != nil || ann.CoreVersion != core || ann.Registry.Total < 30 || len(ann.Adapters) != len(got) {
		t.Fatalf("%v %+v", err, ann.Registry)
	}
	if err := CheckDocs(root, ann); err != nil {
		t.Fatalf("docs/16 must carry the generated matrix: %v", err)
	}
}

func TestMatrixIsDeterministicAndDriftIsRed(t *testing.T) {
	root, _ := adapterRoot(t, strings.Replace(okManifest, "  schemas:\n    nomos_project: {min_version: \"0.1.0\"}\n", "", 1))
	_ = os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	doc := filepath.Join(root, filepath.FromSlash(MatrixDoc))
	_ = os.WriteFile(doc, []byte("# 16\n\n## Compatibility Matrix\n\nold hand-written table\n\n## Deprecation Policy\n\nrules\n"), 0o644)
	ann, err := Announce(root, "0.2.0-ALPHA")
	if err != nil {
		t.Fatal(err)
	}
	if RenderMatrix(ann) != RenderMatrix(ann) || !strings.Contains(RenderMatrix(ann), "| `x` | `0.1.0` | experimental | >= 0.1.0, <= 0.9.0 | `0.1.0` |  | compatible |") {
		t.Fatalf("render:\n%s", RenderMatrix(ann))
	}
	wantCode(t, CheckDocs(root, ann), CodeDocsDrift, "run `nomos contracts status --emit-docs`")
	if err := EmitDocs(root, ann); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(doc)
	if strings.Contains(string(out), "old hand-written table") || !strings.Contains(string(out), "## Deprecation Policy\n\nrules") {
		t.Fatalf("splice wrong:\n%s", out)
	}
	if err := CheckDocs(root, ann); err != nil {
		t.Fatal(err)
	}
	// hand edit inside the generated block → drift
	_ = os.WriteFile(doc, []byte(strings.Replace(string(out), "| compatible |", "| certified |", 1)), 0o644)
	wantCode(t, CheckDocs(root, ann), CodeDocsDrift, MatrixDoc)
	// second emission replaces the block in place, idempotently
	if err := EmitDocs(root, ann); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(doc)
	if string(again) != string(out) {
		t.Fatal("re-emission is not idempotent")
	}
}

func TestDeprecatedContractIsWarnedAndAnnounced(t *testing.T) {
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    stability: experimental\n", "    stability: deprecated\n    deprecated_since: 2026-09-01\n    removal_not_before: 2027-03-01\n", 1)
	})
	rep, err := Verify(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], "contract b (specs/b.cue) is deprecated since 2026-09-01; removal not before 2027-03-01") {
		t.Fatalf("%+v", rep.Warnings)
	}
	ann, err := Announce(root, "0.2.0-ALPHA")
	if err != nil {
		t.Fatal(err)
	}
	if len(ann.Warnings) != 1 || ann.Contracts[1].RemovalNotBefore != "2027-03-01" || !strings.Contains(RenderMatrix(ann), "deprecated (since 2026-09-01, removal not before 2027-03-01)") {
		t.Fatalf("%+v\n%s", ann.Contracts, RenderMatrix(ann))
	}
	rep2, err := Verify(mini(t, nil), now)
	if err != nil || len(rep2.Warnings) != 0 {
		t.Fatalf("no deprecation → no warning: %v %+v", err, rep2.Warnings)
	}
}

func TestAnnouncementReadsAndWritesComeFromTheRegistry(t *testing.T) {
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    readers: [x.go]\n", "    readers: [x.go]\n    writers: [w.go]\n", 1)
	})
	_ = os.WriteFile(filepath.Join(root, "x.go"), []byte("package x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "w.go"), []byte("package x\n"), 0o644)
	ann, err := Announce(root, "0.2.0-ALPHA")
	if err != nil {
		t.Fatal(err)
	}
	if !ann.Contracts[0].Reads || !ann.Contracts[0].Writes || ann.Contracts[1].Reads || ann.Contracts[1].Writes {
		t.Fatalf("%+v", ann.Contracts)
	}
	if ann.Formats["attestation_predicate"] != "https://nomos.dev/attestation/v1" || ann.Registry.Sha256 == "" {
		t.Fatalf("%+v", ann)
	}
}

func TestSemverRejectsTrailingGarbageAndOrdersReleasesAbovePreReleases(t *testing.T) {
	for _, bad := range []string{"0.2.0.5", "1.2.3 beta", "1.2.3-", "", "latest"} {
		if _, err := parseSemver(bad); err == nil {
			t.Errorf("%q must not parse", bad)
		}
	}
	rel, _ := parseSemver("0.2.0")
	pre, _ := parseSemver("0.2.0-ALPHA")
	if rel.compare(pre) != 1 || pre.compare(rel) != -1 {
		t.Fatalf("release must sort above its pre-release: %d %d", rel.compare(pre), pre.compare(rel))
	}
}

func TestAdapterSchemaMaxBelowShippedIsRed(t *testing.T) {
	root, reg := adapterRoot(t, strings.Replace(okManifest, `nomos_project: {min_version: "0.1.0"}`, `nomos_project: {min_version: "0.0.1", max_version: "0.0.5"}`, 1))
	got, err := CheckAdapters(root, "0.2.0-ALPHA", reg)
	wantCode(t, err, CodeAdapterIncompatible, "schemas.nomos_project supports <= 0.0.5, the core ships 0.1.0")
	if got[0].Schemas["nomos_project"] != "0.0.1..0.0.5" {
		t.Fatalf("%+v", got[0].Schemas)
	}
}

func TestEveryDeprecatedContractIsWarned(t *testing.T) {
	cueC := "package specs\n#C: {y: int}\n"
	root := mini(t, func(reg *string) {
		*reg = strings.Replace(*reg, "    stability: experimental\n", "    stability: deprecated\n    deprecated_since: 2026-09-01\n    removal_not_before: 2027-03-01\n", 1)
		*reg += `  - id: c
    path: specs/c.cue
    stability: deprecated
    deprecated_since: 2026-08-01
    removal_not_before: 2027-02-01
    version_kind: none
    schema_version: ""
    sha256: "` + sha([]byte(cueC)) + `"
    definition: "#C"
    fixtures: {valid: [], invalid: []}
    readers: []
    compat_fixtures: []
`
	})
	_ = os.WriteFile(filepath.Join(root, "specs", "c.cue"), []byte(cueC), 0o644)
	rep, err := Verify(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Warnings) != 2 || !strings.Contains(rep.Warnings[0], "contract b ") || !strings.Contains(rep.Warnings[1], "contract c ") {
		t.Fatalf("%+v", rep.Warnings)
	}
}
