import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import { buildCreateRoomPayload, getRoomActivity } from './roomClient.js'

test('buildCreateRoomPayload omits dialogue policy for default fanout rooms', () => {
  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm'], 'secret', 'mention_fanout'), {
    name: 'Planning',
    agentIds: ['pm'],
    passcode: 'secret',
  })
})

test('buildCreateRoomPayload includes guided dialogue policy when requested', () => {
  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm', 'qa'], '', 'guided_dialogue'), {
    name: 'Planning',
    agentIds: ['pm', 'qa'],
    passcode: '',
    dialoguePolicy: {
      mode: 'guided_dialogue',
    },
  })
})

test('getRoomActivity requests room activity with passcode header', async () => {
  const originalFetch = globalThis.fetch
  let captured = null
  globalThis.fetch = async (url, options = {}) => {
    captured = { url, options }
    return new Response(JSON.stringify({ agentRuns: [], dialogueRuns: [] }), {
      status: 200,
      headers: { 'content-type': 'application/json' },
    })
  }

  try {
    const payload = await getRoomActivity('room 1', 'secret')

    assert.deepEqual(payload, { agentRuns: [], dialogueRuns: [] })
    assert.equal(captured.url, '/api/rooms/room%201/activity')
    assert.equal(captured.options.headers['X-Room-Passcode'], 'secret')
  } finally {
    globalThis.fetch = originalFetch
  }
})
