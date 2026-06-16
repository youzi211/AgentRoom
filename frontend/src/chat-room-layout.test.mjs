import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const css = readFileSync(new URL('./chat-room.css', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./components/ChatRoom.jsx', import.meta.url), 'utf8')
const roomGatewaySource = readFileSync(new URL('./components/RoomGateway.jsx', import.meta.url), 'utf8')
const roomReadOnlySource = readFileSync(new URL('./components/RoomReadOnly.jsx', import.meta.url), 'utf8')

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

test('agent activity timeline scrolls inside its own bounded panel', () => {
  const workbenchPanel = ruleBlock('.agent-workbench-panel')
  const rosterRegion = ruleBlock('.agent-roster-region')
  const activityPanel = ruleBlock('.agent-activity-panel')
  const activityList = ruleBlock('.agent-activity-list')
  const focusPanel = ruleBlock('.agent-workbench-panel .focus-panel')

  assert.match(workbenchPanel, /display:\s*grid;/)
  assert.match(workbenchPanel, /grid-template-rows:\s*minmax\(180px,\s*0\.9fr\)\s+minmax\(180px,\s*1fr\)\s+auto;/)
  assert.match(rosterRegion, /min-height:\s*0;/)
  assert.match(rosterRegion, /overflow-y:\s*auto;/)
  assert.match(activityPanel, /max-height:\s*min\(22vh,\s*180px\);/)
  assert.match(activityPanel, /overflow:\s*hidden;/)
  assert.match(activityList, /overflow-y:\s*auto;/)
  assert.match(activityList, /min-height:\s*0;/)
  assert.match(focusPanel, /min-height:\s*0;/)
})

test('meeting focus owns the primary right-panel space before activity logs', () => {
  const focusIndex = chatRoomSource.indexOf('<FocusTimeline focusPoints={focusPoints} />')
  const activityIndex = chatRoomSource.indexOf('<AgentActivityPanel activities={activityItems}')

  assert.ok(focusIndex > -1)
  assert.ok(activityIndex > -1)
  assert.ok(focusIndex < activityIndex)
  assert.doesNotMatch(chatRoomSource, /rightTopHeight/)
})

test('closed-room read-only screen avoids live socket and composer imports', () => {
  assert.match(roomGatewaySource, /import RoomReadOnly from '\.\/RoomReadOnly'/)
  assert.match(roomGatewaySource, /ROOM_SURFACES\.readOnly/)
  assert.doesNotMatch(roomReadOnlySource, /createRoomSocket/)
  assert.doesNotMatch(roomReadOnlySource, /MessageComposer/)
})

test('chat room includes owner controls and lifecycle exit handling', () => {
  assert.match(chatRoomSource, /const ROOM_CLOSED_EVENT = 'room_closed'/)
  assert.match(chatRoomSource, /const ROOM_ARCHIVED_EVENT = 'room_archived'/)
  assert.match(chatRoomSource, /type:\s*'close_room'/)
  assert.match(chatRoomSource, /type:\s*'transfer_owner'/)
  assert.match(chatRoomSource, /clearRoomSession/)
  assert.match(chatRoomSource, /nextRouteAfterLiveTermination/)
  assert.match(chatRoomSource, /meeting-owner-panel/)
})
