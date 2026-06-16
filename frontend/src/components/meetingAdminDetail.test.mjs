import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import {
  actionsForRoomStatus,
  labelForRoomStatus,
  STATUS_FILTERS,
} from './meetingAdminModel.js'

const meetingAdminSource = readFileSync(new URL('./MeetingAdmin.jsx', import.meta.url), 'utf8')
const meetingRoomDetailSource = readFileSync(new URL('./MeetingRoomDetail.jsx', import.meta.url), 'utf8')
const minutesHistorySource = readFileSync(new URL('./MinutesHistory.jsx', import.meta.url), 'utf8')
const appStylesSource = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

function ruleBlock(source, selector) {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = source.match(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, 'm'))
  return match?.[1] ?? ''
}

test('meeting admin exposes active, closed, archived, and all filters', () => {
  assert.deepEqual(
    STATUS_FILTERS.map((filter) => filter.value),
    ['', 'active', 'closed', 'archived'],
  )
  assert.equal(labelForRoomStatus('active'), '进行中')
  assert.equal(labelForRoomStatus('closed'), '已关闭')
  assert.equal(labelForRoomStatus('archived'), '已归档')
})

test('meeting lifecycle actions follow the approved status matrix', () => {
  assert.deepEqual(actionsForRoomStatus('active'), ['detail', 'archive'])
  assert.deepEqual(actionsForRoomStatus('closed'), ['detail', 'reopen', 'archive'])
  assert.deepEqual(actionsForRoomStatus('archived'), ['detail', 'restore'])
})

test('meeting admin wires a dedicated room detail surface', () => {
  assert.match(meetingAdminSource, /import MeetingRoomDetail from '\.\/MeetingRoomDetail'/)
  assert.match(meetingAdminSource, /actionsForRoomStatus/)
  assert.match(meetingAdminSource, /setSelectedRoom/)
  assert.match(meetingRoomDetailSource, /getMessages/)
  assert.match(meetingRoomDetailSource, /MinutesHistory/)
  assert.match(meetingRoomDetailSource, /exportRoomMinutesMarkdown/)
})

test('meeting detail modal remains vertically scrollable', () => {
  const overlayBlock = ruleBlock(appStylesSource, '.delete-confirm-overlay--scrollable')
  const detailCardBlock = ruleBlock(appStylesSource, '.meeting-detail-card')

  assert.match(overlayBlock, /overflow-y:\s*auto;/)
  assert.match(detailCardBlock, /overflow-y:\s*auto;/)
  assert.doesNotMatch(detailCardBlock, /overflow:\s*hidden;/)
  assert.match(meetingRoomDetailSource, /delete-confirm-overlay--scrollable/)
})

test('minutes history modal gets extra size and scroll headroom', () => {
  const overlayBlock = ruleBlock(appStylesSource, '.delete-confirm-overlay--scrollable')
  const minutesCardBlock = ruleBlock(appStylesSource, '.minutes-history-card')

  assert.match(minutesHistorySource, /delete-confirm-overlay--scrollable/)
  assert.match(overlayBlock, /overflow-y:\s*auto;/)
  assert.match(minutesCardBlock, /width:\s*min\(980px,\s*95vw\);/)
  assert.match(minutesCardBlock, /max-height:\s*92vh;/)
  assert.match(minutesCardBlock, /overflow-y:\s*auto;/)
})
