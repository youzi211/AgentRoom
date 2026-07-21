import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./ModelProfileAdmin.jsx', import.meta.url), 'utf8')

test('model profile editor never refills a saved API key', () => {
  assert.match(source, /openEdit[\s\S]*apiKey:\s*''/)
  assert.match(source, /留空保留现有密钥/)
  assert.doesNotMatch(source, /apiKeyCiphertext/)
})

test('connection tests retain pending, success latency, and sanitized failure states', () => {
  assert.match(source, /profileTestResults/)
  assert.match(source, /pending:\s*true/)
  assert.match(source, /latencyMS/)
  assert.match(source, /color=\{profileTestResults\[profile\.id\]\.ok \? 'teal' : 'red'\}/)
  assert.match(source, /测试连接会向配置的模型发送最小真实请求/)
})

test('destructive model actions ask for confirmation and report reference conflicts', () => {
  assert.match(source, /设为[\s\S]*默认模型吗/)
  assert.match(source, /确定停用/)
  assert.match(source, /清除 API Key 后/)
  assert.match(source, /替换现有 API Key/)
  assert.match(source, /全局 Agent 或房间快照引用/)
})
