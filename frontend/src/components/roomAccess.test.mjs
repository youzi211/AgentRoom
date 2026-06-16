import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import { nextRouteAfterLiveTermination, resolveRoomSurface, ROOM_SURFACES } from './roomAccess.js'

test('active room without participant session resolves to entry', () => {
  assert.equal(resolveRoomSurface({ roomStatus: 'active', participantName: '', hasRoomData: true }), ROOM_SURFACES.entry)
})

test('active room with participant session resolves to live chat', () => {
  assert.equal(resolveRoomSurface({ roomStatus: 'active', participantName: 'Alice', hasRoomData: true }), ROOM_SURFACES.live)
})

test('closed room resolves to read-only history even without participant name', () => {
  assert.equal(resolveRoomSurface({ roomStatus: 'closed', participantName: '', hasRoomData: true }), ROOM_SURFACES.readOnly)
})

test('archived room resolves to denied for ordinary users', () => {
  assert.equal(resolveRoomSurface({ roomStatus: 'archived', participantName: 'Alice', hasRoomData: true }), ROOM_SURFACES.denied)
})

test('room_closed clears the live session but keeps the room link', () => {
  assert.deepEqual(nextRouteAfterLiveTermination({ status: 'closed', roomId: 'room 1', passcode: 'secret' }), {
    route: 'room',
    path: '/rooms/room%201?passcode=secret',
    clearSession: true,
  })
})

test('room_archived clears the session and exits the live room surface', () => {
  assert.deepEqual(nextRouteAfterLiveTermination({ status: 'archived', roomId: 'room_1', passcode: 'secret' }), {
    route: 'home',
    path: '/',
    clearSession: true,
  })
})
