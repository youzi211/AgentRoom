const STATUS_LABELS = {
  running: '运行中',
  created: '等待开始',
  succeeded: '已完成',
  stopped: '已停止',
  cancelled: '已取消',
  interrupted: '已中断',
  failed: '失败',
  timeout: '超时',
  stopped_limit: '达到轮次上限',
  stopped_duplicate: '重复内容停止',
  stopped_empty: '空回复停止',
}

export function labelForActivityStatus(status = '') {
  return STATUS_LABELS[status] || '未知状态'
}

export function descriptionForActivity(activity = {}) {
  if (activity.kind === 'collaboration_run') {
    return descriptionForCollaboration(activity)
  }
  if (activity.kind === 'dialogue_run') {
    return activity.turnCount > 0 ? `多 Agent 对话，已完成 ${activity.turnCount} 轮` : '多 Agent 对话已开始'
  }

  const name = activity.agentName || activity.agentID || 'Agent'
  return `${name} 响应触发消息`
}

export function normalizeActivityPayload(payload = {}) {
  const agentRuns = (payload.agentRuns ?? []).map((run) => ({
    kind: 'agent_run',
    phase: run.completedAt ? 'finished' : 'started',
    ...run,
  }))
  const dialogueRuns = (payload.dialogueRuns ?? []).map((run) => ({
    kind: 'dialogue_run',
    phase: run.completedAt ? 'finished' : 'started',
    ...run,
  }))
  const collaborationRuns = (payload.collaborationRuns ?? []).map((run) => ({
    kind: 'collaboration_run',
    phase: run.completedAt ? 'finished' : 'started',
    ...run,
  }))

  return sortActivityItems([...agentRuns, ...dialogueRuns, ...collaborationRuns])
}

const COLLABORATION_EVENT_KINDS = new Set([
  'collaboration_started',
  'speaker_selected',
  'agent_turn_started',
  'model_started',
  'model_completed',
  'tool_started',
  'tool_completed',
  'tool_failed',
  'artifact_ready',
  'handoff_requested',
  'agent_message_completed',
  'completed',
  'stopped',
  'cancelled',
  'failed',
])

export function mergeCollaborationActivityEvent(current = [], eventActivity = null) {
  if (!eventActivity?.collaborationRunID || !COLLABORATION_EVENT_KINDS.has(eventActivity.kind)) {
    return current
  }

  const sequence = Number(eventActivity.sequence)
  if (!Number.isSafeInteger(sequence) || sequence < 1) {
    return current
  }

  const id = eventActivity.collaborationRunID
  const existing = current.find((item) => item.kind === 'collaboration_run' && item.id === id)
  if ((existing?.sequence ?? 0) >= sequence) {
    return current
  }

  const terminal = ['completed', 'stopped', 'cancelled', 'failed'].includes(eventActivity.kind)
  const next = {
    ...(existing ?? {}),
    kind: 'collaboration_run',
    phase: terminal ? 'finished' : 'started',
    id,
    roomID: eventActivity.roomID || existing?.roomID,
    triggerMessageID: eventActivity.triggerMessageID || existing?.triggerMessageID,
    status: statusForCollaborationEvent(eventActivity.kind),
    latestEvent: eventActivity.kind,
    sequence,
    turnID: eventActivity.turnID || existing?.turnID,
    agentID: eventActivity.agentID || existing?.agentID,
    agentName: eventActivity.agentName || existing?.agentName,
    targetAgentID: eventActivity.targetAgentID || '',
    targetAgentName: eventActivity.targetAgentName || '',
    reasonCategory: eventActivity.reasonCategory || '',
    stopReason: eventActivity.stopReason || existing?.stopReason,
    errorCode: eventActivity.errorCode || existing?.errorCode,
    toolName: eventActivity.toolName || '',
    artifactID: eventActivity.artifactID || '',
    turnCount: eventActivity.turnCount ?? existing?.turnCount ?? 0,
    createdAt: existing?.createdAt || eventActivity.occurredAt,
    completedAt: terminal ? eventActivity.occurredAt : existing?.completedAt,
  }

  return mergeActivityEvent(current, next)
}

