import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('./AgentAdmin.jsx', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api/roomClient.js', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

test('room client fetches agent templates from the dedicated endpoint', () => {
  assert.match(apiSource, /export async function getAgentTemplates/)
  assert.match(apiSource, /\/agent-templates/)
})

test('AgentAdmin lets templates prefill editable Agent fields', () => {
  assert.match(source, /getAgentTemplates/)
  assert.match(source, /selectedTemplateId/)
  assert.match(source, /handleApplyTemplate/)
  assert.match(source, /systemPrompt/)
  assert.match(source, /createAgent/)
  assert.match(source, /updateAgent/)
})

test('AgentAdmin includes template picker styling hooks', () => {
  assert.match(source, /agent-template-picker/)
  assert.match(styles, /\.agent-template-picker/)
})
