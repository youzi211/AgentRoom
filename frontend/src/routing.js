const NAVIGATION_EVENT = 'agentroom:navigation'
const ROOM_SESSION_PREFIX = 'agentroom:room-session:'

export const ROUTE_NAMES = {
  home: 'home',
  agents: 'agents',
  models: 'models',
  admin: 'admin',
  room: 'room',
  notFound: 'notFound',
}

export const ADMIN_SECTIONS = {
  meetings: 'meetings',
  agents: 'agents',
  models: 'models',
}

export function parseCurrentRoute() {
  const normalizedPath = normalizePath(window.location.pathname)
  const segments = normalizedPath.split('/').filter(Boolean)

  if (segments.length === 0) {
    return { name: ROUTE_NAMES.home }
  }

  if (segments.length === 1 && segments[0] === 'agents') {
    return { name: ROUTE_NAMES.admin, section: ADMIN_SECTIONS.agents }
  }

  if (segments.length === 1 && segments[0] === 'models') {
    return { name: ROUTE_NAMES.admin, section: ADMIN_SECTIONS.models }
  }

  if (segments.length >= 1 && segments[0] === 'admin') {
    const section = segments[1] === 'agents' ? ADMIN_SECTIONS.agents : segments[1] === 'models' ? ADMIN_SECTIONS.models : ADMIN_SECTIONS.meetings
    return { name: ROUTE_NAMES.admin, section }
  }

  if (segments.length === 2 && segments[0] === 'rooms') {
    try {
      const roomId = decodeURIComponent(segments[1])
      const participantName = new URLSearchParams(window.location.search).get('name')?.trim() || ''
      const passcode = new URLSearchParams(window.location.search).get('passcode')?.trim() || ''
      return { name: ROUTE_NAMES.room, roomId, participantName, passcode }
    } catch {
      return { name: ROUTE_NAMES.notFound }
    }
  }

  return { name: ROUTE_NAMES.notFound }
}

export function navigateHome({ replace = false } = {}) {
  navigate('/', { replace })
}

export function navigateAdmin(section = ADMIN_SECTIONS.meetings, { replace = false } = {}) {
	const path = section === ADMIN_SECTIONS.agents ? '/agents' : section === ADMIN_SECTIONS.models ? '/models' : '/admin'
  navigate(path, { replace })
}

export function navigateRoom(roomId, roomSession, { replace = false } = {}) {
  const nextSession = {
    roomId,
    participantName: roomSession?.participantName || '',
    initialRoom: roomSession?.initialRoom || null,
    passcode: roomSession?.passcode || '',
  }
  persistRoomSession(roomId, nextSession)
  const searchParams = new URLSearchParams()
  if (nextSession.passcode) {
    searchParams.set('passcode', nextSession.passcode)
  }
  navigate(`/rooms/${encodeURIComponent(roomId)}${searchParams.toString() ? `?${searchParams.toString()}` : ''}`, {
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

export function resolveRoomSession(roomId, routeParticipantName = '', routePasscode = '') {
  const stateSession = window.history.state?.agentRoom
  if (stateSession?.roomId === roomId && stateSession?.participantName) {
    return {
      participantName: stateSession.participantName,
      initialRoom: stateSession.initialRoom,
      passcode: stateSession.passcode || routePasscode,
    }
  }

  const storedSession = readStoredRoomSession(roomId)
  if (storedSession?.participantName) {
    return {
      ...storedSession,
      passcode: storedSession.passcode || routePasscode,
    }
  }

  if (routeParticipantName || routePasscode) {
    return {
      participantName: routeParticipantName,
      initialRoom: null,
      passcode: routePasscode,
    }
  }

  return null
}

export function clearRoomSession(roomId) {
  try {
    window.sessionStorage.removeItem(roomSessionKey(roomId))
  } catch {
    // Session storage can be unavailable in hardened browser contexts.
  }
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
        passcode: roomSession.passcode || '',
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
