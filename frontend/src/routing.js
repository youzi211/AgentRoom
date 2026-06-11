const NAVIGATION_EVENT = 'agentroom:navigation'
const ROOM_SESSION_PREFIX = 'agentroom:room-session:'

export const ROUTE_NAMES = {
  home: 'home',
  agents: 'agents',
  room: 'room',
  notFound: 'notFound',
}

export function parseCurrentRoute() {
  const normalizedPath = normalizePath(window.location.pathname)
  const segments = normalizedPath.split('/').filter(Boolean)

  if (segments.length === 0) {
    return { name: ROUTE_NAMES.home }
  }

  if (segments.length === 1 && segments[0] === 'agents') {
    return { name: ROUTE_NAMES.agents }
  }

  if (segments.length === 2 && segments[0] === 'rooms') {
    try {
      const roomId = decodeURIComponent(segments[1])
      const participantName = new URLSearchParams(window.location.search).get('name')?.trim() || ''
      return { name: ROUTE_NAMES.room, roomId, participantName }
    } catch {
      return { name: ROUTE_NAMES.notFound }
    }
  }

  return { name: ROUTE_NAMES.notFound }
}

export function navigateHome({ replace = false } = {}) {
  navigate('/', { replace })
}

export function navigateAgents({ replace = false } = {}) {
  navigate('/agents', { replace })
}

export function navigateRoom(roomId, roomSession, { replace = false } = {}) {
  const nextSession = {
    roomId,
    participantName: roomSession.participantName,
    initialRoom: roomSession.initialRoom,
  }
  persistRoomSession(roomId, nextSession)
  navigate(`/rooms/${encodeURIComponent(roomId)}`, {
    replace,
    state: { agentRoom: nextSession },
  })
}

export function subscribeToNavigation(listener) {
  window.addEventListener('popstate', listener)
  window.addEventListener(NAVIGATION_EVENT, listener)

  return () => {
    window.removeEventListener('popstate', listener)
    window.removeEventListener(NAVIGATION_EVENT, listener)
  }
}

export function resolveRoomSession(roomId, routeParticipantName = '') {
  const stateSession = window.history.state?.agentRoom
  if (stateSession?.roomId === roomId && stateSession?.participantName) {
    return {
      participantName: stateSession.participantName,
      initialRoom: stateSession.initialRoom,
    }
  }

  const storedSession = readStoredRoomSession(roomId)
  if (storedSession?.participantName) {
    return storedSession
  }

  if (routeParticipantName) {
    return {
      participantName: routeParticipantName,
      initialRoom: null,
    }
  }

  return null
}

function navigate(path, { replace = false, state = null } = {}) {
  if (replace) {
    window.history.replaceState(state, '', path)
  } else {
    window.history.pushState(state, '', path)
  }

  window.scrollTo(0, 0)
  window.dispatchEvent(new Event(NAVIGATION_EVENT))
}

function normalizePath(pathname) {
  if (!pathname || pathname === '/') {
    return '/'
  }

  return pathname.replace(/\/+$/, '')
}

function persistRoomSession(roomId, roomSession) {
  if (!roomSession?.participantName) {
    return
  }

  try {
    window.sessionStorage.setItem(
      roomSessionKey(roomId),
      JSON.stringify({
        participantName: roomSession.participantName,
        roomId,
        initialRoom: roomSession.initialRoom,
      }),
    )
  } catch {
    // Session storage can be unavailable in hardened browser contexts.
  }
}

function readStoredRoomSession(roomId) {
  try {
    const storedValue = window.sessionStorage.getItem(roomSessionKey(roomId))
    return storedValue ? JSON.parse(storedValue) : null
  } catch {
    return null
  }
}

function roomSessionKey(roomId) {
  return `${ROOM_SESSION_PREFIX}${roomId}`
}
