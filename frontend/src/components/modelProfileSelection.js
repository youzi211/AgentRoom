export function runtimeScopeForAgent(runtime) {
  return runtime === 'deepagent' ? 'deepagent' : 'go'
}

export function runtimeDefaultModelLabel(runtime) {
  return runtimeScopeForAgent(runtime) === 'deepagent'
    ? '使用 DeepAgent 默认模型'
    : '使用 Go 默认模型'
}

export function profileOptionsForRuntime(profiles, runtime) {
  const scope = runtimeScopeForAgent(runtime)
  return profiles
    .filter((profile) => profile.enabled !== false && profile.runtimeScope === scope)
    .map((profile) => ({
      value: profile.id,
      label: `${profile.name} · ${profile.modelName || '未命名模型'}${profile.isDefault ? '（默认）' : ''}`,
    }))
}

export function buildAgentModelPayload(form) {
  return { runtime: form.runtime || 'llm', modelProfileID: form.modelProfileID || '' }
}

export function buildAgentSavePayload(form) {
  return {
    name: form.name.trim(),
    role: form.role.trim(),
    description: form.description.trim(),
    systemPrompt: form.systemPrompt.trim(),
    enabled: form.enabled,
    ...buildAgentModelPayload(form),
  }
}

export function reconcileModelProfileBinding(profiles, modelProfileID, nextRuntime) {
  if (!modelProfileID) {
    return ''
  }
  const compatibleIDs = new Set(profileOptionsForRuntime(profiles, nextRuntime).map((profile) => profile.value))
  return compatibleIDs.has(modelProfileID) ? modelProfileID : ''
}

export function describeAgentModelBinding(agent, profiles) {
  if (!agent?.modelProfileID) {
    return runtimeDefaultModelLabel(agent?.runtime)
  }
  const profile = profiles.find((candidate) => candidate.id === agent.modelProfileID)
  if (!profile) {
    return '专用模型（不可用）'
  }
  return `${profile.name}${profile.enabled === false ? '（已停用）' : ''}`
}
