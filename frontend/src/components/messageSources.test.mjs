import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('./MessageList.jsx', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../chat-room.css', import.meta.url), 'utf8')

test('MessageList renders knowledge sources for agent messages', () => {
  assert.match(source, /knowledgeSources/)
  assert.match(source, /formatKnowledgeSources/)
  assert.match(source, /message-sources/)
  assert.match(source, /参考：/)
})

test('knowledge source display has compact styles', () => {
  assert.match(styles, /\.message-sources/)
  assert.match(styles, /\.message-source-chip/)
})
