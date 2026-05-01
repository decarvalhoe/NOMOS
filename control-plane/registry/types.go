package registry

import "time"

// ExecutionStatus represents the lifecycle state of a single run.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
)

// Project is the unit of registration in the Nomos control plane.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Repository  string    `json:"repository"`
	DefaultRef  string    `json:"defaultRef"`
	RegisteredAt time.Time `json:"registeredAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Execution records one run of Nomos against a registered project.
type Execution struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"projectId"`
	Ref         string          `json:"ref"`
	Status      ExecutionStatus `json:"status"`
	Command     string          `json:"command"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	ExitCode    *int            `json:"exitCode,omitempty"`
	Summary     string          `json:"summary,omitempty"`
}

// ProjectStatus is the derived current state of a project in the registry.
type ProjectStatus struct {
	Project       Project     `json:"project"`
	LastExecution *Execution  `json:"lastExecution,omitempty"`
	TotalRuns     int         `json:"totalRuns"`
}
