import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import assert from 'node:assert/strict'

const componentSource = (name) => readFileSync(new URL(`./${name}.jsx`, import.meta.url), 'utf8')
const appStyles = readFileSync(new URL('../styles.css', import.meta.url), 'utf8')

test('admin and room surfaces use Mantine as the shared component layer', () => {
  const requiredImports = [
    'AdminGate',
    'AdminConsole',
    'MeetingAdmin',
    'AgentAdmin',
    'ChatRoom',
    'KnowledgePanel',
    'MeetingMinutesPanel',
    'MessageComposer',
    'MessageList',
    'RoomEntry',
    'RoomGateway',
    'RoomReadOnly',
    'NotFound',
    'MeetingRoomDetail',
    'MinutesHistory',
    'ModelProfileAdmin',
  ]

  for (const name of requiredImports) {
    assert.match(componentSource(name), /from '@mantine\/core'/, `${name} should import Mantine components`)
  }
})

test('migrated high-level surfaces do not add generic native controls', () => {
  const migratedSurfaces = [
    'AdminGate',
    'AdminConsole',
    'MeetingAdmin',
    'RoomEntry',
    'RoomGateway',
    'RoomReadOnly',
    'NotFound',
    'ModelProfileAdmin',
  ]

  for (const name of migratedSurfaces) {
    const source = componentSource(name)
    assert.doesNotMatch(source, /<button\b/, `${name} should use Mantine Button for visible actions`)
    assert.doesNotMatch(source, /<input\b/, `${name} should use Mantine inputs for visible fields`)
    assert.doesNotMatch(source, /<textarea\b/, `${name} should use Mantine Textarea for visible fields`)
    assert.doesNotMatch(source, /<select\b/, `${name} should use Mantine Select for visible choices`)
  }
})

test('entry dashboard navigation keeps native and Mantine items the same height', () => {
  assert.match(appStyles, /\.entry-dashboard-nav-item[\s\S]*min-height: 76px/)
  assert.match(appStyles, /\.entry-dashboard-nav \.mantine-Button-root\.entry-dashboard-nav-item[\s\S]*--button-height: 100%/)
  assert.match(appStyles, /@media \(max-width: 720px\)[\s\S]*\.entry-dashboard-nav \.mantine-Button-root\.entry-dashboard-nav-item[\s\S]*--button-height: 38px/)
})
