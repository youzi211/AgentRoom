import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const messageComposerSource = readFileSync(new URL('./MessageComposer.jsx', import.meta.url), 'utf8')
const chatRoomSource = readFileSync(new URL('./ChatRoom.jsx', import.meta.url), 'utf8')

let messageComposerModel = {}

try {
  messageComposerModel = await import('./messageComposerModel.js')
} catch {
  messageComposerModel = {}
}

const { buildMentionTargets, filterMentionTargets, findMentionQuery } = messageComposerModel

test('mention targets include other online participants alongside agents', () => {
  assert.equal(typeof buildMentionTargets, 'function')

  const targets = buildMentionTargets({
    agents: [
      { id: 'agent_pm', name: 'Product', mention: '@Product', role: 'PM' },
    ],
    participants: [
      { id: 'participant_alice', name: 'Alice' },
      { id: 'participant_bob', name: 'Bob' },
    ],
    currentParticipantName: 'Alice',
  })

  assert.deepEqual(
    targets.map(({ id, mention, kind, role }) => ({ id, mention, kind, role })),
    [
      { id: 'agent_pm', mention: '@Product', kind: 'agent', role: 'PM' },
      { id: 'participant_bob', mention: '@Bob', kind: 'participant', role: '参会者' },
    ],
  )
})

test('mention query and filtering can resolve a human participant target', () => {
  assert.equal(typeof findMentionQuery, 'function')
  assert.equal(typeof filterMentionTargets, 'function')

  const targets = [
    { id: 'agent_pm', name: 'Product', mention: '@Product', role: 'PM', kind: 'agent' },
    { id: 'participant_bob', name: 'Bob', mention: '@Bob', role: '参会者', kind: 'participant' },
  ]

  assert.equal(findMentionQuery('请 @Bo 继续补充', 5), 'Bo')
  assert.deepEqual(
    filterMentionTargets(targets, 'Bo').map((target) => target.id),
    ['participant_bob'],
  )
})

test('composer only captures Enter for autocomplete when there are visible candidates', () => {
  assert.match(messageComposerSource, /showAutocomplete\s*&&\s*filteredMentionTargets\.length\s*>\s*0/)
})

test('chat room passes participants into the message composer mention targets', () => {
  assert.match(chatRoomSource, /<MessageComposer[\s\S]*participants=\{participants\}/)
  assert.match(chatRoomSource, /<MessageComposer[\s\S]*currentParticipantName=\{participantName\}/)
})
