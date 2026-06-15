const JSON_HEADERS = {
  'Content-Type': 'application/json',
}

const API_BASE_PATH = '/api'
const ADMIN_API_KEY = (import.meta.env.VITE_ADMIN_API_KEY || '').trim()

async function parseResponse(response) {
  let payload = null

  const contentType = response.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    payload = await response.json()
  }

  if (!response.ok) {
    const message = payload?.error ?? '请求失败，请稍后重试。'
    throw new Error(message)
  }

  return payload
}

function withRoomPasscode(headers = {}, passcode = '') {
  const trimmed = passcode?.trim()
  if (!trimmed) {
    return headers
  }
  return {
    ...headers,
    'X-Room-Passcode': trimmed,
  }
}

function withAdminKey(headers = {}) {
  if (!ADMIN_API_KEY) {
    return headers
  }
  return {
    ...headers,
    'X-Admin-Key': ADMIN_API_KEY,
  }
}

export async function createRoom(name, agentIds, passcode = '') {
  const response = await fetch(`${API_BASE_PATH}/rooms`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify({ name, agentIds, passcode }),
  })

  return parseResponse(response)
}

export async function getRoom(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}`, {
    headers: withRoomPasscode({}, passcode),
  })
  return parseResponse(response)
}

export async function getMessages(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/messages`, {
    headers: withRoomPasscode({}, passcode),
  })
  return parseResponse(response)
}

export async function generateRoomMinutes(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes`, {
    method: 'POST',
    headers: withRoomPasscode(JSON_HEADERS, passcode),
  })

  return parseResponse(response)
}

export async function exportRoomMinutesMarkdown(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes.md`, {
    headers: withRoomPasscode({}, passcode),
  })

  if (!response.ok) {
    let message = '导出会议纪要失败，请稍后重试。'
    const contentType = response.headers.get('content-type') ?? ''
    if (contentType.includes('application/json')) {
      const payload = await response.json()
      message = payload?.error ?? message
    }
    throw new Error(message)
  }

  return response.text()
}

export async function getAgents() {
  const response = await fetch(`${API_BASE_PATH}/agents`)
  return parseResponse(response)
}

export async function updateAgent(agentId, agent) {
  const encodedAgentId = encodeURIComponent(agentId)
  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}`, {
    method: 'PUT',
    headers: withAdminKey(JSON_HEADERS),
    body: JSON.stringify(agent),
  })

  return parseResponse(response)
}

export async function createAgent(agent) {
  const response = await fetch(`${API_BASE_PATH}/agents`, {
    method: 'POST',
    headers: withAdminKey(JSON_HEADERS),
    body: JSON.stringify(agent),
  })

  return parseResponse(response)
}

export async function deleteAgent(agentId) {
  const encodedAgentId = encodeURIComponent(agentId)
  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}`, {
    method: 'DELETE',
    headers: withAdminKey(),
  })

  return parseResponse(response)
}

export async function getRoomKnowledge(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/knowledge`, {
    headers: withRoomPasscode({}, passcode),
  })
  return parseResponse(response)
}

export async function uploadRoomKnowledge(roomId, file) {
  const encodedRoomId = encodeURIComponent(roomId)
  const formData = new FormData()
  formData.append('file', file)

  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/knowledge`, {
    method: 'POST',
    headers: withAdminKey(),
    body: formData,
  })

  return parseResponse(response)
}

export async function getAgentKnowledge(agentId) {
  const encodedAgentId = encodeURIComponent(agentId)
  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}/knowledge`)
  return parseResponse(response)
}

export async function uploadAgentKnowledge(agentId, file) {
  const encodedAgentId = encodeURIComponent(agentId)
  const formData = new FormData()
  formData.append('file', file)

  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}/knowledge`, {
    method: 'POST',
    headers: withAdminKey(),
    body: formData,
  })

  return parseResponse(response)
}

export async function deleteKnowledgeDocument(documentId) {
  const encodedDocumentId = encodeURIComponent(documentId)
  const response = await fetch(`${API_BASE_PATH}/knowledge/${encodedDocumentId}`, {
    method: 'DELETE',
    headers: withAdminKey(),
  })

  return parseResponse(response)
}

export function createRoomSocket(roomId, participantName, passcode = '') {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const encodedRoomId = encodeURIComponent(roomId)
  const url = new URL(`${API_BASE_PATH}/rooms/${encodedRoomId}/ws`, `${protocol}//${window.location.host}`)
  url.searchParams.set('name', participantName)
  if (passcode?.trim()) {
    url.searchParams.set('passcode', passcode.trim())
  }
  return new WebSocket(url)
}
