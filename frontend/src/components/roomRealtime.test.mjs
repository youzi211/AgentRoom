import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  activitiesFromAuditPayload,
  mergeChatMessageEvent,
  messagesFromHistoryPayload,
} from './roomRealtime.js'

test('framework and unknown events never enter the chat message list', () => {
  const current = [{ id: 'message_1', content: '已提交消息' }]
  const collaborationEvent = {
    type: 'collaboration_activity',
    collaboration: {
      kind: 'speaker_selected',
      collaborationRunID: 'collab_1',
      sequence: 2,
      agentID: 'pm',
    },
  }

  assert.equal(mergeChatMessageEvent(current, collaborationEvent), current)
  assert.equal(mergeChatMessageEvent(current, { type: 'framework_internal', message: { id: 'hidden' } }), current)
})

test('committed message events merge by message id', () => {
  const current = [{ id: 'message_1', content: '旧内容' }]
  const updated = mergeChatMessageEvent(current, {
    type: 'message',
    message: { id: 'message_1', content: '已提交内容' },
  })
  const appended = mergeChatMessageEvent(updated, {
    type: 'message',
    message: { id: 'message_2', content: '下一条消息' },
  })

  assert.deepEqual(appended, [
    { id: 'message_1', content: '已提交内容' },
    { id: 'message_2', content: '下一条消息' },
  ])
})

test('REST refresh restores messages and collaboration audit into separate state', () => {
  const messages = messagesFromHistoryPayload({
    messages: [{ id: 'message_1', content: '历史消息' }],
    collaborationRuns: [{ id: 'must_not_be_a_message' }],
  })
  const activities = activitiesFromAuditPayload({
    messages: [{ id: 'must_not_be_activity' }],
    collaborationRuns: [{
      id: 'collab_1',
      engine: 'native',
      status: 'succeeded',
      turnCount: 1,
      createdAt: '2026-06-15T10:00:00Z',
      completedAt: '2026-06-15T10:00:05Z',
    }],
  })

  assert.deepEqual(messages, [{ id: 'message_1', content: '历史消息' }])
  assert.equal(activities.length, 1)
  assert.equal(activities[0].kind, 'collaboration_run')
  assert.equal(activities[0].id, 'collab_1')
})
