package engine

import (
	"fmt"
	"log"

	"github.com/anurag46-code/workflow-engine/internal/models"
	"github.com/anurag46-code/workflow-engine/internal/store"
	"github.com/google/uuid"
)

// Engine turns a WorkflowDef into a set of TaskRuns in the database,
// validates there are no cycles in the DAG, and advances workflow state
// when tasks complete or fail.
type Engine struct {
	store *store.Store
}

func New(s *store.Store) *Engine {
	return &Engine{store: s}
}

// Submit validates the DAG and creates all TaskRuns in pending state.
func (e *Engine) Submit(def models.WorkflowDef) (*models.WorkflowRun, error) {
	if err := validateDAG(def); err != nil {
		return nil, fmt.Errorf("invalid DAG: %w", err)
	}

	run := &models.WorkflowRun{
		ID:         uuid.New().String(),
		Name:       def.Name,
		Status:     models.WorkflowPending,
		Definition: def,
	}
	if err := e.store.CreateWorkflowRun(run); err != nil {
		return nil, fmt.Errorf("create workflow run: %w", err)
	}

	for _, task := range def.Tasks {
		maxRetries := task.MaxRetries
		if maxRetries == 0 {
			maxRetries = 3
		}
		deps := task.DependsOn
		if deps == nil {
			deps = []string{}
		}
		t := &models.TaskRun{
			ID:         uuid.New().String(),
			WorkflowID: run.ID,
			TaskID:     task.ID,
			Type:       task.Type,
			Status:     models.TaskPending,
			MaxRetries: maxRetries,
			DependsOn:  deps,
			Config:     task.Config,
		}
		if t.Config == nil {
			t.Config = map[string]any{}
		}
		if err := e.store.CreateTaskRun(t); err != nil {
			return nil, fmt.Errorf("create task run %s: %w", task.ID, err)
		}
	}

	if err := e.store.UpdateWorkflowStatus(run.ID, models.WorkflowRunning); err != nil {
		return nil, err
	}
	run.Status = models.WorkflowRunning

	log.Printf("workflow %s (%s) submitted with %d tasks", run.Name, run.ID, len(def.Tasks))
	return run, nil
}

// Advance checks if the workflow is complete or failed after a task finishes.
func (e *Engine) Advance(workflowID string) {
	tasks, err := e.store.GetTaskRunsByWorkflow(workflowID)
	if err != nil {
		log.Printf("advance: get tasks for %s: %v", workflowID, err)
		return
	}

	allDone := true
	anyFailed := false
	for _, t := range tasks {
		if t.Status == models.TaskFailed {
			anyFailed = true
		}
		if t.Status != models.TaskSuccess && t.Status != models.TaskFailed {
			allDone = false
		}
	}

	if !allDone {
		return
	}

	status := models.WorkflowSuccess
	if anyFailed {
		status = models.WorkflowFailed
	}

	if err := e.store.UpdateWorkflowStatus(workflowID, status); err != nil {
		log.Printf("advance: update workflow %s: %v", workflowID, err)
	}
	log.Printf("workflow %s finished with status %s", workflowID, status)
}

// validateDAG ensures all dependency IDs exist and there are no cycles.
// Uses DFS with three-color marking (white=unvisited, gray=in-stack, black=done).
func validateDAG(def models.WorkflowDef) error {
	taskIDs := make(map[string]bool)
	for _, t := range def.Tasks {
		if taskIDs[t.ID] {
			return fmt.Errorf("duplicate task ID: %s", t.ID)
		}
		taskIDs[t.ID] = true
	}

	deps := make(map[string][]string)
	for _, t := range def.Tasks {
		for _, dep := range t.DependsOn {
			if !taskIDs[dep] {
				return fmt.Errorf("task %s depends on unknown task %s", t.ID, dep)
			}
		}
		deps[t.ID] = t.DependsOn
	}

	// DFS cycle detection
	color := make(map[string]int) // 0=white, 1=gray, 2=black
	var dfs func(id string) error
	dfs = func(id string) error {
		color[id] = 1
		for _, dep := range deps[id] {
			if color[dep] == 1 {
				return fmt.Errorf("cycle detected involving task %s", id)
			}
			if color[dep] == 0 {
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		color[id] = 2
		return nil
	}
	for _, t := range def.Tasks {
		if color[t.ID] == 0 {
			if err := dfs(t.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
