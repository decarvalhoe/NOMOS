package registry

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const maxHistoryPerProject = 50

var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrProjectExists      = errors.New("project already registered")
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrInvalidTransition  = errors.New("invalid status transition")
)

// Registry is an in-memory store for project registrations and executions.
type Registry struct {
	mu         sync.RWMutex
	projects   map[string]*Project
	executions map[string][]*Execution // keyed by project ID
	nextExecID int
}

// New creates an empty registry.
func New() *Registry {
	return &Registry{
		projects:   make(map[string]*Project),
		executions: make(map[string][]*Execution),
	}
}

// Register adds a new project to the registry.
func (r *Registry) Register(id, name, repository, defaultRef string, labels map[string]string) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[id]; exists {
		return Project{}, fmt.Errorf("%w: %s", ErrProjectExists, id)
	}

	now := time.Now().UTC()
	p := &Project{
		ID:           id,
		Name:         name,
		Repository:   repository,
		DefaultRef:   defaultRef,
		RegisteredAt: now,
		UpdatedAt:    now,
		Labels:       labels,
	}
	r.projects[id] = p
	return *p, nil
}

// Get returns a project by ID.
func (r *Registry) Get(id string) (Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.projects[id]
	if !ok {
		return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}
	return *p, nil
}

// List returns all registered projects sorted by registration time.
func (r *Registry) List() []Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Project, 0, len(r.projects))
	for _, p := range r.projects {
		result = append(result, *p)
	}
	return result
}

// Update modifies a project's mutable fields.
func (r *Registry) Update(id, name, repository, defaultRef string, labels map[string]string) (Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.projects[id]
	if !ok {
		return Project{}, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}

	if name != "" {
		p.Name = name
	}
	if repository != "" {
		p.Repository = repository
	}
	if defaultRef != "" {
		p.DefaultRef = defaultRef
	}
	if labels != nil {
		p.Labels = labels
	}
	p.UpdatedAt = time.Now().UTC()
	return *p, nil
}

// Remove deletes a project and its execution history.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.projects[id]; !ok {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}
	delete(r.projects, id)
	delete(r.executions, id)
	return nil
}

// StartExecution records a new execution for a project.
func (r *Registry) StartExecution(projectID, ref, command string) (Execution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.projects[projectID]; !ok {
		return Execution{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	r.nextExecID++
	now := time.Now().UTC()
	exec := &Execution{
		ID:        fmt.Sprintf("exec-%d", r.nextExecID),
		ProjectID: projectID,
		Ref:       ref,
		Status:    StatusPending,
		Command:   command,
		StartedAt: now,
	}

	r.executions[projectID] = append(r.executions[projectID], exec)
	r.trimHistory(projectID)
	return *exec, nil
}

// UpdateExecution transitions an execution's status.
func (r *Registry) UpdateExecution(projectID, executionID string, status ExecutionStatus, exitCode *int, summary string) (Execution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	exec, err := r.findExecution(projectID, executionID)
	if err != nil {
		return Execution{}, err
	}

	if !isValidTransition(exec.Status, status) {
		return Execution{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, exec.Status, status)
	}

	exec.Status = status
	if status == StatusCompleted || status == StatusFailed {
		now := time.Now().UTC()
		exec.FinishedAt = &now
	}
	if exitCode != nil {
		exec.ExitCode = exitCode
	}
	if summary != "" {
		exec.Summary = summary
	}
	return *exec, nil
}

// GetExecution returns a specific execution.
func (r *Registry) GetExecution(projectID, executionID string) (Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exec, err := r.findExecution(projectID, executionID)
	if err != nil {
		return Execution{}, err
	}
	return *exec, nil
}

// Status returns the current status of a project including its last execution.
func (r *Registry) Status(projectID string) (ProjectStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.projects[projectID]
	if !ok {
		return ProjectStatus{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	ps := ProjectStatus{
		Project:   *p,
		TotalRuns: len(r.executions[projectID]),
	}

	if execs := r.executions[projectID]; len(execs) > 0 {
		last := *execs[len(execs)-1]
		ps.LastExecution = &last
	}
	return ps, nil
}

// History returns the execution history for a project, most recent first.
func (r *Registry) History(projectID string, limit int) ([]Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.projects[projectID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	execs := r.executions[projectID]
	if limit <= 0 || limit > len(execs) {
		limit = len(execs)
	}

	result := make([]Execution, limit)
	for i := 0; i < limit; i++ {
		result[i] = *execs[len(execs)-1-i]
	}
	return result, nil
}

func (r *Registry) findExecution(projectID, executionID string) (*Execution, error) {
	if _, ok := r.projects[projectID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	for _, exec := range r.executions[projectID] {
		if exec.ID == executionID {
			return exec, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrExecutionNotFound, executionID)
}

func (r *Registry) trimHistory(projectID string) {
	execs := r.executions[projectID]
	if len(execs) > maxHistoryPerProject {
		r.executions[projectID] = execs[len(execs)-maxHistoryPerProject:]
	}
}

func isValidTransition(from, to ExecutionStatus) bool {
	switch from {
	case StatusPending:
		return to == StatusRunning || to == StatusFailed
	case StatusRunning:
		return to == StatusCompleted || to == StatusFailed
	default:
		return false
	}
}
