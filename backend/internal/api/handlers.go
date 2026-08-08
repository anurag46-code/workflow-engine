package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anurag46-code/workflow-engine/internal/engine"
	"github.com/anurag46-code/workflow-engine/internal/models"
	"github.com/anurag46-code/workflow-engine/internal/store"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	store  *store.Store
	engine *engine.Engine
}

func New(s *store.Store, eng *engine.Engine) *Handler {
	return &Handler{store: s, engine: eng}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/workflows", h.submitWorkflow)
	r.GET("/workflows", h.listWorkflows)
	r.GET("/workflows/:id", h.getWorkflow)
	r.GET("/workflows/:id/tasks", h.getWorkflowTasks)
	r.GET("/workflows/:id/stream", h.streamWorkflow) // SSE endpoint
	r.POST("/workflows/demo", h.submitDemo)
}

func (h *Handler) submitWorkflow(c *gin.Context) {
	var def models.WorkflowDef
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	run, err := h.engine.Submit(def)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}

func (h *Handler) listWorkflows(c *gin.Context) {
	runs, err := h.store.ListWorkflowRuns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []*models.WorkflowRun{}
	}
	c.JSON(http.StatusOK, runs)
}

func (h *Handler) getWorkflow(c *gin.Context) {
	run, err := h.store.GetWorkflowRun(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *Handler) getWorkflowTasks(c *gin.Context) {
	tasks, err := h.store.GetTaskRunsByWorkflow(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []*models.TaskRun{}
	}
	c.JSON(http.StatusOK, tasks)
}

// streamWorkflow sends SSE events to the client, polling the DB every second
// and pushing updates whenever task/workflow state changes.
// SSE is one-directional (server -> client) so it's simpler than WebSockets
// for this use case - the client only needs to receive status updates.
func (h *Handler) streamWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // disable nginx buffering for SSE

	clientGone := c.Request.Context().Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	send := func(event string, data any) bool {
		b, err := json.Marshal(data)
		if err != nil {
			return true
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(b))
		c.Writer.Flush()
		return false
	}

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			run, err := h.store.GetWorkflowRun(workflowID)
			if err != nil {
				return
			}
			tasks, err := h.store.GetTaskRunsByWorkflow(workflowID)
			if err != nil {
				return
			}
			if tasks == nil {
				tasks = []*models.TaskRun{}
			}

			payload := gin.H{"workflow": run, "tasks": tasks}
			if done := send("update", payload); done {
				return
			}

			// Stop streaming once the workflow reaches a terminal state
			if run.Status == models.WorkflowSuccess || run.Status == models.WorkflowFailed {
				send("done", gin.H{"status": run.Status})
				return
			}
		}
	}
}

// submitDemo creates a pre-built demo workflow that showcases all features:
// parallel tasks, dependencies, retries, and different task types.
func (h *Handler) submitDemo(c *gin.Context) {
	def := models.WorkflowDef{
		Name: "Demo Pipeline",
		Tasks: []models.TaskDef{
			{
				ID:   "fetch-data",
				Type: "http",
				Config: map[string]any{
					"url": "https://httpbin.org/get",
				},
				MaxRetries: 3,
			},
			{
				ID:        "process-a",
				Type:      "wait",
				DependsOn: []string{"fetch-data"},
				Config:    map[string]any{"seconds": 3},
				MaxRetries: 2,
			},
			{
				ID:        "process-b",
				Type:      "wait",
				DependsOn: []string{"fetch-data"},
				Config:    map[string]any{"seconds": 2},
				MaxRetries: 2,
			},
			{
				ID:        "flaky-task",
				Type:      "fail_sometimes",
				DependsOn: []string{"fetch-data"},
				Config:    map[string]any{"failRate": 0.8},
				MaxRetries: 3,
			},
			{
				ID:        "aggregate",
				Type:      "transform",
				DependsOn: []string{"process-a", "process-b", "flaky-task"},
				Config:    map[string]any{"input": "merged results", "op": "upper"},
				MaxRetries: 1,
			},
			{
				ID:        "notify",
				Type:      "wait",
				DependsOn: []string{"aggregate"},
				Config:    map[string]any{"seconds": 1},
				MaxRetries: 1,
			},
		},
	}

	run, err := h.engine.Submit(def)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}
