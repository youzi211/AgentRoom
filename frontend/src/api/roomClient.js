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
    const message = payload?.error ?? 'Request failed'
    throw new Error(message)
  }

  return payload
}

export async function createRoom(name) {
  const response = await fetch(`${API_BASE_PATH}/rooms`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify({ name }),
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

export function createRoomSocket(roomId, participantName) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const encodedRoomId = encodeURIComponent(roomId)
  const url = new URL(`${API_BASE_PATH}/rooms/${encodedRoomId}/ws`, `${protocol}//${window.location.host}`)
  url.searchParams.set('name', participantName)
  return new WebSocket(url)
}
