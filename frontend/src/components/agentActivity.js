const STATUS_LABELS = {
  running: '运行中',
  succeeded: '已完成',
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

  return sortActivityItems([...agentRuns, ...dialogueRuns])
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
