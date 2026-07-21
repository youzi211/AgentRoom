import assert from 'node:assert/strict'
import test from 'node:test'
import { ADMIN_SECTIONS, ROUTE_NAMES, navigateAdmin, parseCurrentRoute } from './routing.js'

function withWindow(pathname, callback) {
  const originalWindow = globalThis.window
  const navigations = []
  globalThis.window = {
    location: { pathname, search: '' },
    history: {
      pushState(state, _title, path) { navigations.push({ method: 'push', state, path }) },
      replaceState(state, _title, path) { navigations.push({ method: 'replace', state, path }) },
    },
    scrollTo() {},
    dispatchEvent() {},
  }
  try {
    return callback(navigations)
  } finally {
    globalThis.window = originalWindow
  }
}

test('standalone admin routes select their expected sections', () => {
  withWindow('/agents', () => {
    assert.deepEqual(parseCurrentRoute(), { name: ROUTE_NAMES.admin, section: ADMIN_SECTIONS.agents })
  })
  withWindow('/models/', () => {
    assert.deepEqual(parseCurrentRoute(), { name: ROUTE_NAMES.admin, section: ADMIN_SECTIONS.models })
  })
  withWindow('/admin/models', () => {
    assert.deepEqual(parseCurrentRoute(), { name: ROUTE_NAMES.admin, section: ADMIN_SECTIONS.models })
  })
})

test('admin navigation emits canonical agent and model paths', () => {
  withWindow('/', (navigations) => {
    navigateAdmin(ADMIN_SECTIONS.models)
    navigateAdmin(ADMIN_SECTIONS.agents)
    navigateAdmin(ADMIN_SECTIONS.meetings, { replace: true })
    assert.deepEqual(navigations, [
      { method: 'push', state: null, path: '/models' },
      { method: 'push', state: null, path: '/agents' },
      { method: 'replace', state: null, path: '/admin' },
    ])
  })
})
