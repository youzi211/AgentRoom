export const EMPTY_MODEL_PROFILE = Object.freeze({
  name: '',
  runtimeScope: 'go',
  protocol: 'openai_chat_completions',
  baseURL: '',
  modelName: '',
  apiKey: '',
  enabled: true,
  isDefault: false,
  clearAPIKey: false,
})

export function createEmptyModelProfile(runtimeScope = 'go') {
  return { ...EMPTY_MODEL_PROFILE, runtimeScope }
}

export function createModelProfilePayload(form) {
  return {
    name: form.name.trim(),
    runtimeScope: form.runtimeScope,
    protocol: form.protocol || 'openai_chat_completions',
    baseURL: form.baseURL.trim(),
    modelName: form.modelName.trim(),
    apiKey: form.apiKey,
    enabled: form.enabled !== false,
    isDefault: form.isDefault === true,
  }
}

export function updateModelProfilePayload(form) {
  const payload = {
    name: form.name.trim(),
    baseURL: form.baseURL.trim(),
    modelName: form.modelName.trim(),
    enabled: form.enabled !== false,
    clearAPIKey: form.clearAPIKey === true,
  }
  if (form.apiKey) {
    payload.apiKey = form.apiKey
  }
  return payload
}

export function draftModelProfileTestPayload(form) {
  return {
    baseURL: form.baseURL.trim(),
    modelName: form.modelName.trim(),
    apiKey: form.apiKey,
  }
}

export function usesSavedProfileForTest(editing, form) {
  return Boolean(editing && editing !== 'new' && !form.apiKey)
}
