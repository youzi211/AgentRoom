import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('./MessageList.jsx', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../chat-room.css', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./ChatRoom.jsx', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api/roomClient.js', import.meta.url), 'utf8')

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

test('MessageList renders downloadable report artifacts', () => {
  assert.match(source, /message\.artifacts/)
  assert.match(source, /message-artifacts/)
  assert.match(source, /onDownloadArtifact/)
  assert.match(source, /下载报告/)
})

test('ChatRoom downloads message artifacts through the room API', () => {
  assert.match(apiSource, /export async function downloadMessageArtifact/)
  assert.match(apiSource, /\/rooms\/\$\{encodedRoomId\}\/messages\/\$\{encodedMessageId\}\/artifacts\/\$\{encodedArtifactId\}/)
  assert.match(chatRoomSource, /downloadMessageArtifact/)
  assert.match(chatRoomSource, /onDownloadArtifact=\{handleDownloadArtifact\}/)
})
