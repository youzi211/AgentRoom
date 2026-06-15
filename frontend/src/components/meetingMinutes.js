const DEFAULT_TITLE = '会议纪要'

export function normalizeMinutesPayload(payload) {
  if (typeof payload === 'string') {
    return payload
  }
  if (!payload || typeof payload !== 'object') {
    return ''
  }

  return payload.markdown || payload.content || payload.minutes?.markdown || payload.minutes?.content || ''
}

export function buildLocalMeetingMinutesMarkdown({ room, roomId, participants = [], agents = [], focusPoints = [], messages = [] }) {
  const title = room?.name?.trim() || DEFAULT_TITLE
  const lines = [`# ${title}`, '']

  lines.push('## 基本信息')
  lines.push(`- 房间 ID：\`${roomId}\``)
  lines.push(`- 导出时间：${formatDateTime(new Date())}`)
  lines.push(`- 参会者：${joinNames(participants) || '暂无'}`)
  lines.push(`- Agent：${joinNames(agents) || '暂无'}`)
  lines.push('')

  lines.push('## 会议焦点')
  if (focusPoints.length === 0) {
    lines.push('- 暂无自动提取的焦点。')
  } else {
    focusPoints.forEach((point) => {
      const category = point.category || '焦点'
      lines.push(`- **${category}**：${point.content || ''}`)
    })
  }
  lines.push('')

  lines.push('## 消息记录')
  if (messages.length === 0) {
    lines.push('- 暂无消息。')
  } else {
    messages.forEach((message) => {
      lines.push(`- **${message.senderName || '未知成员'}**：${message.content || ''}`)
    })
  }
  lines.push('')

  return lines.join('\n')
}

export function downloadMarkdownFile(markdown, filename) {
  const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

export function minutesFilename(room, roomId) {
  const safeName = (room?.name || roomId || 'meeting').replace(/[\\/:*?"<>|]+/g, '-').trim() || 'meeting'
  return `${safeName}-会议纪要.md`
}

function joinNames(items) {
  return items
    .map((item) => item.name)
    .filter(Boolean)
    .join('、')
}

function formatDateTime(date) {
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}
