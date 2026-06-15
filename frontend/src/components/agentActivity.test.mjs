import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  labelForActivityStatus,
  mergeActivityEvent,
  normalizeActivityPayload,
  sortActivityItems,
} from './agentActivity.js'

test('labelForActivityStatus maps known run states to Chinese labels', () => {
  assert.equal(labelForActivityStatus('running'), '运行中')
  assert.equal(labelForActivityStatus('succeeded'), '已完成')
  assert.equal(labelForActivityStatus('failed'), '失败')
  assert.equal(labelForActivityStatus('timeout'), '超时')
  assert.equal(labelForActivityStatus('stopped_limit'), '达到轮次上限')
  assert.equal(labelForActivityStatus('stopped_duplicate'), '重复内容停止')
  assert.equal(labelForActivityStatus('stopped_empty'), '空回复停止')
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
