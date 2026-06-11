import { useEffect, useMemo, useState } from 'react'
import { getAgents, updateAgent } from '../api/roomClient'

const EMPTY_FORM = {
  name: '',
  role: '',
  description: '',
  systemPrompt: '',
  enabled: true,
}

function AgentAdmin({ onBack }) {
  const [agents, setAgents] = useState([])
  const [selectedAgentId, setSelectedAgentId] = useState('')
  const [form, setForm] = useState(EMPTY_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [notice, setNotice] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)

  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentId) ?? null,
    [agents, selectedAgentId],
  )

  useEffect(() => {
    let isCurrent = true

    const loadAgents = async () => {
      setIsLoading(true)
      setErrorMessage('')
      try {
        const response = await getAgents()
        if (!isCurrent) {
          return
        }
        const nextAgents = response.agents ?? []
        setAgents(nextAgents)
        setSelectedAgentId((current) => current || nextAgents[0]?.id || '')
      } catch (error) {
        if (isCurrent) {
          setErrorMessage(error.message || '加载 Agent 列表失败。')
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false)
        }
      }
    }

    void loadAgents()

    return () => {
      isCurrent = false
    }
  }, [])

  useEffect(() => {
    if (!selectedAgent) {
      setForm(EMPTY_FORM)
      return
    }

    setForm({
      name: selectedAgent.name ?? '',
      role: selectedAgent.role ?? '',
      description: selectedAgent.description ?? '',
      systemPrompt: selectedAgent.systemPrompt ?? '',
      enabled: selectedAgent.enabled !== false,
    })
    setNotice('')
    setShowSystemPrompt(false)
  }, [selectedAgent])

  const handleFieldChange = (field, value) => {
    setForm((current) => ({ ...current, [field]: value }))
  }

  const handleSave = async (event) => {
    event.preventDefault()
    if (!selectedAgent || !form.name.trim()) {
      return
    }

    setIsSaving(true)
    setErrorMessage('')
    setNotice('')
    try {
      const updatedAgent = await updateAgent(selectedAgent.id, {
        name: form.name.trim(),
        role: form.role.trim(),
        description: form.description.trim(),
        systemPrompt: form.systemPrompt.trim(),
        enabled: form.enabled,
      })
      setAgents((current) => current.map((agent) => (agent.id === updatedAgent.id ? updatedAgent : agent)))
      setNotice('Agent 配置已保存。新建房间会使用最新配置，已存在房间的可用 Agent 也会同步更新。')
    } catch (error) {
      setErrorMessage(error.message || '保存 Agent 配置失败。')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <main className="app-shell app-shell--admin">
      <header className="page-header">
        <div>
          <p className="eyebrow">Agent 管理</p>
          <h1>配置会议室里的预定义角色</h1>
          <p className="section-copy">
            管理 Agent 的展示名称、职责说明、启用状态和系统提示词。停用后的 Agent 不会出现在新房间，也不会继续响应 @ 提及。
          </p>
        </div>
        <button className="button button--secondary" type="button" onClick={onBack}>
          返回会议入口
        </button>
      </header>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      {notice ? <p className="banner banner--success">{notice}</p> : null}

      <div className="admin-layout">
        <aside className="panel agent-admin-list">
          <div className="panel-header">
            <div className="panel-title-row">
              <h2>Agent 列表</h2>
              <span className="panel-badge panel-badge--neutral">{agents.length}</span>
            </div>
            <p className="panel-copy">选择一个 Agent 后，在右侧编辑它的会议职责和行为边界。</p>
          </div>

          {isLoading ? (
            <p className="empty-state sidebar-empty">正在加载...</p>
          ) : (
            <ul className="admin-agent-list">
              {agents.map((agent) => (
                <li key={agent.id}>
                  <button
                    className={`admin-agent-button${agent.id === selectedAgentId ? ' admin-agent-button--active' : ''}`}
                    type="button"
                    onClick={() => setSelectedAgentId(agent.id)}
                  >
                    <span className="admin-agent-name">{agent.name}</span>
                    <span className={`agent-state ${agent.enabled === false ? 'agent-state--off' : ''}`}>
                      {agent.enabled === false ? '停用' : '启用'}
                    </span>
                    <span className="admin-agent-role">{agent.role}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>

        <form className="panel admin-editor" onSubmit={handleSave}>
          <div className="panel-header">
            <div className="panel-title-row">
              <h2>{selectedAgent ? selectedAgent.name : '选择 Agent'}</h2>
              {selectedAgent ? <span className="panel-badge">{selectedAgent.mention}</span> : null}
            </div>
            <p className="panel-copy">名称会自动生成新的 @ 提及词；系统提示词为空时，会保留后端当前提示词。</p>
          </div>

          <div className="toggle-row">
            <div>
              <p className="toggle-title">当前状态：{form.enabled ? '已启用' : '已停用'}</p>
              <p className="field-hint">关闭后，这个 Agent 不会出现在会议室侧栏，也不会被触发。</p>
            </div>
            <label className="switch">
              <input
                type="checkbox"
                checked={form.enabled}
                disabled={!selectedAgent || isSaving}
                onChange={(event) => handleFieldChange('enabled', event.target.checked)}
              />
              <span />
            </label>
          </div>

          <div className="admin-form-grid">
            <div className="field-group">
              <label htmlFor="agent-name">Agent 名称</label>
              <input
                id="agent-name"
                type="text"
                value={form.name}
                onChange={(event) => handleFieldChange('name', event.target.value)}
                disabled={!selectedAgent || isSaving}
                maxLength={40}
                required
              />
            </div>

            <div className="field-group">
              <label htmlFor="agent-role">角色英文名 / 职位标签</label>
              <input
                id="agent-role"
                type="text"
                value={form.role}
                onChange={(event) => handleFieldChange('role', event.target.value)}
                disabled={!selectedAgent || isSaving}
                maxLength={80}
              />
            </div>
          </div>

          <div className="field-group">
            <label htmlFor="agent-description">会议职责说明</label>
            <textarea
              id="agent-description"
              className="text-input"
              value={form.description}
              onChange={(event) => handleFieldChange('description', event.target.value)}
              disabled={!selectedAgent || isSaving}
              rows={3}
              maxLength={240}
            />
          </div>

          <div className="field-group">
            <button
              className="collapse-toggle"
              type="button"
              aria-expanded={showSystemPrompt}
              onClick={() => setShowSystemPrompt((current) => !current)}
              disabled={!selectedAgent}
            >
              <span className="collapse-toggle-label">行为规则</span>
              <span className="collapse-toggle-hint">
                {form.systemPrompt ? '已自定义系统提示词' : '使用默认系统提示词'}
              </span>
              <span className={`collapse-chevron${showSystemPrompt ? ' collapse-chevron--open' : ''}`} aria-hidden="true">
                ▾
              </span>
            </button>
            {showSystemPrompt ? (
              <textarea
                id="agent-system-prompt"
                className="text-input text-input--prompt"
                value={form.systemPrompt}
                onChange={(event) => handleFieldChange('systemPrompt', event.target.value)}
                disabled={!selectedAgent || isSaving}
                rows={8}
                placeholder="留空则保留当前后端提示词。"
              />
            ) : null}
          </div>

          <div className="button-row">
            <span className="helper-text">保存后，名称对应的 @ 提及词会同步更新。</span>
            <button className="button button--primary" type="submit" disabled={!selectedAgent || isSaving || !form.name.trim()}>
              {isSaving ? '保存中...' : '保存配置'}
            </button>
          </div>
        </form>
      </div>
    </main>
  )
}

export default AgentAdmin
