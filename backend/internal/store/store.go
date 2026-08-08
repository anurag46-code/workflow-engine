package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/anurag46-code/workflow-engine/internal/models"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func New() (*Store, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://workflow:workflow@localhost:5432/workflow_engine?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() { s.db.Close() }

// GetUpstreamOutputs returns taskID->output for all succeeded upstream tasks.
// Workers use this to pass data between pipeline stages.
func (s *Store) GetUpstreamOutputs(workflowID string, dependsOn []string) (map[string]string, error) {
	if len(dependsOn) == 0 {
		return map[string]string{}, nil
	}
	rows, err := s.db.Query(`
		SELECT task_id, output FROM task_runs
		WHERE workflow_id=$1 AND task_id=ANY($2) AND status='success'
	`, workflowID, pq.Array(dependsOn))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var id, out string
		if err := rows.Scan(&id, &out); err != nil {
			return nil, err
		}
		result[id] = out
	}
	return result, nil
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS workflow_runs (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			definition  JSONB NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS task_runs (
			id           TEXT PRIMARY KEY,
			workflow_id  TEXT NOT NULL REFERENCES workflow_runs(id),
			task_id      TEXT NOT NULL,
			type         TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			retries      INT  NOT NULL DEFAULT 0,
			max_retries  INT  NOT NULL DEFAULT 3,
			depends_on   TEXT[] NOT NULL DEFAULT '{}',
			config       JSONB NOT NULL DEFAULT '{}',
			output       TEXT NOT NULL DEFAULT '',
			error        TEXT NOT NULL DEFAULT '',
			locked_by    TEXT NOT NULL DEFAULT '',
			locked_until TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_task_runs_workflow ON task_runs(workflow_id);
		CREATE INDEX IF NOT EXISTS idx_task_runs_status   ON task_runs(status);
	`)
	return err
}

func (s *Store) CreateWorkflowRun(run *models.WorkflowRun) error {
	def, err := json.Marshal(run.Definition)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO workflow_runs (id, name, status, definition, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, run.ID, run.Name, run.Status, def)
	return err
}

func (s *Store) GetWorkflowRun(id string) (*models.WorkflowRun, error) {
	row := s.db.QueryRow(`SELECT id, name, status, definition, created_at, updated_at FROM workflow_runs WHERE id=$1`, id)
	return scanWorkflow(row)
}

func (s *Store) ListWorkflowRuns() ([]*models.WorkflowRun, error) {
	rows, err := s.db.Query(`SELECT id, name, status, definition, created_at, updated_at FROM workflow_runs ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []*models.WorkflowRun
	for rows.Next() {
		run, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (s *Store) UpdateWorkflowStatus(id string, status models.WorkflowStatus) error {
	_, err := s.db.Exec(`UPDATE workflow_runs SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (s *Store) CreateTaskRun(t *models.TaskRun) error {
	cfg, err := json.Marshal(t.Config)
	if err != nil {
		return err
	}
	deps := t.DependsOn
	if deps == nil {
		deps = []string{}
	}
	_, err = s.db.Exec(`
		INSERT INTO task_runs (id, workflow_id, task_id, type, status, retries, max_retries, depends_on, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, t.ID, t.WorkflowID, t.TaskID, t.Type, t.Status, t.Retries, t.MaxRetries, pq.Array(deps), cfg)
	return err
}

func (s *Store) GetTaskRunsByWorkflow(workflowID string) ([]*models.TaskRun, error) {
	rows, err := s.db.Query(`SELECT id, workflow_id, task_id, type, status, retries, max_retries, depends_on, config, output, error, locked_by, locked_until, created_at, updated_at FROM task_runs WHERE workflow_id=$1`, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*models.TaskRun
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// ClaimTask is the core of the Postgres queue pattern.
// SELECT FOR UPDATE SKIP LOCKED atomically finds a pending task whose
// dependencies are all done and locks it for this worker - no two workers
// can claim the same task simultaneously.
func (s *Store) ClaimTask(workerID string, leaseDuration time.Duration) (*models.TaskRun, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Only claim a task if every task it depends on has status='success'.
	// The subquery checks: for each task_id in this task's depends_on array,
	// there must exist a row in the same workflow with that task_id and status='success'.
	// If depends_on is empty the NOT EXISTS clause is vacuously false - task is ready.
	row := tx.QueryRow(`
		SELECT id, workflow_id, task_id, type, status, retries, max_retries, depends_on, config, output, error, locked_by, locked_until, created_at, updated_at
		FROM task_runs t
		WHERE status IN ('pending', 'retrying')
		  AND (locked_until IS NULL OR locked_until < NOW())
		  AND NOT EXISTS (
		    SELECT 1 FROM unnest(t.depends_on) AS dep_task_id
		    WHERE NOT EXISTS (
		      SELECT 1 FROM task_runs dep
		      WHERE dep.workflow_id = t.workflow_id
		        AND dep.task_id = dep_task_id
		        AND dep.status = 'success'
		    )
		  )
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`)

	task, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	lockedUntil := time.Now().Add(leaseDuration)
	_, err = tx.Exec(`
		UPDATE task_runs SET status='running', locked_by=$1, locked_until=$2, updated_at=NOW() WHERE id=$3
	`, workerID, lockedUntil, task.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	task.Status = models.TaskRunning
	task.LockedBy = workerID
	task.LockedUntil = &lockedUntil
	return task, nil
}

func (s *Store) CompleteTask(id, output string) error {
	_, err := s.db.Exec(`
		UPDATE task_runs SET status='success', output=$1, locked_by='', locked_until=NULL, updated_at=NOW() WHERE id=$2
	`, output, id)
	return err
}

func (s *Store) FailTask(id, errMsg string, retries, maxRetries int) error {
	newRetries := retries + 1
	status := "failed"
	if retries < maxRetries {
		status = "retrying"
	} else {
		newRetries = retries // don't increment past maxRetries
	}
	_, err := s.db.Exec(`
		UPDATE task_runs SET status=$1, error=$2, retries=$3, locked_by='', locked_until=NULL, updated_at=NOW() WHERE id=$4
	`, status, errMsg, newRetries, id)
	return err
}

// helpers

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkflow(s scanner) (*models.WorkflowRun, error) {
	var r models.WorkflowRun
	var defRaw []byte
	err := s.Scan(&r.ID, &r.Name, &r.Status, &defRaw, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(defRaw, &r.Definition); err != nil {
		return nil, err
	}
	return &r, nil
}

func scanTask(s scanner) (*models.TaskRun, error) {
	var t models.TaskRun
	var cfgRaw []byte
	err := s.Scan(&t.ID, &t.WorkflowID, &t.TaskID, &t.Type, &t.Status, &t.Retries, &t.MaxRetries, pq.Array(&t.DependsOn), &cfgRaw, &t.Output, &t.Error, &t.LockedBy, &t.LockedUntil, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cfgRaw, &t.Config); err != nil {
		t.Config = map[string]any{}
	}
	return &t, nil
}
