package registry

import (
	"errors"
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	r := New()
	p, err := r.Register("acme", "ACME Corp", "https://github.com/acme/app", "main", map[string]string{"team": "platform"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if p.ID != "acme" || p.Name != "ACME Corp" {
		t.Fatalf("unexpected project: %+v", p)
	}

	got, err := r.Get("acme")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Repository != "https://github.com/acme/app" {
		t.Fatalf("unexpected repository: %s", got.Repository)
	}
}

func TestRegisterDuplicateReturnsError(t *testing.T) {
	r := New()
	if _, err := r.Register("dup", "Dup", "repo", "main", nil); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := r.Register("dup", "Dup2", "repo2", "main", nil)
	if !errors.Is(err, ErrProjectExists) {
		t.Fatalf("expected ErrProjectExists, got: %v", err)
	}
}

func TestGetNotFoundReturnsError(t *testing.T) {
	r := New()
	_, err := r.Get("nonexistent")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got: %v", err)
	}
}

func TestListReturnsAllProjects(t *testing.T) {
	r := New()
	r.Register("a", "A", "r1", "main", nil)
	r.Register("b", "B", "r2", "main", nil)

	projects := r.List()
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

func TestUpdateModifiesMutableFields(t *testing.T) {
	r := New()
	r.Register("proj", "Old", "repo", "main", nil)

	updated, err := r.Update("proj", "New", "", "develop", map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "New" || updated.DefaultRef != "develop" {
		t.Fatalf("unexpected update: %+v", updated)
	}
	if updated.Labels["env"] != "prod" {
		t.Fatalf("labels not updated: %v", updated.Labels)
	}
	// Repository should remain unchanged since empty string was passed.
	if updated.Repository != "repo" {
		t.Fatalf("repository should not change: %s", updated.Repository)
	}
}

func TestRemoveDeletesProjectAndHistory(t *testing.T) {
	r := New()
	r.Register("gone", "Gone", "repo", "main", nil)
	r.StartExecution("gone", "main", "detect")

	if err := r.Remove("gone"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, err := r.Get("gone")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected project removed, got: %v", err)
	}
}

func TestExecutionLifecycle(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)

	exec, err := r.StartExecution("proj", "main", "detect")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if exec.Status != StatusPending {
		t.Fatalf("expected pending, got %s", exec.Status)
	}

	exec, err = r.UpdateExecution("proj", exec.ID, StatusRunning, nil, "")
	if err != nil {
		t.Fatalf("transition to running: %v", err)
	}
	if exec.Status != StatusRunning {
		t.Fatalf("expected running, got %s", exec.Status)
	}

	exitCode := 0
	exec, err = r.UpdateExecution("proj", exec.ID, StatusCompleted, &exitCode, "3 findings")
	if err != nil {
		t.Fatalf("transition to completed: %v", err)
	}
	if exec.Status != StatusCompleted || exec.FinishedAt == nil || *exec.ExitCode != 0 {
		t.Fatalf("unexpected completed state: %+v", exec)
	}
	if exec.Summary != "3 findings" {
		t.Fatalf("expected summary, got: %s", exec.Summary)
	}
}

func TestExecutionInvalidTransition(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)
	exec, _ := r.StartExecution("proj", "main", "detect")

	// pending -> completed is not valid (must go through running)
	_, err := r.UpdateExecution("proj", exec.ID, StatusCompleted, nil, "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got: %v", err)
	}
}

func TestExecutionFailFromPending(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)
	exec, _ := r.StartExecution("proj", "main", "detect")

	exec, err := r.UpdateExecution("proj", exec.ID, StatusFailed, nil, "timeout")
	if err != nil {
		t.Fatalf("fail from pending: %v", err)
	}
	if exec.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", exec.Status)
	}
}

func TestStatusShowsLastExecution(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)

	status, err := r.Status("proj")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.LastExecution != nil {
		t.Fatal("expected no last execution before any runs")
	}
	if status.TotalRuns != 0 {
		t.Fatalf("expected 0 runs, got %d", status.TotalRuns)
	}

	r.StartExecution("proj", "main", "detect")
	r.StartExecution("proj", "main", "diagnose")

	status, _ = r.Status("proj")
	if status.TotalRuns != 2 {
		t.Fatalf("expected 2 runs, got %d", status.TotalRuns)
	}
	if status.LastExecution.Command != "diagnose" {
		t.Fatalf("expected last command=diagnose, got %s", status.LastExecution.Command)
	}
}

func TestHistoryReturnsMostRecentFirst(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)

	r.StartExecution("proj", "main", "first")
	r.StartExecution("proj", "main", "second")
	r.StartExecution("proj", "main", "third")

	history, err := r.History("proj", 2)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(history))
	}
	if history[0].Command != "third" || history[1].Command != "second" {
		t.Fatalf("unexpected order: %s, %s", history[0].Command, history[1].Command)
	}
}

func TestHistoryUnlimited(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)
	r.StartExecution("proj", "main", "a")
	r.StartExecution("proj", "main", "b")

	history, _ := r.History("proj", 0)
	if len(history) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(history))
	}
}

func TestHistoryTrimsOldEntries(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)

	for i := 0; i < maxHistoryPerProject+10; i++ {
		r.StartExecution("proj", "main", "detect")
	}

	history, _ := r.History("proj", 0)
	if len(history) != maxHistoryPerProject {
		t.Fatalf("expected %d entries after trim, got %d", maxHistoryPerProject, len(history))
	}
}

func TestStartExecutionForUnknownProject(t *testing.T) {
	r := New()
	_, err := r.StartExecution("ghost", "main", "detect")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound, got: %v", err)
	}
}

func TestGetExecution(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)
	started, _ := r.StartExecution("proj", "main", "detect")

	got, err := r.GetExecution("proj", started.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.ID != started.ID {
		t.Fatalf("expected %s, got %s", started.ID, got.ID)
	}
}

func TestGetExecutionNotFound(t *testing.T) {
	r := New()
	r.Register("proj", "P", "repo", "main", nil)

	_, err := r.GetExecution("proj", "exec-999")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Fatalf("expected ErrExecutionNotFound, got: %v", err)
	}
}
