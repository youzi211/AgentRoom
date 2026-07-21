import assert from 'node:assert/strict'
import test from 'node:test'
import {
  createEmptyModelProfile,
  createModelProfilePayload,
  draftModelProfileTestPayload,
  updateModelProfilePayload,
  usesSavedProfileForTest,
} from './modelProfileForm.js'

test('new profile payload is normalized without inventing secret values', () => {
  assert.deepEqual(createModelProfilePayload({
    ...createEmptyModelProfile('deepagent'),
    name: ' Deep ',
    baseURL: ' https://example.test/v1 ',
    modelName: ' model-x ',
    apiKey: 'new-key',
    isDefault: true,
  }), {
    name: 'Deep',
    runtimeScope: 'deepagent',
    protocol: 'openai_chat_completions',
    baseURL: 'https://example.test/v1',
    modelName: 'model-x',
    apiKey: 'new-key',
    enabled: true,
    isDefault: true,
  })
})

test('editing with a blank key preserves the saved secret', () => {
  const payload = updateModelProfilePayload({
    name: 'Go', baseURL: 'https://example.test/v1', modelName: 'm', enabled: true,
    apiKey: '', clearAPIKey: false, hasAPIKey: true, apiKeyHint: '…1234',
  })
  assert.equal(Object.hasOwn(payload, 'apiKey'), false)
  assert.equal(payload.clearAPIKey, false)
})

test('replacement and clear secret intentions are explicit', () => {
  assert.deepEqual(updateModelProfilePayload({
    name: 'Go', baseURL: 'https://example.test/v1', modelName: 'm', enabled: true,
    apiKey: 'replacement', clearAPIKey: false,
  }).apiKey, 'replacement')
  const cleared = updateModelProfilePayload({
    name: 'Go', baseURL: 'https://example.test/v1', modelName: 'm', enabled: true,
    apiKey: '', clearAPIKey: true,
  })
  assert.equal(cleared.clearAPIKey, true)
  assert.equal(Object.hasOwn(cleared, 'apiKey'), false)
})

test('draft connection payload contains only the accepted API contract', () => {
  assert.deepEqual(draftModelProfileTestPayload({
    name: 'ignored', runtimeScope: 'go', protocol: 'ignored', enabled: false,
    baseURL: ' https://example.test/v1 ', modelName: ' model ', apiKey: 'draft-key',
  }), { baseURL: 'https://example.test/v1', modelName: 'model', apiKey: 'draft-key' })
  assert.equal(usesSavedProfileForTest('profile-1', { apiKey: '' }), true)
  assert.equal(usesSavedProfileForTest('profile-1', { apiKey: 'replacement' }), false)
  assert.equal(usesSavedProfileForTest('new', { apiKey: '' }), false)
})
