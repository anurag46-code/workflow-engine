package models

import "time"

type WorkflowStatus string
type TaskStatus string

const (
	WorkflowPending  WorkflowStatus = "pending"
	WorkflowRunning  WorkflowStatus = "running"
	WorkflowSuccess  WorkflowStatus = "success"
	WorkflowFailed   WorkflowStatus = "failed"

	TaskPending  TaskStatus = "pending"
	TaskRunning  TaskStatus = "running"
	TaskSuccess  TaskStatus = "success"
	TaskFailed   TaskStatus = "failed"
	TaskRetrying TaskStatus = "retrying"
)

// WorkflowDef is the user-supplied DAG definition (what to run and in what order).
type WorkflowDef struct {
	Name  string      `json:"name"`
	Tasks []TaskDef   `json:"tasks"`
}

// TaskDef is one node in the DAG.
type TaskDef struct {
	ID       string   `json:"id"`        // unique within the workflow
	Type     string   `json:"type"`      // "http", "wait", "transform"
	DependsOn []string `json:"dependsOn"` // IDs of tasks that must succeed first
	Config   map[string]any `json:"config"`
	MaxRetries int    `json:"maxRetries"`
}

// WorkflowRun is one execution instance of a WorkflowDef.
type WorkflowRun struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Status     WorkflowStatus `json:"status"`
	Definition WorkflowDef    `json:"definition"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// TaskRun is one execution instance of a TaskDef within a WorkflowRun.
type TaskRun struct {
	ID          string         `json:"id"`
	WorkflowID  string         `json:"workflowId"`
	TaskID      string         `json:"taskId"`      // matches TaskDef.ID
	Type        string         `json:"type"`
	Status      TaskStatus     `json:"status"`
	Retries     int            `json:"retries"`
	MaxRetries  int            `json:"maxRetries"`
	DependsOn   []string       `json:"dependsOn"`
	Config      map[string]any `json:"config"`
	Output      string         `json:"output"`
	Error       string         `json:"error"`
	LockedBy    string         `json:"lockedBy"`    // worker ID holding the lease
	LockedUntil *time.Time     `json:"lockedUntil"` // lease expiry
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}
