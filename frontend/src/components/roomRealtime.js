import { normalizeActivityPayload } from './agentActivity.js'

export function mergeChatMessageEvent(current = [], event = null) {
  if (event?.type !== 'message' || !event.message?.id) {
    return current
  }

  const existingIndex = current.findIndex((message) => message.id === event.message.id)
  if (existingIndex === -1) {
    return [...current, event.message]
  }

  const next = [...current]
  next[existingIndex] = event.message
  return next
}

export function messagesFromHistoryPayload(payload = {}) {
  return Array.isArray(payload.messages) ? payload.messages : []
}

export function activitiesFromAuditPayload(payload = {}) {
  return normalizeActivityPayload(payload)
}
