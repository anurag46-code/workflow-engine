import { useState, useEffect } from 'react'
import { WorkflowRun } from './types'
import { DAGView } from './components/DAGView'
import { useWorkflowStream } from './hooks/useWorkflowStream'

const API = import.meta.env.DEV ? 'http://localhost:8080' : ''

const STATUS_COLOR: Record<string, string> = {
  pending: '#888',
  running: '#3498db',
  success: '#2ecc71',
  failed:  '#e74c3c',
}

export default function App() {
  const [runs, setRuns] = useState<WorkflowRun[]>([])
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [launching, setLaunching] = useState(false)

  const { workflow, tasks } = useWorkflowStream(selectedID)

  useEffect(() => {
    fetchRuns()
    const t = setInterval(fetchRuns, 3000)
    return () => clearInterval(t)
  }, [])

  // Auto-select the most recently launched workflow
  useEffect(() => {
    if (runs.length > 0 && !selectedID) setSelectedID(runs[0].id)
  }, [runs])

  async function fetchRuns() {
    const res = await fetch(`${API}/workflows`)
    if (res.ok) setRuns(await res.json())
  }

  async function launchDemo() {
    setLaunching(true)
    const res = await fetch(`${API}/workflows/demo`, { method: 'POST' })
    if (res.ok) {
      const run: WorkflowRun = await res.json()
      setRuns(prev => [run, ...prev])
      setSelectedID(run.id)
    }
    setLaunching(false)
  }

  const displayed = workflow ?? runs.find(r => r.id === selectedID) ?? null

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      {/* Sidebar */}
      <div style={{
        width: 260, flexShrink: 0, background: '#141414', borderRight: '1px solid #222',
        display: 'flex', flexDirection: 'column',
      }}>
        <div style={{ padding: '16px 14px', borderBottom: '1px solid #222' }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: '#fff', marginBottom: 2 }}>Workflow Engine</div>
          <div style={{ fontSize: 11, color: '#666' }}>DAG-based task orchestrator</div>
        </div>

        <div style={{ padding: '10px 14px' }}>
          <button
            onClick={launchDemo}
            disabled={launching}
            style={{
              width: '100%', padding: '9px', borderRadius: 6,
              background: launching ? '#2a2a2a' : '#3498db',
              border: 'none', color: '#fff', fontSize: 13,
              cursor: launching ? 'not-allowed' : 'pointer', fontWeight: 500,
            }}
          >
            {launching ? 'Launching...' : '+ Run Demo Workflow'}
          </button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '0 8px 8px' }}>
          {runs.map(run => (
            <div
              key={run.id}
              onClick={() => setSelectedID(run.id)}
              style={{
                padding: '9px 10px', borderRadius: 6, cursor: 'pointer', marginBottom: 2,
                background: selectedID === run.id ? '#1e1e1e' : 'transparent',
                border: `1px solid ${selectedID === run.id ? '#2a2a2a' : 'transparent'}`,
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontSize: 13, color: '#ddd', fontWeight: 500 }}>{run.name}</span>
                <span style={{
                  fontSize: 10, padding: '2px 6px', borderRadius: 8,
                  background: STATUS_COLOR[run.status] + '33',
                  color: STATUS_COLOR[run.status],
                }}>
                  {run.status}
                </span>
              </div>
              <div style={{ fontSize: 10, color: '#555', marginTop: 3, fontFamily: 'monospace' }}>
                {run.id.slice(0, 8)}
              </div>
            </div>
          ))}
          {runs.length === 0 && (
            <div style={{ color: '#555', fontSize: 12, textAlign: 'center', marginTop: 24 }}>
              No workflows yet.<br />Click "Run Demo" to start.
            </div>
          )}
        </div>
      </div>

      {/* Main panel */}
      <div style={{ flex: 1, overflow: 'auto', padding: 24 }}>
        {displayed ? (
          <>
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
              <h2 style={{ color: '#fff', fontSize: 18, fontWeight: 700 }}>{displayed.name}</h2>
              <span style={{
                fontSize: 12, padding: '3px 10px', borderRadius: 10,
                background: (STATUS_COLOR[displayed.status] ?? '#888') + '33',
                color: STATUS_COLOR[displayed.status] ?? '#888',
              }}>
                {displayed.status}
              </span>
              <span style={{ color: '#555', fontSize: 12, fontFamily: 'monospace' }}>
                {displayed.id.slice(0, 12)}
              </span>
            </div>

            {/* DAG */}
            <DAGView definition={displayed.definition} taskRuns={tasks} />

            {/* Task table */}
            <div style={{ marginTop: 20 }}>
              <div style={{ fontSize: 13, color: '#888', marginBottom: 10, fontWeight: 500 }}>Tasks</div>
              <div style={{ border: '1px solid #1e1e1e', borderRadius: 8, overflow: 'hidden' }}>
                {tasks.length === 0 ? (
                  <div style={{ padding: 16, color: '#555', fontSize: 13 }}>Loading tasks...</div>
                ) : tasks.map((t, i) => (
                  <div key={t.id} style={{
                    display: 'grid', gridTemplateColumns: '1fr 80px 60px 1fr',
                    padding: '10px 14px', alignItems: 'center',
                    background: i % 2 === 0 ? '#111' : '#131313',
                    borderBottom: '1px solid #1a1a1a', gap: 12,
                  }}>
                    <div>
                      <span style={{ color: '#ddd', fontSize: 13, fontWeight: 500 }}>{t.taskId}</span>
                      <span style={{ color: '#555', fontSize: 11, marginLeft: 8 }}>{t.type}</span>
                    </div>
                    <span style={{
                      fontSize: 11, padding: '2px 8px', borderRadius: 8, textAlign: 'center',
                      background: (STATUS_COLOR[t.status] ?? '#888') + '33',
                      color: STATUS_COLOR[t.status] ?? '#888',
                    }}>
                      {t.status}
                    </span>
                    <span style={{ color: '#555', fontSize: 11 }}>
                      {t.retries > 0 ? `retry ${t.retries}/${t.maxRetries}` : ''}
                    </span>
                    <span style={{ color: '#666', fontSize: 11, fontFamily: 'monospace', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {t.error || t.output}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </>
        ) : (
          <div style={{ color: '#555', fontSize: 14, marginTop: 40, textAlign: 'center' }}>
            Select a workflow or run a demo to get started.
          </div>
        )}
      </div>
    </div>
  )
}
