export type WorkflowStatus = 'pending' | 'running' | 'success' | 'failed'
export type TaskStatus = 'pending' | 'running' | 'success' | 'failed' | 'retrying'

export interface TaskDef {
  id: string
  type: string
  dependsOn: string[]
  config: Record<string, unknown>
  maxRetries: number
}

export interface WorkflowDef {
  name: string
  tasks: TaskDef[]
}

export interface WorkflowRun {
  id: string
  name: string
  status: WorkflowStatus
  definition: WorkflowDef
  createdAt: string
  updatedAt: string
}

export interface TaskRun {
  id: string
  workflowId: string
  taskId: string
  type: string
  status: TaskStatus
  retries: number
  maxRetries: number
  output: string
  error: string
  createdAt: string
  updatedAt: string
}
