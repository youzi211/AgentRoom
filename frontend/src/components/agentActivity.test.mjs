import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  descriptionForActivity,
  labelForActivityStatus,
  mergeActivityEvent,
  mergeCollaborationActivityEvent,
  normalizeActivityPayload,
  sortActivityItems,
} from './agentActivity.js'

test('labelForActivityStatus maps known run states to Chinese labels', () => {
  assert.equal(labelForActivityStatus('running'), '运行中')
  assert.equal(labelForActivityStatus('cancelled'), '已取消')
  assert.equal(labelForActivityStatus('interrupted'), '已中断')
  assert.equal(labelForActivityStatus('succeeded'), '已完成')
  assert.equal(labelForActivityStatus('failed'), '失败')
  assert.equal(labelForActivityStatus('timeout'), '超时')
  assert.equal(labelForActivityStatus('stopped_limit'), '达到轮次上限')
  assert.equal(labelForActivityStatus('stopped_duplicate'), '重复内容停止')
  assert.equal(labelForActivityStatus('stopped_empty'), '空回复停止')
})

test('normalizeActivityPayload includes collaboration run audit state', () => {
  const items = normalizeActivityPayload({
    collaborationRuns: [
      {
        id: 'collab_1',
        roomID: 'room_1',
        engine: 'native',
        status: 'cancelled',
        stopReason: 'cancelled',
        turnCount: 1,
        createdAt: '2026-06-15T10:00:00Z',
        completedAt: '2026-06-15T10:00:05Z',
      },
    ],
  })

  assert.equal(items.length, 1)
  assert.equal(items[0].kind, 'collaboration_run')
  assert.equal(items[0].status, 'cancelled')
  assert.equal(descriptionForActivity(items[0]), '协作已取消')
})

test('collaboration activity events merge speaker, handoff, cancellation and terminal state by run', () => {
  let items = mergeCollaborationActivityEvent([], {
    kind: 'speaker_selected',
    collaborationRunID: 'collab_1',
    sequence: 2,
    roomID: 'room_1',
    agentID: 'pm',
    agentName: '产品经理',
    occurredAt: '2026-06-15T10:00:01Z',
  })

  assert.equal(items.length, 1)
  assert.equal(items[0].status, 'running')
  assert.equal(descriptionForActivity(items[0]), '产品经理 已选为当前发言者')

  items = mergeCollaborationActivityEvent(items, {
    kind: 'handoff_requested',
    collaborationRunID: 'collab_1',
    sequence: 3,
    roomID: 'room_1',
    agentID: 'pm',
    agentName: '产品经理',
    targetAgentID: 'architect',
    targetAgentName: '架构师',
    occurredAt: '2026-06-15T10:00:02Z',
  })
  assert.equal(descriptionForActivity(items[0]), '产品经理 将协作交给 架构师')

  items = mergeCollaborationActivityEvent(items, {
    kind: 'cancelled',
    collaborationRunID: 'collab_1',
    sequence: 4,
    roomID: 'room_1',
    stopReason: 'cancelled',
    turnCount: 1,
    occurredAt: '2026-06-15T10:00:03Z',
  })
  assert.equal(items[0].status, 'cancelled')
  assert.equal(items[0].completedAt, '2026-06-15T10:00:03Z')
  assert.equal(descriptionForActivity(items[0]), '协作已取消')
})

test('late collaboration events cannot overwrite a newer terminal state', () => {
  const completed = mergeCollaborationActivityEvent([], {
    kind: 'completed',
    collaborationRunID: 'collab_1',
    sequence: 5,
    roomID: 'room_1',
    turnCount: 2,
    occurredAt: '2026-06-15T10:00:05Z',
  })

  const afterLateEvent = mergeCollaborationActivityEvent(completed, {
    kind: 'speaker_selected',
    collaborationRunID: 'collab_1',
    sequence: 4,
    roomID: 'room_1',
    agentID: 'late-agent',
    occurredAt: '2026-06-15T10:00:04Z',
  })

  assert.equal(afterLateEvent, completed)
  assert.equal(afterLateEvent[0].status, 'succeeded')
  assert.equal(afterLateEvent[0].sequence, 5)
  assert.equal(afterLateEvent[0].agentID, undefined)
})

test('normalizeActivityPayload combines agent and dialogue runs into sortable activity items', () => {
  const payload = {
    agentRuns: [
      {
        id: 'run_1',
        roomID: 'room_1',
        agentID: 'builder',
        agentName: 'Builder',
        status: 'succeeded',
        createdAt: '2026-06-15T10:00:00Z',
        completedAt: '2026-06-15T10:00:02Z',
      },
    ],
    dialogueRuns: [
      {
        id: 'dialogue_1',
        roomID: 'room_1',
        status: 'running',
        turnCount: 1,
        createdAt: '2026-06-15T10:01:00Z',
      },
    ],
  }

  const items = normalizeActivityPayload(payload)

  assert.equal(items.length, 2)
  assert.equal(items[0].kind, 'dialogue_run')
  assert.equal(items[0].id, 'dialogue_1')
  assert.equal(items[1].kind, 'agent_run')
})

test('mergeActivityEvent updates an existing activity by kind and id', () => {
  const current = [
    {
      kind: 'agent_run',
      phase: 'started',
      id: 'run_1',
      status: 'running',
      createdAt: '2026-06-15T10:00:00Z',
    },
  ]

  const next = mergeActivityEvent(current, {
    kind: 'agent_run',
    phase: 'finished',
    id: 'run_1',
    status: 'succeeded',
    completedAt: '2026-06-15T10:00:02Z',
  })

  assert.equal(next.length, 1)
  assert.equal(next[0].phase, 'finished')
  assert.equal(next[0].status, 'succeeded')
  assert.equal(next[0].completedAt, '2026-06-15T10:00:02Z')
})

test('sortActivityItems keeps running items first and then newest completed items', () => {
  const sorted = sortActivityItems([
    { kind: 'agent_run', id: 'old_done', status: 'succeeded', createdAt: '2026-06-15T10:00:00Z', completedAt: '2026-06-15T10:00:02Z' },
    { kind: 'agent_run', id: 'running', status: 'running', createdAt: '2026-06-15T09:59:00Z' },
    { kind: 'dialogue_run', id: 'new_done', status: 'stopped_limit', createdAt: '2026-06-15T10:02:00Z', completedAt: '2026-06-15T10:02:05Z' },
  ])

  assert.deepEqual(sorted.map((item) => item.id), ['running', 'new_done', 'old_done'])
})
