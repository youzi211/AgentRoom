import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildAgentModelPayload,
  buildAgentSavePayload,
  describeAgentModelBinding,
  profileOptionsForRuntime,
  reconcileModelProfileBinding,
  runtimeDefaultModelLabel,
} from './modelProfileSelection.js'

test('filters enabled profiles by agent runtime', () => {
  const profiles = [
    { id: 'go', name: 'Go', runtimeScope: 'go', enabled: true },
    { id: 'deep', name: 'Deep', runtimeScope: 'deepagent', enabled: true },
    { id: 'off', name: 'Off', runtimeScope: 'go', enabled: false },
  ]
  assert.deepEqual(profileOptionsForRuntime(profiles, 'llm').map((item) => item.value), ['go'])
  assert.deepEqual(profileOptionsForRuntime(profiles, 'deepagent').map((item) => item.value), ['deep'])
})

test('agent payload carries runtime and nullable model binding', () => {
  assert.deepEqual(buildAgentModelPayload({ runtime: 'deepagent', modelProfileID: 'deep' }), {
    runtime: 'deepagent',
    modelProfileID: 'deep',
  })
  assert.deepEqual(buildAgentModelPayload({ runtime: 'llm', modelProfileID: '' }), {
    runtime: 'llm',
    modelProfileID: '',
  })
})

test('default inheritance labels follow the selected runtime', () => {
  assert.equal(runtimeDefaultModelLabel('llm'), '使用 Go 默认模型')
  assert.equal(runtimeDefaultModelLabel('deepagent'), '使用 DeepAgent 默认模型')
  assert.equal(describeAgentModelBinding({ runtime: 'llm', modelProfileID: '' }, []), '使用 Go 默认模型')
  assert.equal(describeAgentModelBinding({ runtime: 'deepagent' }, []), '使用 DeepAgent 默认模型')
})

test('runtime switching preserves only an enabled compatible binding', () => {
  const profiles = [
    { id: 'go', runtimeScope: 'go', enabled: true },
    { id: 'deep', runtimeScope: 'deepagent', enabled: true },
    { id: 'off', runtimeScope: 'deepagent', enabled: false },
  ]

  assert.equal(reconcileModelProfileBinding(profiles, 'go', 'llm'), 'go')
  assert.equal(reconcileModelProfileBinding(profiles, 'go', 'deepagent'), '')
  assert.equal(reconcileModelProfileBinding(profiles, 'off', 'deepagent'), '')
  assert.equal(reconcileModelProfileBinding(profiles, '', 'deepagent'), '')
})

test('agent save payload trims fields and explicitly clears default inheritance', () => {
  assert.deepEqual(buildAgentSavePayload({
    name: '  Researcher  ',
    role: ' analyst ',
    description: ' investigates ',
    systemPrompt: ' be precise ',
    enabled: true,
    runtime: 'deepagent',
    modelProfileID: '',
  }), {
    name: 'Researcher',
    role: 'analyst',
    description: 'investigates',
    systemPrompt: 'be precise',
    enabled: true,
    runtime: 'deepagent',
    modelProfileID: '',
  })
})

test('binding descriptions expose profile state without secret fields', () => {
  const profiles = [{ id: 'profile-1', name: 'Research', enabled: false, apiKeyHint: 'secret' }]
  assert.equal(describeAgentModelBinding({ runtime: 'llm', modelProfileID: 'profile-1' }, profiles), 'Research（已停用）')
  assert.equal(describeAgentModelBinding({ runtime: 'llm', modelProfileID: 'missing' }, profiles), '专用模型（不可用）')
})
