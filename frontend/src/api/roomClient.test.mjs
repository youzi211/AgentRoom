import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  buildCreateRoomPayload,
  clearModelProfileAPIKey,
  clearStoredAdminKey,
  createModelProfile,
  deleteModelProfile,
  disableModelProfile,
  getAgentRoleSets,
  exportRoomMinutesMarkdown,
  getAgentTemplates,
  getMessages,
  getRoom,
  getRoomActivity,
  listModelProfiles,
  reopenRoom,
  setDefaultModelProfile,
  setStoredAdminKey,
  testDraftModelProfile,
  testSavedModelProfile,
  updateModelProfile,
  uploadRoomKnowledge,
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

    globalThis.fetch = async (url, options = {}) => {
      captured = { url, options }
      return new Response(JSON.stringify({
        templates: [
          { id: 'product_manager', name: '产品经理', role: 'Product Manager', description: 'Clarifies scope.', systemPrompt: 'Prompt.' },
        ],
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    }

    const templates = await getAgentTemplates()

    assert.equal(captured.url, '/api/agent-templates')
    assert.deepEqual(templates, {
      templates: [
        { id: 'product_manager', name: '产品经理', role: 'Product Manager', description: 'Clarifies scope.', systemPrompt: 'Prompt.' },
      ],
    })

    globalThis.fetch = async (url, options = {}) => {
      captured = { url, options }
      return new Response(JSON.stringify({
        roleSets: [
          { id: 'product_review', name: '产品评审', description: 'Review product scope.', templateIDs: ['product_manager'] },
        ],
      }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    }

    const roleSets = await getAgentRoleSets()

    assert.equal(captured.url, '/api/agent-role-sets')
    assert.deepEqual(roleSets, {
      roleSets: [
        { id: 'product_review', name: '产品评审', description: 'Review product scope.', templateIDs: ['product_manager'] },
      ],
    })
  } finally {
    clearStoredAdminKey()
    globalThis.fetch = originalFetch
    globalThis.window = originalWindow
  }
})

test('model profile client follows the admin API contract', async () => {
  const originalWindow = globalThis.window
  const originalFetch = globalThis.fetch
  const storage = new Map()
  const requests = []

  globalThis.window = {
    localStorage: {
      getItem(key) { return storage.get(key) ?? null },
      setItem(key, value) { storage.set(key, String(value)) },
      removeItem(key) { storage.delete(key) },
    },
  }
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url: String(url), options })
    return new Response(JSON.stringify({ ok: true, profiles: [] }), {
      status: 200,
      headers: { 'content-type': 'application/json' },
    })
  }

  try {
    setStoredAdminKey('model-admin')
    await listModelProfiles()
    await createModelProfile({ name: 'Go', apiKey: 'create-secret' })
    await updateModelProfile('profile /1', { name: 'Updated' })
    await setDefaultModelProfile('profile /1')
    await disableModelProfile('profile /1')
    await clearModelProfileAPIKey('profile /1')
    await deleteModelProfile('profile /1')
    await testSavedModelProfile('profile /1')
    await testDraftModelProfile({ baseURL: 'https://model.test/v1', modelName: 'm', apiKey: 'draft-secret' })

    assert.deepEqual(requests.map(({ url, options }) => [url, options.method || 'GET']), [
      ['/api/model-profiles', 'GET'],
      ['/api/model-profiles', 'POST'],
      ['/api/model-profiles/profile%20%2F1', 'PUT'],
      ['/api/model-profiles/profile%20%2F1/default', 'POST'],
      ['/api/model-profiles/profile%20%2F1', 'PUT'],
      ['/api/model-profiles/profile%20%2F1', 'PUT'],
      ['/api/model-profiles/profile%20%2F1', 'DELETE'],
      ['/api/model-profiles/profile%20%2F1/test', 'POST'],
      ['/api/model-profiles/test', 'POST'],
    ])
    requests.forEach(({ options }) => assert.equal(options.headers['X-Admin-Key'], 'model-admin'))
    assert.deepEqual(JSON.parse(requests[4].options.body), { enabled: false })
    assert.deepEqual(JSON.parse(requests[5].options.body), { clearAPIKey: true })
    assert.deepEqual(JSON.parse(requests[8].options.body), {
      baseURL: 'https://model.test/v1',
      modelName: 'm',
      apiKey: 'draft-secret',
    })
  } finally {
    clearStoredAdminKey()
    globalThis.fetch = originalFetch
    globalThis.window = originalWindow
  }
})

test('room knowledge upload sends the admin key and lets the browser set multipart content type', async () => {
  const originalWindow = globalThis.window
  const originalFetch = globalThis.fetch
  const storage = new Map()
  let captured = null

  globalThis.window = {
    localStorage: {
      getItem(key) { return storage.get(key) ?? null },
      setItem(key, value) { storage.set(key, String(value)) },
      removeItem(key) { storage.delete(key) },
    },
  }
  globalThis.fetch = async (url, options = {}) => {
    captured = { url: String(url), options }
    return new Response(JSON.stringify({ document: { id: 'doc-1' } }), {
      status: 201,
      headers: { 'content-type': 'application/json' },
    })
  }

  try {
    setStoredAdminKey('room-admin')
    const file = new File(['# Notes'], 'notes.md', { type: 'text/markdown' })
    const response = await uploadRoomKnowledge('room /1', file)

    assert.deepEqual(response, { document: { id: 'doc-1' } })
    assert.equal(captured.url, '/api/rooms/room%20%2F1/knowledge')
    assert.equal(captured.options.method, 'POST')
    assert.equal(captured.options.headers['X-Admin-Key'], 'room-admin')
    assert.equal(captured.options.headers['Content-Type'], undefined)
    assert.equal(captured.options.body instanceof FormData, true)
    assert.equal(captured.options.body.get('file').name, 'notes.md')
  } finally {
    clearStoredAdminKey()
    globalThis.fetch = originalFetch
    globalThis.window = originalWindow
  }
})
