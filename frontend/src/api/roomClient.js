const JSON_HEADERS = {
  'Content-Type': 'application/json',
}

const API_BASE_PATH = '/api'
const ADMIN_KEY_STORAGE = 'agentroom:admin-key'
const BUILD_ADMIN_API_KEY = (import.meta.env?.VITE_ADMIN_API_KEY || '').trim()

export function getStoredAdminKey() {
  try {
    return (window.localStorage.getItem(ADMIN_KEY_STORAGE) || '').trim()
  } catch {
    return ''
  }
}

export function setStoredAdminKey(key) {
  try {
    const trimmed = (key || '').trim()
    if (trimmed) {
      window.localStorage.setItem(ADMIN_KEY_STORAGE, trimmed)
    } else {
      window.localStorage.removeItem(ADMIN_KEY_STORAGE)
    }
  } catch {
    // localStorage can be unavailable in hardened browser contexts.
  }
}

export function clearStoredAdminKey() {
  setStoredAdminKey('')
}

function adminApiKey() {
  return getStoredAdminKey() || BUILD_ADMIN_API_KEY
}

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
  const key = adminApiKey()
  if (!key) {
    return headers
  }
  return {
    ...headers,
    'X-Admin-Key': key,
  }
}

function filenameFromContentDisposition(value = '') {
  const match = value.match(/filename="?([^";]+)"?/i)
  return match?.[1] || ''
}

export async function createRoom(name, agentIds, passcode = '', dialogueMode = 'mention_fanout') {
  const response = await fetch(`${API_BASE_PATH}/rooms`, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify(buildCreateRoomPayload(name, agentIds, passcode, dialogueMode)),
  })

  return parseResponse(response)
}

export function buildCreateRoomPayload(name, agentIds, passcode = '', dialogueMode = 'mention_fanout') {
  const payload = { name, agentIds, passcode }
  if (dialogueMode === 'guided_dialogue') {
    payload.dialoguePolicy = {
      mode: dialogueMode,
    }
  }
  return payload
}

export async function getRoom(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })
  return parseResponse(response)
}

export async function getMessages(roomId, passcode = '', { before = '', limit } = {}) {
  const encodedRoomId = encodeURIComponent(roomId)
  const params = new URLSearchParams()
  if (before) {
    params.set('before', before)
  }
  if (limit) {
    params.set('limit', String(limit))
  }
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/messages${params.toString() ? `?${params.toString()}` : ''}`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })
  return parseResponse(response)
}

export async function getRoomActivity(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/activity`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })
  return parseResponse(response)
}

export async function generateRoomMinutes(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes`, {
    method: 'POST',
    headers: withAdminKey(withRoomPasscode(JSON_HEADERS, passcode)),
  })

  return parseResponse(response)
}

export async function exportRoomMinutesMarkdown(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes.md`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
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

export async function downloadMessageArtifact(roomId, messageId, artifactId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const encodedMessageId = encodeURIComponent(messageId)
  const encodedArtifactId = encodeURIComponent(artifactId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/messages/${encodedMessageId}/artifacts/${encodedArtifactId}`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })

  if (!response.ok) {
    let message = '下载报告失败，请稍后重试。'
    const contentType = response.headers.get('content-type') ?? ''
    if (contentType.includes('application/json')) {
      const payload = await response.json()
      message = payload?.error ?? message
    }
    throw new Error(message)
  }

  return {
    blob: await response.blob(),
    fileName: filenameFromContentDisposition(response.headers.get('content-disposition')) || 'report.md',
  }
}

export async function getAgents() {
  const response = await fetch(`${API_BASE_PATH}/agents`)
  return parseResponse(response)
}

export async function getAgentTemplates() {
  const response = await fetch(`${API_BASE_PATH}/agent-templates`)
  return parseResponse(response)
}

export async function getAgentRoleSets() {
  const response = await fetch(`${API_BASE_PATH}/agent-role-sets`)
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
    headers: withAdminKey(withRoomPasscode({}, passcode)),
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

export async function verifyAdminKey() {
  const response = await fetch(`${API_BASE_PATH}/admin/verify`, {
    headers: withAdminKey(),
  })
  return parseResponse(response)
}

export async function listRooms({ status = '', limit, offset } = {}) {
  const params = new URLSearchParams()
  if (status) {
    params.set('status', status)
  }
  if (limit) {
    params.set('limit', String(limit))
  }
  if (offset) {
    params.set('offset', String(offset))
  }
  const query = params.toString()
  const response = await fetch(`${API_BASE_PATH}/rooms${query ? `?${query}` : ''}`, {
    headers: withAdminKey(),
  })
  return parseResponse(response)
}

export async function archiveRoom(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/archive`, {
    method: 'POST',
    headers: withAdminKey(),
  })
  return parseResponse(response)
}

export async function restoreRoom(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/restore`, {
    method: 'POST',
    headers: withAdminKey(),
  })
  return parseResponse(response)
}

export async function reopenRoom(roomId) {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/reopen`, {
    method: 'POST',
    headers: withAdminKey(),
  })
  return parseResponse(response)
}

export async function getMinutesHistory(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes/history`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })
  return parseResponse(response)
}

export async function saveRoomMinutes(roomId, content, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/minutes`, {
    method: 'PUT',
    headers: withAdminKey(withRoomPasscode(JSON_HEADERS, passcode)),
    body: JSON.stringify({ content }),
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
