import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const css = readFileSync(new URL('./chat-room.css', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./components/ChatRoom.jsx', import.meta.url), 'utf8')

function ruleBlock(selector) {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = css.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, 'm'))
  return match?.[1] ?? ''
}

test('left meeting context panel remains vertically scrollable', () => {
  const block = ruleBlock('.chat-context-panel')

  assert.match(block, /overflow-y:\s*auto;/)
  assert.doesNotMatch(block, /overflow:\s*hidden;/)
})

test('chat room surfaces the current dialogue mode in visible room chrome', () => {
  assert.match(chatRoomSource, /Agent 模式：\$\{labelForDialogueMode\(room\.dialoguePolicy\?\.mode\)\}/)
  assert.match(chatRoomSource, /<span>Agent 模式<\/span>/)
  assert.match(chatRoomSource, /'引导多轮'/)
  assert.match(chatRoomSource, /'点名单轮'/)
})

test('chat room renders the agent activity panel', () => {
  assert.match(chatRoomSource, /import AgentActivityPanel from '\.\/AgentActivityPanel'/)
  assert.match(chatRoomSource, /const AGENT_ACTIVITY_EVENT = 'agent_activity'/)
  assert.match(chatRoomSource, /<AgentActivityPanel activities=\{activityItems\}/)
})
