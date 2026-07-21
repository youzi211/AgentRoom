export function filterMessagesByKind(messages = [], filter = 'all') {
  if (filter === 'human') {
    return messages.filter((message) => message.senderType === 'human')
  }
  if (filter === 'agent') {
    return messages.filter((message) => message.senderType === 'agent')
  }
  return messages
}
