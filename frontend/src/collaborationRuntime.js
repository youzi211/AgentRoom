export const COMPATIBLE_COLLABORATION_POLICY = Object.freeze({
  engine: 'native',
  triggerMode: 'mention_only',
})

const KNOWN_ENGINES = new Set(['native', 'autogen'])
const KNOWN_TRIGGER_MODES = new Set(['mention_only', 'automatic'])

export function legacyCollaborationCapabilities() {
  return {
    mode: 'legacy',
    ready: true,
    supportedProtocolVersions: [],
    engines: [
      {
        engine: 'native',
        version: 'legacy-go',
        enabled: true,
        ready: true,
      },
    ],
    supportedTriggerModes: ['mention_only'],
  }
}

export function normalizeCollaborationCapabilities(payload) {
  const engines = Array.isArray(payload?.engines)
    ? payload.engines
      .filter((item) => item && typeof item.engine === 'string' && item.engine.trim())
      .map((item) => ({
        engine: item.engine.trim(),
        version: typeof item.version === 'string' ? item.version.trim() : '',
        enabled: item.enabled === true,
        ready: item.ready === true,
      }))
    : []

  return {
    mode: typeof payload?.mode === 'string' && payload.mode.trim() ? payload.mode.trim() : 'legacy',
    ready: payload?.ready === true,
    supportedProtocolVersions: uniqueStrings(payload?.supportedProtocolVersions),
    engines,
    supportedTriggerModes: uniqueStrings(payload?.supportedTriggerModes),
  }
}

export function collaborationChoices(capabilities) {
  const normalized = normalizeCollaborationCapabilities(capabilities)
  if (!normalized.ready) {
    return { engines: [], triggerModes: [] }
  }

  return {
    engines: normalized.engines.filter((item) => (
      KNOWN_ENGINES.has(item.engine) && item.enabled && item.ready
    )),
    triggerModes: normalized.supportedTriggerModes.filter((mode) => KNOWN_TRIGGER_MODES.has(mode)),
  }
}

export function reconcileCollaborationPolicy(capabilities, current = COMPATIBLE_COLLABORATION_POLICY) {
  const choices = collaborationChoices(capabilities)
  if (choices.engines.length === 0 || choices.triggerModes.length === 0) {
    return null
  }

  const engine = choices.engines.some((item) => item.engine === current?.engine)
    ? current.engine
    : choices.engines[0].engine
  const triggerMode = choices.triggerModes.includes(current?.triggerMode)
    ? current.triggerMode
    : choices.triggerModes[0]

  return { engine, triggerMode }
}

export function labelForCollaborationEngine(engine) {
  if (engine === 'native') {
    return 'Native'
  }
  if (engine === 'autogen') {
    return 'AutoGen'
  }
  return engine || '未知引擎'
}

export function labelForTriggerMode(mode) {
  if (mode === 'mention_only') {
    return '兼容模式'
  }
  if (mode === 'automatic') {
    return '自动协作'
  }
  return mode || '未知模式'
}

function uniqueStrings(values) {
  if (!Array.isArray(values)) {
    return []
  }
  return Array.from(new Set(values.filter((value) => typeof value === 'string' && value.trim()).map((value) => value.trim())))
}
