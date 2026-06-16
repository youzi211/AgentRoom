export const ROOM_SURFACES = {
  loading: 'loading',
  entry: 'entry',
  live: 'live',
  readOnly: 'readOnly',
  denied: 'denied',
}

export function resolveRoomSurface({ roomStatus = '', participantName = '', hasRoomData = true }) {
  if (!hasRoomData) {
    return ROOM_SURFACES.loading
  }
  if (roomStatus === 'archived') {
    return ROOM_SURFACES.denied
  }
  if (roomStatus === 'closed') {
    return ROOM_SURFACES.readOnly
  }
  return participantName ? ROOM_SURFACES.live : ROOM_SURFACES.entry
}

export function nextRouteAfterLiveTermination({ status = '', roomId = '', passcode = '' }) {
  const encodedRoomId = encodeURIComponent(roomId)
  const search = passcode ? `?passcode=${encodeURIComponent(passcode)}` : ''

  if (status === 'closed') {
    return {
      route: 'room',
      path: `/rooms/${encodedRoomId}${search}`,
      clearSession: true,
    }
  }

  return {
    route: 'home',
    path: '/',
    clearSession: true,
  }
}
