export function buildParticipantJoinedNotice({ participant, currentParticipantID, now } = {}) {
  if (!participant?.id || participant.id === currentParticipantID) {
    return null
  }

  const createdAt = participant.joinedAt || now || new Date().toISOString()
  const participantName = participant.name?.trim() || '\u65b0\u6210\u5458'

  return {
    id: `notice:participant_joined:${participant.id}:${createdAt}`,
    senderID: 'system',
    senderName: '\u7cfb\u7edf',
    senderType: 'system',
    content: `${participantName} \u52a0\u5165\u4e86\u4f1a\u8bae`,
    createdAt,
  }
}

export function mergeTimelineMessages(messages = [], liveNotices = []) {
  return [...messages, ...liveNotices]
    .map((item, index) => ({
      item,
      index,
      timestamp: Date.parse(item.createdAt || ''),
    }))
    .sort((left, right) => {
      const leftTimestamp = Number.isNaN(left.timestamp) ? Number.MAX_SAFE_INTEGER : left.timestamp
      const rightTimestamp = Number.isNaN(right.timestamp) ? Number.MAX_SAFE_INTEGER : right.timestamp

      if (leftTimestamp !== rightTimestamp) {
        return leftTimestamp - rightTimestamp
      }

      return left.index - right.index
    })
    .map(({ item }) => item)
}
