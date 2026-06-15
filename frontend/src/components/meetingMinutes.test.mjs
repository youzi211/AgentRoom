import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import { buildLocalMeetingMinutesMarkdown, normalizeMinutesPayload } from './meetingMinutes.js'

test('buildLocalMeetingMinutesMarkdown exports room context, focus points, and messages', () => {
  const markdown = buildLocalMeetingMinutesMarkdown({
    room: { name: 'v0.2 上线预备会' },
    roomId: 'room_123',
    participants: [{ name: '小明' }, { name: '小李' }],
    agents: [{ name: '会议秘书' }],
    focusPoints: [
      { category: '决策', content: '先做上线安全闭环。' },
      { category: '计划', content: '补 Docker Compose。' },
    ],
    messages: [
      { senderName: '小明', senderType: 'human', content: '今天确认 v0.2 范围。', createdAt: '2026-06-15T09:00:00Z' },
      { senderName: '会议秘书', senderType: 'agent', content: '建议拆成安全、纪要、部署三段。', createdAt: '2026-06-15T09:01:00Z' },
    ],
  })

  assert.match(markdown, /^# v0\.2 上线预备会/m)
  assert.match(markdown, /房间 ID：`room_123`/)
  assert.match(markdown, /参会者：小明、小李/)
  assert.match(markdown, /Agent：会议秘书/)
  assert.match(markdown, /- \*\*决策\*\*：先做上线安全闭环。/)
  assert.match(markdown, /- \*\*小明\*\*：今天确认 v0\.2 范围。/)
})

test('normalizeMinutesPayload accepts common backend payload shapes', () => {
  assert.equal(normalizeMinutesPayload({ markdown: '# 后端纪要' }), '# 后端纪要')
  assert.equal(normalizeMinutesPayload({ minutes: { markdown: '# 嵌套纪要' } }), '# 嵌套纪要')
  assert.equal(normalizeMinutesPayload('# 纯文本纪要'), '# 纯文本纪要')
})
