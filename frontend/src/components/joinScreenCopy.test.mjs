import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('./JoinScreen.jsx', import.meta.url), 'utf8')

test('JoinScreen keeps key Chinese copy readable', () => {
  assert.match(source, /AI 会议工作台/)
  assert.match(source, /角色化 Agent 协作/)
  assert.match(source, /创建会议室/)
  assert.match(source, /加入会议室/)
  assert.match(source, /已有房间/)
  assert.match(source, /房间口令/)
  assert.doesNotMatch(source, /锛|绠|鎴|浼|鐨|鍙|鈥|�/)
})
