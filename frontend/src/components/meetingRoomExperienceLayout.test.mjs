import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const styles = readFileSync(new URL('../chat-room.css', import.meta.url), 'utf8')
const roomSource = readFileSync(new URL('./MeetingRoomExperience.jsx', import.meta.url), 'utf8')
const activityPanelSource = readFileSync(new URL('./AgentActivityPanel.jsx', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./ChatRoom.jsx', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api/roomClient.js', import.meta.url), 'utf8')

test('meeting room topbar keeps only in-room actions', () => {
  assert.doesNotMatch(roomSource, /管理后台/)
  assert.doesNotMatch(roomSource, /navigateAdmin/)
  assert.doesNotMatch(roomSource, /ADMIN_SECTIONS/)
})

test('meeting room v2 does not render fake clickable labels with CSS content', () => {
  assert.doesNotMatch(styles, /\.meeting-room-v2__files \.knowledge-panel \.sidebar-header::after[\s\S]*content: '知识文件/)
  assert.doesNotMatch(styles, /\.meeting-room-v2 \.composer::before[\s\S]*content: '@ 提及 Agent/)
  assert.doesNotMatch(styles, /\.meeting-room-v2__files \.knowledge-actions \.mantine-Button-label::after/)
  assert.doesNotMatch(styles, /\.meeting-room-v2__minutes\s*\{[^}]*display:\s*none/)
  assert.match(styles, /\.meeting-room-v2__files \.knowledge-panel \.sidebar-header::after[\s\S]*content: none/)
  assert.match(styles, /\.meeting-room-v2 \.composer::before[\s\S]*content: none/)
})

test('meeting room message filters are real interactive controls', () => {
  assert.match(roomSource, /useState\('all'\)/)
  assert.match(roomSource, /filterMessagesByKind\(visibleMessages,\s*messageFilter\)/)
  assert.match(roomSource, /setMessageFilter\('human'\)/)
  assert.match(roomSource, /setMessageFilter\('agent'\)/)
  assert.match(roomSource, /messages=\{displayedMessages\}/)
})

test('meeting room passes stable knowledge callbacks to avoid reloading on timer ticks', () => {
  assert.match(roomSource, /useCallback/)
  assert.match(roomSource, /const listRoomDocuments = useCallback/)
  assert.match(roomSource, /listDocuments=\{listRoomDocuments\}/)
})

test('agent activity panel separates current activity from full history', () => {
  assert.match(activityPanelSource, /currentActivities/)
  assert.match(activityPanelSource, /activity\.status === 'running'/)
  assert.match(activityPanelSource, /查看历史/)
  assert.match(activityPanelSource, /<Modal/)
  assert.match(apiSource, /getRoomActivity\(roomId,\s*passcode = '',\s*\{ limit \}/)
  assert.match(chatRoomSource, /getRoomActivity\(roomId,\s*roomPasscode,\s*\{ limit: 100 \}\)/)
})
