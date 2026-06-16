const PARTICIPANT_ROLE_LABEL = '参会者'

export function buildMentionTargets({ agents = [], participants = [], currentParticipantName = '' }) {
  const currentNameKey = normalizeKey(currentParticipantName)
  const seenMentions = new Set()
  const targets = []

  agents.forEach((agent) => {
    const mention = String(agent.mention || '').trim() || `@${String(agent.name || '').trim()}`
    const mentionKey = normalizeKey(mention)
    if (!mentionKey || seenMentions.has(mentionKey)) {
      return
    }

    targets.push({
      id: agent.id,
      kind: 'agent',
      mention,
      name: String(agent.name || '').trim() || mention.replace(/^[@＠]/, ''),
      role: agent.role || '',
    })
    seenMentions.add(mentionKey)
  })

  participants.forEach((participant) => {
    const name = String(participant.name || '').trim()
    if (!name || normalizeKey(name) === currentNameKey) {
      return
    }

    const mention = `@${name}`
    const mentionKey = normalizeKey(mention)
    if (seenMentions.has(mentionKey)) {
      return
    }

    targets.push({
      id: participant.id,
      kind: 'participant',
      mention,
      name,
      role: PARTICIPANT_ROLE_LABEL,
    })
    seenMentions.add(mentionKey)
  })

  return targets
}

export function filterMentionTargets(targets = [], query = '') {
  const normalizedQuery = normalizeKey(query)
  return targets.filter((target) => {
    if (!normalizedQuery) {
      return true
    }

    return normalizeKey(target.name).includes(normalizedQuery) || normalizeKey(target.mention).includes(normalizedQuery)
  })
}

export function findMentionQuery(content = '', cursorPosition = 0) {
  return resolveMentionTrigger(content, cursorPosition)?.query ?? null
}

export function replaceMentionQuery(content = '', cursorPosition = 0, mention = '') {
  const trigger = resolveMentionTrigger(content, cursorPosition)
  if (!trigger) {
    return {
      content,
      cursorPosition,
    }
  }

  const suffix = content.substring(cursorPosition)
  return {
    content: `${content.substring(0, trigger.atIndex)}${mention} ${suffix}`,
    cursorPosition: trigger.atIndex + mention.length + 1,
  }
}

function resolveMentionTrigger(content, cursorPosition) {
  const textBeforeCursor = String(content).substring(0, cursorPosition)
  const atIndex = findLastMentionIndex(textBeforeCursor)
  if (atIndex === -1) {
    return null
  }

  const charBeforeAt = atIndex > 0 ? textBeforeCursor[atIndex - 1] : ' '
  if (!(charBeforeAt === ' ' || charBeforeAt === '\n' || atIndex === 0)) {
    return null
  }

  const query = textBeforeCursor.substring(atIndex + 1)
  if (query.includes(' ') || query.includes('\n')) {
    return null
  }

  return {
    atIndex,
    query,
  }
}

function findLastMentionIndex(content) {
  return Math.max(content.lastIndexOf('@'), content.lastIndexOf('＠'))
}

function normalizeKey(value) {
  return String(value || '').trim().toLowerCase()
}
