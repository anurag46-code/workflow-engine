# Workflow Engine

A fault-tolerant DAG-based task orchestration engine with a live React dashboard. Define workflows as directed acyclic graphs, submit them via API, and watch tasks execute in parallel with automatic retries.

Think lightweight Temporal or Apache Airflow - built from scratch in Go.

**Live demo:** http://15.252.165.209:3001

---

## Features

- DAG validation with cycle detection (DFS 3-color marking)
- Postgres-backed worker queue using `SELECT FOR UPDATE SKIP LOCKED` - no double execution across 3 concurrent workers
- 30s fault-tolerant worker leases - tasks auto-reclaimed if a worker crashes mid-execution
- Live DAG visualization via ReactFlow, streamed over SSE
- Text analysis pipeline: word count, keyword extraction, sentiment analysis, SMTP email delivery
- Custom workflow builder in the UI

---

## Tech Stack

| Layer | Choice |
|---|---|
| Backend | Go + Gin |
| Queue | PostgreSQL (`SELECT FOR UPDATE SKIP LOCKED`) |
| Frontend | React + TypeScript + ReactFlow |
| Email | Mailhog (local SMTP) |
| Infra | Docker Compose |

---

## Architecture

```
[React UI] --POST /workflows--> [Go API]
                                     |
                              [Engine: DAG validation,
                               task creation]
                                     |
                              [PostgreSQL queue]
                                     |
                    [Worker 1] [Worker 2] [Worker 3]
                    (poll + claim with SKIP LOCKED)
                                     |
                    [SSE stream] --> [React DAG view]
```

---

## Running Locally

```bash
docker-compose up -d
```

- UI: http://localhost:3001
- API: http://localhost:8081
- Mailhog: http://localhost:8025

---

## Key Design Decisions

**Why Postgres over RabbitMQ/Kafka?**
One less dependency. `SELECT FOR UPDATE SKIP LOCKED` gives exactly-once task claiming with full ACID guarantees. For a workflow engine where tasks have complex dependency logic already stored in Postgres, keeping the queue in the same DB simplifies atomic state transitions.

**Why SSE over WebSockets?**
Workflow status is one-directional - server pushes updates to browser, browser never pushes back. SSE is simpler, works over HTTP/1.1, and auto-reconnects natively.

**Why at-least-once delivery?**
Worker leases (30s `locked_until`) mean a crashed worker's tasks are automatically reclaimed. Tasks are idempotent by design so re-execution is safe.

---

## Interview Topics Covered

- DAG cycle detection and topological ordering
- Distributed task queues and idempotency
- Worker lease expiry and failure recovery
- At-least-once vs exactly-once delivery semantics
- SELECT FOR UPDATE SKIP LOCKED pattern
- SSE vs WebSockets tradeoffs
