package worker

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/anurag46-code/workflow-engine/internal/engine"
	"github.com/anurag46-code/workflow-engine/internal/models"
	"github.com/anurag46-code/workflow-engine/internal/store"
	"github.com/google/uuid"
)

const (
	pollInterval  = 2 * time.Second
	leaseDuration = 30 * time.Second
)

// Worker polls the Postgres queue for available tasks and executes them.
// Multiple workers can run concurrently - SELECT FOR UPDATE SKIP LOCKED
// ensures each task is claimed by exactly one worker.
type Worker struct {
	id     string
	store  *store.Store
	engine *engine.Engine
}

func New(s *store.Store, eng *engine.Engine) *Worker {
	return &Worker{
		id:     "worker-" + uuid.New().String()[:8],
		store:  s,
		engine: eng,
	}
}

// Run starts the poll loop. Call in a goroutine.
func (w *Worker) Run() {
	log.Printf("[%s] started", w.id)
	for {
		task, err := w.store.ClaimTask(w.id, leaseDuration)
		if err != nil {
			log.Printf("[%s] claim error: %v", w.id, err)
			time.Sleep(pollInterval)
			continue
		}
		if task == nil {
			time.Sleep(pollInterval)
			continue
		}
		w.execute(task)
	}
}

func (w *Worker) execute(task *models.TaskRun) {
	log.Printf("[%s] executing task %s (type=%s, workflow=%s)", w.id, task.TaskID, task.Type, task.WorkflowID)

	output, err := w.dispatch(task)
	if err != nil {
		log.Printf("[%s] task %s failed (attempt %d/%d): %v", w.id, task.TaskID, task.Retries+1, task.MaxRetries, err)
		if storeErr := w.store.FailTask(task.ID, err.Error(), task.Retries, task.MaxRetries); storeErr != nil {
			log.Printf("[%s] fail task store error: %v", w.id, storeErr)
		}
	} else {
		log.Printf("[%s] task %s succeeded", w.id, task.TaskID)
		if storeErr := w.store.CompleteTask(task.ID, output); storeErr != nil {
			log.Printf("[%s] complete task store error: %v", w.id, storeErr)
		}
	}

	w.engine.Advance(task.WorkflowID)
}

// dispatch routes a task to the right handler based on its type.
func (w *Worker) dispatch(task *models.TaskRun) (string, error) {
	switch task.Type {
	case "http":
		return w.runHTTP(task)
	case "wait":
		return w.runWait(task)
	case "transform":
		return w.runTransform(task)
	case "fail_sometimes":
		return w.runFailSometimes(task)
	default:
		return "", fmt.Errorf("unknown task type: %s", task.Type)
	}
}

func (w *Worker) runHTTP(task *models.TaskRun) (string, error) {
	url, _ := task.Config["url"].(string)
	if url == "" {
		return "", fmt.Errorf("http task missing url")
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http %d from %s", resp.StatusCode, url)
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}

func (w *Worker) runWait(task *models.TaskRun) (string, error) {
	secs := 2.0
	if v, ok := task.Config["seconds"].(float64); ok {
		secs = v
	}
	// Add jitter so tasks don't all finish at the same time - looks better in the UI
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	time.Sleep(time.Duration(secs*float64(time.Second)) + jitter)
	return fmt.Sprintf("waited %.1fs", secs), nil
}

func (w *Worker) runTransform(task *models.TaskRun) (string, error) {
	input, _ := task.Config["input"].(string)
	op, _ := task.Config["op"].(string)
	switch op {
	case "upper":
		return fmt.Sprintf("transformed: %s", input), nil
	default:
		return fmt.Sprintf("transformed: %s", input), nil
	}
}

// runFailSometimes simulates flaky tasks for demonstrating retry logic.
func (w *Worker) runFailSometimes(task *models.TaskRun) (string, error) {
	failRate := 0.6
	if v, ok := task.Config["failRate"].(float64); ok {
		failRate = v
	}
	// First attempt always fails to make retries visible in the UI
	if task.Retries == 0 && rand.Float64() < failRate {
		return "", fmt.Errorf("simulated failure (retry to succeed)")
	}
	return "succeeded after retry", nil
}
