const JSON_HEADERS = {
  'Content-Type': 'application/json',
}

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
  const response = await fetch('/rooms', {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify({ name }),
  })

  return parseResponse(response)
}

export async function getRoom(roomId) {
  const response = await fetch(`/rooms/${roomId}`)
  return parseResponse(response)
}

export async function getMessages(roomId) {
  const response = await fetch(`/rooms/${roomId}/messages`)
  return parseResponse(response)
}

export function createRoomSocket(roomId, participantName) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`/rooms/${roomId}/ws`, `${protocol}//${window.location.host}`)
  url.searchParams.set('name', participantName)
  return new WebSocket(url)
}