export function mergeActivityEvent(current = [], eventActivity = null) {
  if (!eventActivity?.kind || !eventActivity?.id) {
    return current
  }

  const byKey = new Map(current.map((item) => [activityKey(item), item]))
  const key = activityKey(eventActivity)
  byKey.set(key, {
    ...(byKey.get(key) ?? {}),
    ...eventActivity,
  })

  return sortActivityItems(Array.from(byKey.values()))
}

export function sortActivityItems(items = []) {
  return [...items].sort((left, right) => {
    const leftRunning = isRunning(left)
    const rightRunning = isRunning(right)
    if (leftRunning !== rightRunning) {
      return leftRunning ? -1 : 1
    }

    return timestampFor(right) - timestampFor(left)
  })
}

function activityKey(activity) {
  return `${activity.kind}:${activity.id}`
}

function isRunning(activity) {
  return activity.status === 'running' || !activity.completedAt
}

function timestampFor(activity) {
  const timestamp = Date.parse(activity.createdAt || activity.completedAt || '')
  return Number.isNaN(timestamp) ? 0 : timestamp
}

function statusForCollaborationEvent(kind) {
  switch (kind) {
    case 'completed':
      return 'succeeded'
    case 'stopped':
      return 'stopped'
    case 'cancelled':
      return 'cancelled'
    case 'failed':
      return 'failed'
    default:
      return 'running'
  }
}

function descriptionForCollaboration(activity) {
  const agentName = activity.agentName || activity.agentID || 'Agent'
  const targetName = activity.targetAgentName || activity.targetAgentID || '下一位 Agent'

  switch (activity.latestEvent) {
    case 'speaker_selected':
      return `${agentName} 已选为当前发言者`
    case 'agent_turn_started':
    case 'model_started':
      return `${agentName} 正在响应`
    case 'model_completed':
      return `${agentName} 已完成模型调用`
    case 'tool_started':
      return `${agentName} 正在使用 ${activity.toolName || '工具'}`
    case 'tool_completed':
      return `${agentName} 已完成工具调用`
    case 'tool_failed':
      return `${agentName} 的工具调用失败`
    case 'artifact_ready':
      return `${agentName} 已生成产物`
    case 'handoff_requested':
      return `${agentName} 将协作交给 ${targetName}`
    case 'agent_message_completed':
      return `${agentName} 已完成本轮发言`
    case 'completed':
      return `协作完成，共 ${activity.turnCount ?? 0} 轮`
    case 'stopped':
      return `协作已停止：${labelForStopReason(activity.stopReason)}`
    case 'cancelled':
      return '协作已取消'
    case 'failed':
      return activity.errorCode ? `协作失败：${activity.errorCode}` : '协作失败'
    case 'collaboration_started':
      return '协作运行已开始'
    default: {
      if (activity.status === 'succeeded') {
        return `协作完成，共 ${activity.turnCount ?? 0} 轮`
      }
      if (activity.status === 'stopped') {
        return `协作已停止：${labelForStopReason(activity.stopReason)}`
      }
      if (activity.status === 'cancelled') {
        return '协作已取消'
      }
      if (activity.status === 'timeout') {
        return '协作运行超时'
      }
      if (activity.status === 'interrupted') {
        return '协作运行已中断'
      }
      if (activity.status === 'failed') {
        return activity.errorText ? `协作失败：${activity.errorText}` : '协作失败'
      }
      const engine = activity.engine || '协作引擎'
      return activity.turnCount > 0 ? `${engine} 已完成 ${activity.turnCount} 轮` : `${engine} 协作运行`
    }
  }
}

function labelForStopReason(reason) {
  switch (reason) {
    case 'max_turns':
      return '达到总轮次上限'
    case 'max_turns_per_agent':
      return '达到单 Agent 轮次上限'
    case 'empty_output':
      return '空回复'
    case 'duplicate_output':
      return '重复回复'
    case 'no_eligible_agent':
      return '没有可用 Agent'
    case 'deadline_exceeded':
      return '运行超时'
    case 'interrupted':
      return '运行中断'
    default:
      return reason || '策略终止'
  }
}
