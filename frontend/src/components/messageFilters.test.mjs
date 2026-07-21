import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import { filterMessagesByKind } from './messageFilters.js'

const messages = [
  { id: 'human-1', senderType: 'human', content: 'hi' },
  { id: 'agent-1', senderType: 'agent', content: 'hello' },
  { id: 'system-1', senderType: 'system', content: 'joined' },
]

test('filterMessagesByKind returns all timeline items for the all filter', () => {
  assert.deepEqual(filterMessagesByKind(messages, 'all').map((message) => message.id), ['human-1', 'agent-1', 'system-1'])
})

test('filterMessagesByKind keeps only human messages for the human filter', () => {
  assert.deepEqual(filterMessagesByKind(messages, 'human').map((message) => message.id), ['human-1'])
})

test('filterMessagesByKind keeps only agent messages for the agent filter', () => {
  assert.deepEqual(filterMessagesByKind(messages, 'agent').map((message) => message.id), ['agent-1'])
})
