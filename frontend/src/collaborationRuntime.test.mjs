import { strict as assert } from 'node:assert'
import { test } from 'node:test'

import {
  collaborationChoices,
  COMPATIBLE_COLLABORATION_POLICY,
  legacyCollaborationCapabilities,
  normalizeCollaborationCapabilities,
  reconcileCollaborationPolicy,
} from './collaborationRuntime.js'

test('collaboration choices only expose enabled and ready runtime capabilities', () => {
  const capabilities = normalizeCollaborationCapabilities({
    mode: 'remote',
    ready: true,
    supportedProtocolVersions: ['v1', 'v1'],
    engines: [
      { engine: 'native', version: '1.0', enabled: true, ready: true },
      { engine: 'autogen', version: '0.7', enabled: true, ready: false },
      { engine: 'future', version: '2.0', enabled: true, ready: true },
    ],
    supportedTriggerModes: ['mention_only', 'automatic', 'future_mode'],
  })

  assert.deepEqual(collaborationChoices(capabilities), {
    engines: [{ engine: 'native', version: '1.0', enabled: true, ready: true }],
    triggerModes: ['mention_only', 'automatic'],
  })
  assert.deepEqual(capabilities.supportedProtocolVersions, ['v1'])
})

test('collaboration policy follows available grey engines and preserves compatibility fallback', () => {
  assert.deepEqual(
    reconcileCollaborationPolicy(legacyCollaborationCapabilities(), { engine: 'autogen', triggerMode: 'automatic' }),
    COMPATIBLE_COLLABORATION_POLICY,
  )

  const autogenOnly = {
    mode: 'remote',
    ready: true,
    engines: [{ engine: 'autogen', version: '0.7', enabled: true, ready: true }],
    supportedTriggerModes: ['automatic'],
  }
  assert.deepEqual(reconcileCollaborationPolicy(autogenOnly, COMPATIBLE_COLLABORATION_POLICY), {
    engine: 'autogen',
    triggerMode: 'automatic',
  })

  assert.equal(reconcileCollaborationPolicy({ ...autogenOnly, ready: false }), null)
})
