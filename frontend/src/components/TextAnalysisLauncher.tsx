import { useState } from 'react'
import { WorkflowRun } from '../types'

const API = import.meta.env.DEV ? 'http://localhost:8080' : ''

const SAMPLE_TEXT = `Artificial intelligence is transforming how we build software systems.
Modern distributed architectures leverage innovative techniques to achieve outstanding performance
and reliability. However, poor design can lead to terrible failure modes and slow, broken systems.

The best engineers focus on building excellent, efficient solutions that solve real problems.
They avoid unnecessary complexity and embrace positive collaboration. This approach leads to
brilliant outcomes and successful products that users love.`

interface Props {
  onLaunched: (run: WorkflowRun) => void
  onCancel: () => void
}

export function TextAnalysisLauncher({ onLaunched, onCancel }: Props) {
  const [text, setText] = useState(SAMPLE_TEXT)
  const [email, setEmail] = useState('demo@example.com')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    if (!text.trim()) { setError('Text is required'); return }
    setError('')
    setLoading(true)
    const res = await fetch(`${API}/workflows/analyze`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text, email }),
    })
    setLoading(false)
    if (res.ok) {
      onLaunched(await res.json())
    } else {
      const d = await res.json()
      setError(d.error ?? 'Failed to submit')
    }
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, background: '#000000bb',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100,
    }}>
      <div style={{
        background: '#141414', border: '1px solid #2a2a2a', borderRadius: 10,
        width: 580, maxHeight: '88vh', display: 'flex', flexDirection: 'column',
      }}>
        <div style={{ padding: '16px 20px', borderBottom: '1px solid #222' }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: '#fff' }}>Text Analysis Pipeline</div>
          <div style={{ fontSize: 12, color: '#666', marginTop: 3 }}>
            ingest → word-count + keywords + sentiment (parallel) → report → email
          </div>
        </div>

        <div style={{ overflowY: 'auto', padding: '16px 20px', flex: 1 }}>
          <label style={labelStyle}>Your Text</label>
          <textarea
            value={text}
            onChange={e => setText(e.target.value)}
            rows={10}
            style={{
              width: '100%', padding: '10px 12px', background: '#1a1a1a',
              border: '1px solid #333', borderRadius: 6, color: '#ddd',
              fontSize: 13, resize: 'vertical', outline: 'none', fontFamily: 'inherit',
              lineHeight: 1.6,
            }}
          />
          <div style={{ fontSize: 11, color: '#555', marginTop: 4 }}>
            {text.trim().split(/\s+/).filter(Boolean).length} words
          </div>

          <label style={{ ...labelStyle, marginTop: 16 }}>Notify email</label>
          <input
            value={email}
            onChange={e => setEmail(e.target.value)}
            placeholder="demo@example.com"
            style={{
              width: '100%', padding: '8px 12px', background: '#1a1a1a',
              border: '1px solid #333', borderRadius: 6, color: '#ddd',
              fontSize: 13, outline: 'none',
            }}
          />
          <div style={{ fontSize: 11, color: '#555', marginTop: 4 }}>
            Email is delivered to{' '}
            <a href="http://localhost:8025" target="_blank" rel="noreferrer"
              style={{ color: '#3498db' }}>
              Mailhog (localhost:8025)
            </a>
            {' '}— no real email sent.
          </div>
        </div>

        <div style={{ padding: '12px 20px', borderTop: '1px solid #222' }}>
          {error && <div style={{ color: '#e74c3c', fontSize: 12, marginBottom: 10 }}>{error}</div>}
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button onClick={onCancel} style={ghostBtn}>Cancel</button>
            <button onClick={submit} disabled={loading} style={{
              padding: '8px 22px', borderRadius: 6, border: 'none',
              background: loading ? '#2a2a2a' : '#2ecc71',
              color: '#fff', fontSize: 13, cursor: loading ? 'not-allowed' : 'pointer', fontWeight: 600,
            }}>
              {loading ? 'Submitting...' : 'Run Pipeline'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 11, color: '#666', marginBottom: 6, fontWeight: 500,
}
const ghostBtn: React.CSSProperties = {
  padding: '8px 16px', borderRadius: 6, border: '1px solid #333',
  background: 'none', color: '#888', fontSize: 13, cursor: 'pointer',
}
