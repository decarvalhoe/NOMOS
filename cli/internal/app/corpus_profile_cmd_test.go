package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

func rbokFixture() string {
	return filepath.Join("testdata", "rbok-corpus")
}

// --- corpus feed --profile ---

func TestCorpusFeedProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var result corpus.ProfileFeedResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if result.Profile != "rbok-lawbook" {
		t.Fatalf("expected rbok-lawbook, got %q", result.Profile)
	}
	if result.SourceCount == 0 {
		t.Fatal("expected non-zero source count")
	}
	for _, flag := range []corpus.OutputFlag{"index", "governance", "citation", "import"} {
		if _, ok := result.Sections[flag]; !ok {
			t.Fatalf("expected section %q", flag)
		}
	}
}

func TestCorpusFeedProfileText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
		"--format", "text",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "rbok-lawbook") {
		t.Fatalf("expected profile name in text, got %q", stdout.String())
	}
}

func TestCorpusFeedProfileToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "feed.json")
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
		"--out", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "rbok-lawbook") {
		t.Fatal("expected profile in output file")
	}
}

func TestCorpusFeedProfileFilterOutputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
		"--outputs", "index,governance",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	var result corpus.ProfileFeedResult
	json.Unmarshal(stdout.Bytes(), &result)
	if _, ok := result.Sections["index"]; !ok {
		t.Fatal("expected index section")
	}
	if _, ok := result.Sections["governance"]; !ok {
		t.Fatal("expected governance section")
	}
}

func TestCorpusFeedProfileMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--profile is required") {
		t.Fatalf("expected profile required, got %q", stderr.String())
	}
}

func TestCorpusFeedProfileUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfileFeedCommand([]string{
		"--profile", "nonexistent",
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

// --- corpus feed dispatch ---

func TestCorpusFeedDispatchRoutesToProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusFeedDispatch([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "rbok-lawbook") {
		t.Fatalf("expected profile feed, got %q", stdout.String())
	}
}

// --- corpus diagnose ---

func TestCorpusDiagnoseProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusDiagnoseCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("expected 0 or 1, got %d; stderr=%q", code, stderr.String())
	}
	var verdict corpus.DiagnoseVerdict
	if err := json.Unmarshal(stdout.Bytes(), &verdict); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if verdict.Profile != "rbok-lawbook" {
		t.Fatalf("expected rbok-lawbook, got %q", verdict.Profile)
	}
	if verdict.Verdict == "" {
		t.Fatal("expected non-empty verdict")
	}
}

func TestCorpusDiagnoseText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusDiagnoseCommand([]string{
		"--profile", "rbok-lawbook",
		"--root", rbokFixture(),
		"--format", "text",
	}, &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("expected 0 or 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "verdict:") {
		t.Fatalf("expected verdict in text, got %q", stdout.String())
	}
}

func TestCorpusDiagnoseMissingProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusDiagnoseCommand([]string{
		"--root", rbokFixture(),
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

// --- corpus profiles ---

func TestCorpusProfilesList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := corpusProfilesCommand(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "rbok-lawbook") {
		t.Fatalf("expected rbok-lawbook, got %q", stdout.String())
	}
}

// --- integration via Run ---

func TestRunCorpusProfiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"corpus", "profiles"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "rbok-lawbook") {
		t.Fatalf("expected rbok-lawbook via Run, got %q", stdout.String())
	}
}

func TestRunCorpusFeedProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"corpus", "feed", "--profile", "rbok-lawbook", "--root", rbokFixture()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "rbok-lawbook") {
		t.Fatalf("expected profile feed via Run")
	}
}

func TestRunCorpusDiagnose(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"corpus", "diagnose", "--profile", "rbok-lawbook", "--root", rbokFixture()}, &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("expected 0 or 1, got %d; stderr=%q", code, stderr.String())
	}
}
