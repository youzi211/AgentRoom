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

test('JoinScreen shows each selectable agent runtime', () => {
  assert.match(source, /agent-runtime-inline/)
  assert.match(source, /agent\.runtime/)
  assert.match(styles, /\.agent-runtime-inline/)
})

test('JoinScreen uses public recent room summaries', () => {
  assert.match(apiSource, /export async function listRecentRooms/)
  assert.match(apiSource, /\/recent-rooms/)
  assert.doesNotMatch(source, /listRooms/)
  assert.match(source, /listRecentRooms\(\{ limit: 3 \}\)/)
  assert.match(source, /roomItem\.hasPasscode/)
  assert.match(source, /roomItem\.dialoguePolicy\?\.mode/)
  assert.match(source, /roomItem\.agentCount/)
  assert.doesNotMatch(source, /ownerName/)
  assert.doesNotMatch(source, /ownerParticipantName/)
})

test('JoinScreen uses public entry summary instead of fake stat values', () => {
  assert.match(apiSource, /export async function getEntrySummary/)
  assert.match(apiSource, /\/entry-summary/)
  assert.match(source, /getEntrySummary/)
  assert.match(source, /entrySummary/)
  assert.match(source, /formatEntryStatValue/)
  assert.doesNotMatch(source, /value: '6'/)
  assert.doesNotMatch(source, /value: '12'/)
  assert.doesNotMatch(source, /value: '24'/)
  assert.doesNotMatch(source, /value: '18'/)
})
