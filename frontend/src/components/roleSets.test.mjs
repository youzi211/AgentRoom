import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('./JoinScreen.jsx', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api/roomClient.js', import.meta.url), 'utf8')
const styles = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

test('room client fetches meeting role sets', () => {
  assert.match(apiSource, /export async function getAgentRoleSets/)
  assert.match(apiSource, /\/agent-role-sets/)
})

test('JoinScreen exposes role-set shortcuts without replacing manual Agent selection', () => {
  assert.match(source, /getAgentRoleSets/)
  assert.match(source, /roleSets/)
  assert.match(source, /handleApplyRoleSet/)
  assert.match(source, /templateIDs/)
  assert.match(source, /handleAgentToggle/)
  assert.match(source, /handleSelectAll/)
  assert.match(source, /handleDeselectAll/)
})

test('role-set shortcut controls have styles', () => {
  assert.match(source, /role-set-shortcuts/)
  assert.match(styles, /\.role-set-shortcuts/)
})
