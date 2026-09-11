package app

// NRT-022 (#670): the CLI pairs each --exceptions with the --project that
// precedes it, and the view is reachable through the registered command.

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPortfolioProjectsPairsExceptionsToPrecedingProject(t *testing.T) {
	td := "../portfolio/testdata/projects/"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"portfolio", "projects", "--project", td + "project-beta.yaml", "--project", td + "project-alpha.yaml", "--exceptions", td + "exceptions-alpha.yaml", "--now", "2026-06-15T00:00:00Z"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var view struct {
		Summary struct {
			Total      int `json:"total"`
			Exceptions int `json:"exceptions"`
		} `json:"summary"`
		Projects []struct {
			ID         string `json:"id"`
			Exceptions []any  `json:"exceptions"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Summary.Total != 2 || view.Summary.Exceptions != 2 {
		t.Fatalf("summary: %+v", view.Summary)
	}
	for _, p := range view.Projects {
		switch p.ID {
		case "alpha":
			if len(p.Exceptions) != 2 {
				t.Fatalf("alpha must carry the exceptions given right after it, got %d", len(p.Exceptions))
			}
		case "beta":
			if len(p.Exceptions) != 0 {
				t.Fatalf("beta must carry no exceptions, got %d", len(p.Exceptions))
			}
		}
	}
	// No project at all is a usage error, not an empty success.
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"portfolio", "projects"}, &stdout, &stderr); code != 2 {
		t.Fatalf("no --project must exit 2, got %d", code)
	}
	// A missing manifest is an error.
	if code := Run([]string{"portfolio", "projects", "--project", td + "nope.yaml"}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing manifest must exit 1, got %d", code)
	}
}
