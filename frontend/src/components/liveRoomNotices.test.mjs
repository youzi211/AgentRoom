import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const chatRoomSource = readFileSync(new URL('./ChatRoom.jsx', import.meta.url), 'utf8')

let liveRoomNotices = {}

try {
  liveRoomNotices = await import('./liveRoomNotices.js')
} catch {
  liveRoomNotices = {}
}

const { buildParticipantJoinedNotice, mergeTimelineMessages } = liveRoomNotices

test('participant join event becomes a local system message for other viewers', () => {
  assert.equal(typeof buildParticipantJoinedNotice, 'function')

  const notice = buildParticipantJoinedNotice({
    participant: { id: 'participant_bob', name: 'Bob' },
    currentParticipantID: 'participant_alice',
    now: '2026-06-16T10:01:00.000Z',
  })

  assert.deepEqual(notice, {
    id: 'notice:participant_joined:participant_bob:2026-06-16T10:01:00.000Z',
    senderID: 'system',
    senderName: '系统',
    senderType: 'system',
    content: 'Bob 加入了会议',
    createdAt: '2026-06-16T10:01:00.000Z',
  })
})

test('participant join event does not generate a notice for the current participant', () => {
  assert.equal(typeof buildParticipantJoinedNotice, 'function')
  assert.equal(
    buildParticipantJoinedNotice({
      participant: { id: 'participant_alice', name: 'Alice' },
      currentParticipantID: 'participant_alice',
      now: '2026-06-16T10:01:00.000Z',
    }),
    null,
  )
})

test('local join notices merge into the visible timeline without polluting persisted messages', () => {
  assert.equal(typeof mergeTimelineMessages, 'function')

  const merged = mergeTimelineMessages(
    [
      { id: 'message_1', createdAt: '2026-06-16T10:00:00.000Z', senderType: 'human', content: 'hello' },
      { id: 'message_2', createdAt: '2026-06-16T10:02:00.000Z', senderType: 'human', content: 'follow-up' },
    ],
    [
      {
        id: 'notice:participant_joined:participant_bob:2026-06-16T10:01:00.000Z',
        createdAt: '2026-06-16T10:01:00.000Z',
        senderType: 'system',
        content: 'Bob 加入了会议',
      },
    ],
  )

  assert.deepEqual(
    merged.map((item) => item.id),
    ['message_1', 'notice:participant_joined:participant_bob:2026-06-16T10:01:00.000Z', 'message_2'],
  )
})

test('chat room renders live notices only in the message timeline, not in persisted meeting data', () => {
  assert.match(chatRoomSource, /const \[liveNotices,\s*setLiveNotices\] = useState\(\[\]\)/)
  assert.match(chatRoomSource, /const visibleMessages = mergeTimelineMessages\(messages,\s*liveNotices\)/)
  assert.match(chatRoomSource, /<MessageList[\s\S]*messages=\{visibleMessages\}/)
  assert.match(chatRoomSource, /<MeetingMinutesPanel[\s\S]*messages=\{messages\}/)
})
