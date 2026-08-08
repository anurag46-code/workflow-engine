import { useMemo } from 'react'
import {
  ReactFlow,
  Node,
  Edge,
  Background,
  Controls,
  Handle,
  Position,
  NodeProps,
} from '@xyflow/react'
// @ts-ignore
import '@xyflow/react/dist/style.css'
import { WorkflowDef, TaskRun, TaskStatus } from '../types'

const STATUS_COLORS: Record<TaskStatus, string> = {
  pending:  '#444',
  running:  '#3498db',
  success:  '#2ecc71',
  failed:   '#e74c3c',
  retrying: '#f39c12',
}

const STATUS_LABELS: Record<TaskStatus, string> = {
  pending:  'pending',
  running:  'running...',
  success:  'done',
  failed:   'failed',
  retrying: 'retrying',
}

function TaskNode({ data }: NodeProps) {
  const { label, type, status, retries, maxRetries } = data as {
    label: string
    type: string
    status: TaskStatus
    retries: number
    maxRetries: number
  }
  const color = STATUS_COLORS[status] ?? '#444'
  const isRunning = status === 'running'

  return (
    <div style={{
      padding: '10px 16px',
      borderRadius: 8,
      border: `2px solid ${color}`,
      background: '#1a1a1a',
      minWidth: 130,
      boxShadow: isRunning ? `0 0 12px ${color}88` : 'none',
      transition: 'box-shadow 0.3s ease',
    }}>
      <Handle type="target" position={Position.Left} style={{ background: '#555' }} />
      <div style={{ fontWeight: 600, fontSize: 13, color: '#eee', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 11, color: '#888', marginBottom: 6 }}>{type}</div>
      <div style={{
        display: 'inline-block', fontSize: 10, padding: '2px 7px',
        borderRadius: 10, background: color, color: '#fff', fontWeight: 500,
      }}>
        {STATUS_LABELS[status] ?? status}
      </div>
      {retries > 0 && (
        <div style={{ fontSize: 10, color: '#f39c12', marginTop: 4 }}>
          retry {retries}/{maxRetries}
        </div>
      )}
      <Handle type="source" position={Position.Right} style={{ background: '#555' }} />
    </div>
  )
}

const nodeTypes = { task: TaskNode }

interface Props {
  definition: WorkflowDef
  taskRuns: TaskRun[]
}

export function DAGView({ definition, taskRuns }: Props) {
  const taskStatusMap = useMemo(() => {
    const m: Record<string, TaskRun> = {}
    for (const t of taskRuns) m[t.taskId] = t
    return m
  }, [taskRuns])

  // Auto-layout: assign x positions by topological depth, y by index within that layer
  const { nodes, edges } = useMemo(() => {
    const depMap: Record<string, string[]> = {}
    for (const t of definition.tasks) depMap[t.id] = t.dependsOn ?? []

    // BFS to compute layer (depth) for each node
    const depth: Record<string, number> = {}
    const inDegree: Record<string, number> = {}
    for (const t of definition.tasks) inDegree[t.id] = 0
    for (const t of definition.tasks) {
      for (const dep of t.dependsOn ?? []) {
        inDegree[t.id] = (inDegree[t.id] ?? 0) + 1
      }
    }

    const queue = definition.tasks.filter(t => (t.dependsOn?.length ?? 0) === 0).map(t => t.id)
    for (const id of queue) depth[id] = 0

    while (queue.length > 0) {
      const cur = queue.shift()!
      for (const t of definition.tasks) {
        if ((t.dependsOn ?? []).includes(cur)) {
          depth[t.id] = Math.max(depth[t.id] ?? 0, (depth[cur] ?? 0) + 1)
          inDegree[t.id]--
          if (inDegree[t.id] === 0) queue.push(t.id)
        }
      }
    }

    // Group by depth layer and assign y positions
    const layers: Record<number, string[]> = {}
    for (const [id, d] of Object.entries(depth)) {
      if (!layers[d]) layers[d] = []
      layers[d].push(id)
    }

    const nodes: Node[] = definition.tasks.map(t => {
      const run = taskStatusMap[t.id]
      const layer = depth[t.id] ?? 0
      const layerItems = layers[layer] ?? [t.id]
      const yIndex = layerItems.indexOf(t.id)
      return {
        id: t.id,
        type: 'task',
        position: { x: layer * 200, y: yIndex * 110 },
        data: {
          label: t.id,
          type: t.type,
          status: run?.status ?? 'pending',
          retries: run?.retries ?? 0,
          maxRetries: run?.maxRetries ?? t.maxRetries,
        },
      }
    })

    const edges: Edge[] = []
    for (const t of definition.tasks) {
      for (const dep of t.dependsOn ?? []) {
        edges.push({
          id: `${dep}-${t.id}`,
          source: dep,
          target: t.id,
          style: { stroke: '#444' },
          animated: taskStatusMap[dep]?.status === 'running',
        })
      }
    }

    return { nodes, edges }
  }, [definition, taskStatusMap])

  return (
    <div style={{ width: '100%', height: 380, background: '#111', borderRadius: 8, border: '1px solid #2a2a2a' }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#222" gap={20} />
        <Controls style={{ background: '#1a1a1a', border: '1px solid #333' }} />
      </ReactFlow>
    </div>
  )
}
