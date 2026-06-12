const JSON_HEADERS = {
  'Content-Type': 'application/json',
}

const API_BASE_PATH = '/api'

async function parseResponse(response) {
  let payload = null

  const contentType = response.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    payload = await response.json()
  }

  if (!response.ok) {
    const message = payload?.error ?? '请求失败，请检查网络或稍后重试。'
    throw new Error(message)
  }

  return payload
}

export async function createRoom(name, agentIds) {
  const response = await fetch(`${API_BASE_PATH}/rooms`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify({ name, agentIds }),
  })

  return parseResponse(response)
}

export async function getRoom(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}`)
  return parseResponse(response)
}

export async function getMessages(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/messages`)
  return parseResponse(response)
}

export async function getAgents() {
  const response = await fetch(`${API_BASE_PATH}/agents`)
  return parseResponse(response)
}

export async function updateAgent(agentId, agent) {
  const encodedAgentId = encodeURIComponent(agentId)
  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}`, {
    method: 'PUT',
    headers: JSON_HEADERS,
    body: JSON.stringify(agent),
  })

  return parseResponse(response)
}

export async function createAgent(agent) {
  const response = await fetch(`${API_BASE_PATH}/agents`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify(agent),
  })

  return parseResponse(response)
}

export async function deleteAgent(agentId) {
  const encodedAgentId = encodeURIComponent(agentId)
  const response = await fetch(`${API_BASE_PATH}/agents/${encodedAgentId}`, {
    method: 'DELETE',
  })

  return parseResponse(response)
}

export async function getRoomKnowledge(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/knowledge`)
  return parseResponse(response)
}

export async function uploadRoomKnowledge(roomId, file) {
  const encodedRoomId = encodeURIComponent(roomId)
  const formData = new FormData()
  formData.append('file', file)

  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/knowledge`, {
    method: 'POST',
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
    body: formData,
  })

  return parseResponse(response)
}

export async function deleteKnowledgeDocument(documentId) {
  const encodedDocumentId = encodeURIComponent(documentId)
  const response = await fetch(`${API_BASE_PATH}/knowledge/${encodedDocumentId}`, {
    method: 'DELETE',
  })

  return parseResponse(response)
}

export function createRoomSocket(roomId, participantName) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const encodedRoomId = encodeURIComponent(roomId)
  const url = new URL(`${API_BASE_PATH}/rooms/${encodedRoomId}/ws`, `${protocol}//${window.location.host}`)
  url.searchParams.set('name', participantName)
  return new WebSocket(url)
}
