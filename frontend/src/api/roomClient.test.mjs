import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  buildCreateRoomPayload,
  clearStoredAdminKey,
  exportRoomMinutesMarkdown,
  getMessages,
  getRoom,
  getRoomActivity,
  reopenRoom,
  setStoredAdminKey,
} from './roomClient.js'

test('room client helpers build expected payloads and headers', async () => {
  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm'], 'secret', 'mention_fanout'), {
    name: 'Planning',
    agentIds: ['pm'],
    passcode: 'secret',
  })

  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm', 'qa'], '', 'guided_dialogue'), {
    name: 'Planning',
    agentIds: ['pm', 'qa'],
    passcode: '',
    dialoguePolicy: {
      mode: 'guided_dialogue',
    },
  })

  const originalWindow = globalThis.window
  const originalFetch = globalThis.fetch
  const storage = new Map()

  globalThis.window = {
    localStorage: {
      getItem(key) {
        return storage.get(key) ?? null
      },
      setItem(key, value) {
        storage.set(key, String(value))
      },
      removeItem(key) {
        storage.delete(key)
      },
    },
    location: { protocol: 'http:', host: 'localhost:5173' },
  }

  try {
    setStoredAdminKey('secret-admin')

    let captured = null
    globalThis.fetch = async (url, options = {}) => {
      captured = { url, options }
      return new Response(JSON.stringify({ messages: [], hasMore: false, nextBefore: '' }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    }

    const pagedMessages = await getMessages('room 1', 'secret', { before: 'msg_100', limit: 25 })

    assert.deepEqual(pagedMessages, { messages: [], hasMore: false, nextBefore: '' })
    assert.equal(captured.url, '/api/rooms/room%201/messages?before=msg_100&limit=25')
    assert.equal(captured.options.headers['X-Room-Passcode'], 'secret')
    assert.equal(captured.options.headers['X-Admin-Key'], 'secret-admin')

    const requests = []
    globalThis.fetch = async (url, options = {}) => {
      requests.push({ url: String(url), options })
      if (String(url).endsWith('/minutes.md')) {
        return new Response('# Minutes', { status: 200, headers: { 'content-type': 'text/markdown' } })
      }
      return new Response(JSON.stringify({ room: { id: 'room_1' }, participants: [], agents: [], agentRuns: [], dialogueRuns: [] }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    }

    await getRoom('room_1')
    await getRoomActivity('room_1')
    await exportRoomMinutesMarkdown('room_1')

    assert.equal(requests.length, 3)
    requests.forEach(({ options }) => {
      assert.equal(options.headers['X-Admin-Key'], 'secret-admin')
    })

    globalThis.fetch = async (url, options = {}) => {
      captured = { url, options }
      return new Response(JSON.stringify({ room: { id: 'room_1', status: 'active' } }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    }

    const reopened = await reopenRoom('room 1')

    assert.deepEqual(reopened, { room: { id: 'room_1', status: 'active' } })
    assert.equal(captured.url, '/api/rooms/room%201/reopen')
    assert.equal(captured.options.method, 'POST')
    assert.equal(captured.options.headers['X-Admin-Key'], 'secret-admin')
  } finally {
    clearStoredAdminKey()
    globalThis.fetch = originalFetch
    globalThis.window = originalWindow
  }
})
