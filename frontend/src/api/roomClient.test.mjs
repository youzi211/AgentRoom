import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import { buildCreateRoomPayload } from './roomClient.js'

test('buildCreateRoomPayload omits dialogue policy for default fanout rooms', () => {
  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm'], 'secret', 'mention_fanout'), {
    name: 'Planning',
    agentIds: ['pm'],
    passcode: 'secret',
  })
})

test('buildCreateRoomPayload includes guided dialogue policy when requested', () => {
  assert.deepEqual(buildCreateRoomPayload('Planning', ['pm', 'qa'], '', 'guided_dialogue'), {
    name: 'Planning',
    agentIds: ['pm', 'qa'],
    passcode: '',
    dialoguePolicy: {
      mode: 'guided_dialogue',
    },
  })
})
