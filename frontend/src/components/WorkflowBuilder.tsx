import { useState } from 'react'
import { WorkflowDef, TaskDef } from '../types'

interface Props {
  onSubmit: (def: WorkflowDef) => void
  onCancel: () => void
}

const TASK_TYPES = ['wait', 'transform', 'fail_sometimes']

const emptyTask = (): TaskDef => ({
  id: '',
  type: 'wait',
  dependsOn: [],
  config: { seconds: 2 },
  maxRetries: 3,
})

export function WorkflowBuilder({ onSubmit, onCancel }: Props) {
  const [name, setName] = useState('My Workflow')
  const [tasks, setTasks] = useState<TaskDef[]>([emptyTask()])
  const [error, setError] = useState('')

  const updateTask = (i: number, patch: Partial<TaskDef>) => {
    setTasks(prev => prev.map((t, idx) => idx === i ? { ...t, ...patch } : t))
  }

  const updateConfig = (i: number, key: string, value: unknown) => {
    setTasks(prev => prev.map((t, idx) =>
      idx === i ? { ...t, config: { ...t.config, [key]: value } } : t
    ))
  }

  const addTask = () => setTasks(prev => [...prev, emptyTask()])
  const removeTask = (i: number) => setTasks(prev => prev.filter((_, idx) => idx !== i))

  const toggleDep = (taskIdx: number, depID: string) => {
    setTasks(prev => prev.map((t, idx) => {
      if (idx !== taskIdx) return t
      const has = t.dependsOn.includes(depID)
      return { ...t, dependsOn: has ? t.dependsOn.filter(d => d !== depID) : [...t.dependsOn, depID] }
    }))
  }

  const submit = () => {
    setError('')
    if (!name.trim()) { setError('Workflow name is required'); return }
    for (const t of tasks) {
      if (!t.id.trim()) { setError('All tasks must have an ID'); return }
      if (!/^[a-z0-9-]+$/.test(t.id)) { setError(`Task ID "${t.id}" must be lowercase letters, numbers, hyphens only`); return }
    }
    const ids = tasks.map(t => t.id)
    if (new Set(ids).size !== ids.length) { setError('Task IDs must be unique'); return }
    onSubmit({ name, tasks })
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, background: '#000000aa',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100,
    }}>
      <div style={{
        background: '#141414', border: '1px solid #2a2a2a', borderRadius: 10,
        width: 560, maxHeight: '85vh', display: 'flex', flexDirection: 'column',
        overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ padding: '16px 20px', borderBottom: '1px solid #222', flexShrink: 0 }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: '#fff' }}>New Workflow</div>
        </div>

        <div style={{ overflowY: 'auto', padding: '16px 20px', flex: 1 }}>
          {/* Name */}
          <div style={{ marginBottom: 20 }}>
            <label style={labelStyle}>Workflow Name</label>
            <input value={name} onChange={e => setName(e.target.value)} style={inputStyle} />
          </div>

          {/* Tasks */}
          <div style={{ fontSize: 12, color: '#888', fontWeight: 600, marginBottom: 10, textTransform: 'uppercase', letterSpacing: 1 }}>
            Tasks
          </div>

          {tasks.map((task, i) => (
            <div key={i} style={{
              border: '1px solid #2a2a2a', borderRadius: 8, padding: 14,
              marginBottom: 10, background: '#111',
            }}>
              <div style={{ display: 'flex', gap: 8, marginBottom: 10 }}>
                <div style={{ flex: 1 }}>
                  <label style={labelStyle}>Task ID</label>
                  <input
                    value={task.id}
                    placeholder="e.g. fetch-data"
                    onChange={e => updateTask(i, { id: e.target.value.toLowerCase().replace(/\s/g, '-') })}
                    style={inputStyle}
                  />
                </div>
                <div style={{ flex: 1 }}>
                  <label style={labelStyle}>Type</label>
                  <select
                    value={task.type}
                    onChange={e => updateTask(i, { type: e.target.value, config: defaultConfig(e.target.value) })}
                    style={inputStyle}
                  >
                    {TASK_TYPES.map(t => <option key={t} value={t}>{t}</option>)}
                  </select>
                </div>
                <div style={{ width: 60 }}>
                  <label style={labelStyle}>Retries</label>
                  <input
                    type="number" min={0} max={10}
                    value={task.maxRetries}
                    onChange={e => updateTask(i, { maxRetries: parseInt(e.target.value) || 0 })}
                    style={inputStyle}
                  />
                </div>
              </div>

              {/* Type-specific config */}
              {task.type === 'wait' && (
                <div style={{ marginBottom: 10 }}>
                  <label style={labelStyle}>Duration (seconds)</label>
                  <input
                    type="number" min={1} max={30}
                    value={(task.config.seconds as number) ?? 2}
                    onChange={e => updateConfig(i, 'seconds', parseFloat(e.target.value) || 1)}
                    style={{ ...inputStyle, width: 100 }}
                  />
                </div>
              )}
              {task.type === 'fail_sometimes' && (
                <div style={{ marginBottom: 10 }}>
                  <label style={labelStyle}>Fail rate (0-1, e.g. 0.7 = 70% chance)</label>
                  <input
                    type="number" min={0} max={1} step={0.1}
                    value={(task.config.failRate as number) ?? 0.7}
                    onChange={e => updateConfig(i, 'failRate', parseFloat(e.target.value) || 0)}
                    style={{ ...inputStyle, width: 100 }}
                  />
                </div>
              )}
              {task.type === 'transform' && (
                <div style={{ marginBottom: 10 }}>
                  <label style={labelStyle}>Input text</label>
                  <input
                    value={(task.config.input as string) ?? ''}
                    onChange={e => updateConfig(i, 'input', e.target.value)}
                    style={inputStyle}
                  />
                </div>
              )}

              {/* Dependencies */}
              {i > 0 && (
                <div>
                  <label style={labelStyle}>Depends on</label>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {tasks.slice(0, i).map(dep => dep.id ? (
                      <button
                        key={dep.id}
                        onClick={() => toggleDep(i, dep.id)}
                        style={{
                          padding: '3px 10px', borderRadius: 12, fontSize: 11, cursor: 'pointer', border: 'none',
                          background: task.dependsOn.includes(dep.id) ? '#3498db' : '#2a2a2a',
                          color: task.dependsOn.includes(dep.id) ? '#fff' : '#888',
                        }}
                      >
                        {dep.id}
                      </button>
                    ) : null)}
                  </div>
                </div>
              )}

              {tasks.length > 1 && (
                <button
                  onClick={() => removeTask(i)}
                  style={{ marginTop: 10, background: 'none', border: 'none', color: '#e74c3c', fontSize: 12, cursor: 'pointer', padding: 0 }}
                >
                  remove task
                </button>
              )}
            </div>
          ))}

          <button onClick={addTask} style={{
            width: '100%', padding: 8, borderRadius: 6, border: '1px dashed #333',
            background: 'none', color: '#666', fontSize: 13, cursor: 'pointer', marginBottom: 4,
          }}>
            + Add Task
          </button>
        </div>

        {/* Footer */}
        <div style={{ padding: '12px 20px', borderTop: '1px solid #222', flexShrink: 0 }}>
          {error && <div style={{ color: '#e74c3c', fontSize: 12, marginBottom: 10 }}>{error}</div>}
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button onClick={onCancel} style={{ ...ghostBtn }}>Cancel</button>
            <button onClick={submit} style={{
              padding: '7px 20px', borderRadius: 6, border: 'none',
              background: '#3498db', color: '#fff', fontSize: 13, cursor: 'pointer', fontWeight: 500,
            }}>
              Submit
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function defaultConfig(type: string): Record<string, unknown> {
  if (type === 'wait') return { seconds: 2 }
  if (type === 'fail_sometimes') return { failRate: 0.7 }
  if (type === 'transform') return { input: 'hello world', op: 'upper' }
  return {}
}

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 11, color: '#666', marginBottom: 4,
}

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '6px 10px', background: '#1a1a1a', border: '1px solid #333',
  borderRadius: 5, color: '#ddd', fontSize: 13, outline: 'none',
}

const ghostBtn: React.CSSProperties = {
  padding: '7px 16px', borderRadius: 6, border: '1px solid #333',
  background: 'none', color: '#888', fontSize: 13, cursor: 'pointer',
}
