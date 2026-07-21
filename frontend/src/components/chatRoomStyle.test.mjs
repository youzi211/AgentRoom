import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const styles = readFileSync(new URL('../chat-room.css', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./ChatRoom.jsx', import.meta.url), 'utf8')
const agentRosterSource = readFileSync(new URL('./AgentRoster.jsx', import.meta.url), 'utf8')

test('room workbench uses entry dashboard visual language', () => {
  assert.match(styles, /\.chat-workbench[\s\S]*radial-gradient\(circle at 20% 24%/)
  assert.match(styles, /\.chat-topbar[\s\S]*rgba\(255, 255, 255, 0\.92\)/)
  assert.match(styles, /\.conversation-heading[\s\S]*box-shadow: 0 18px 44px/)
})

test('agent mention buttons align to a consistent column', () => {
  assert.match(styles, /\.agent-row[\s\S]*grid-template-columns: minmax\(0, 1fr\) 96px/)
  assert.match(styles, /\.mention-button[\s\S]*width: 96px/)
  assert.match(styles, /\.mention-button[\s\S]*text-overflow: ellipsis/)
})

test('room workbench prefers Mantine primitives for visible controls', () => {
  assert.match(chatRoomSource, /@mantine\/core/)
  assert.match(chatRoomSource, /<Paper component="header"/)
  assert.match(chatRoomSource, /<Button variant="default"/)
  assert.match(chatRoomSource, /<Badge variant="light"/)
  assert.match(agentRosterSource, /@mantine\/core/)
  assert.match(agentRosterSource, /<Paper component="li"/)
  assert.match(agentRosterSource, /<Button className="mention-button"/)
})
