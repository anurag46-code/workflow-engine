import { useEffect, useRef, useState } from 'react'
import { WorkflowRun, TaskRun } from '../types'

interface StreamState {
  workflow: WorkflowRun | null
  tasks: TaskRun[]
  done: boolean
}

// useWorkflowStream subscribes to the SSE endpoint for a workflow.
// SSE (Server-Sent Events) is used here instead of WebSockets because
// this is a one-directional stream (server pushes, client only reads).
// SSE is simpler: it uses plain HTTP, reconnects automatically, and
// doesn't need a handshake protocol.
export function useWorkflowStream(workflowID: string | null): StreamState {
  const [state, setState] = useState<StreamState>({ workflow: null, tasks: [], done: false })
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!workflowID) return

    setState({ workflow: null, tasks: [], done: false })

    const apiBase = import.meta.env.DEV ? 'http://localhost:8080' : ''
    const es = new EventSource(`${apiBase}/workflows/${workflowID}/stream`)
    esRef.current = es

    es.addEventListener('update', (e) => {
      const data = JSON.parse(e.data)
      setState(prev => ({ ...prev, workflow: data.workflow, tasks: data.tasks ?? [] }))
    })

    es.addEventListener('done', () => {
      setState(prev => ({ ...prev, done: true }))
      es.close()
    })

    es.onerror = () => {
      es.close()
    }

    return () => {
      es.close()
    }
  }, [workflowID])

  return state
}
