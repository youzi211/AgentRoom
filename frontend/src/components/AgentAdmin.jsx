import { useCallback, useEffect, useMemo, useState } from 'react'
import { createAgent, deleteAgent, getAgents, updateAgent } from '../api/roomClient'

const EMPTY_FORM = {
  name: '',
  role: '',
  description: '',
  systemPrompt: '',
  enabled: true,
}

const CREATE_MODE = '__create__'

function AgentAdmin({ onBack }) {
  const [agents, setAgents] = useState([])
  const [selectedAgentId, setSelectedAgentId] = useState('')
  const [form, setForm] = useState(EMPTY_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [notice, setNotice] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState(null)

  const isCreating = selectedAgentId === CREATE_MODE

  const selectedAgent = useMemo(
    () => (isCreating ? null : agents.find((agent) => agent.id === selectedAgentId) ?? null),
    [agents, selectedAgentId, isCreating],
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
    if (isCreating) {
      setForm(EMPTY_FORM)
      setNotice('')
      setShowSystemPrompt(false)
      return
    }

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
  }, [selectedAgent, isCreating])

  const handleFieldChange = useCallback((field, value) => {
    setForm((current) => ({ ...current, [field]: value }))
  }, [])

  const handleStartCreate = () => {
    setSelectedAgentId(CREATE_MODE)
  }

  const handleCancelCreate = () => {
    setSelectedAgentId(agents[0]?.id || '')
  }

  const handleSave = async (event) => {
    event.preventDefault()
    if (!form.name.trim()) {
      return
    }

    setIsSaving(true)
    setErrorMessage('')
    setNotice('')

    try {
      if (isCreating) {
        const created = await createAgent({
          name: form.name.trim(),
          role: form.role.trim(),
          description: form.description.trim(),
          systemPrompt: form.systemPrompt.trim(),
          enabled: form.enabled,
        })
        const nextAgents = [...agents, created]
        setAgents(nextAgents)
        setSelectedAgentId(created.id)
        setNotice('Agent 已创建。它可以被拉入新的会议室。')
      } else if (selectedAgent) {
        const updatedAgent = await updateAgent(selectedAgent.id, {
          name: form.name.trim(),
          role: form.role.trim(),
          description: form.description.trim(),
          systemPrompt: form.systemPrompt.trim(),
          enabled: form.enabled,
        })
        setAgents((current) => current.map((agent) => (agent.id === updatedAgent.id ? updatedAgent : agent)))
        setNotice('Agent 配置已保存。新建房间会使用最新配置，已存在房间的 Agent 快照不会改变。')
      }
    } catch (error) {
      setErrorMessage(error.message || (isCreating ? '创建 Agent 失败。' : '保存 Agent 配置失败。'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleDeleteRequest = (agent) => {
    setDeleteTarget(agent)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) {
      return
    }

    setIsSaving(true)
    setErrorMessage('')
    setNotice('')

    try {
      await deleteAgent(deleteTarget.id)
      const nextAgents = agents.filter((agent) => agent.id !== deleteTarget.id)
      setAgents(nextAgents)
      if (selectedAgentId === deleteTarget.id) {
        setSelectedAgentId(nextAgents[0]?.id || '')
      }
      setNotice(`Agent「${deleteTarget.name}」已删除。已有房间中的快照不受影响。`)
    } catch (error) {
      setErrorMessage(error.message || '删除 Agent 失败。')
    } finally {
      setIsSaving(false)
      setDeleteTarget(null)
    }
  }

  const handleDeleteCancel = () => {
    setDeleteTarget(null)
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

      {deleteTarget ? (
        <div className="delete-confirm-overlay" role="dialog" aria-modal="true">
          <div className="delete-confirm-card">
            <h2>确认删除 Agent</h2>
            <p>
              确定要删除 Agent「<strong>{deleteTarget.name}</strong>」吗？此操作不可撤销。
            </p>
            <p className="helper-text">
              已有房间中该 Agent 的快照不会被删除，历史消息也不受影响。但新建房间将不再包含此 Agent。
            </p>
            <div className="button-row">
              <button className="button button--secondary" type="button" onClick={handleDeleteCancel} disabled={isSaving}>
                取消
              </button>
              <button className="button button--danger" type="button" onClick={handleDeleteConfirm} disabled={isSaving}>
                {isSaving ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

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
            <>
              <ul className="admin-agent-list">
                {agents.map((agent) => (
                  <li key={agent.id} className="admin-agent-list-item">
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
                    <button
                      className="agent-delete-button"
                      type="button"
                      title="删除此 Agent"
                      onClick={() => handleDeleteRequest(agent)}
                      disabled={isSaving}
                    >
                      ✕
                    </button>
                  </li>
                ))}
              </ul>
              <div className="admin-agent-add">
                <button className="button button--ghost" type="button" onClick={handleStartCreate} disabled={isSaving}>
                  + 新增 Agent
                </button>
              </div>
            </>
          )}
        </aside>

        <form className="panel admin-editor" onSubmit={handleSave}>
          <div className="panel-header">
            <div className="panel-title-row">
              <h2>{isCreating ? '新增 Agent' : selectedAgent ? selectedAgent.name : '选择 Agent'}</h2>
              {selectedAgent ? <span className="panel-badge">{selectedAgent.mention}</span> : null}
              {isCreating ? <span className="panel-badge">新建</span> : null}
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
                disabled={(!selectedAgent && !isCreating) || isSaving}
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
                disabled={(!selectedAgent && !isCreating) || isSaving}
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
                disabled={(!selectedAgent && !isCreating) || isSaving}
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
              disabled={(!selectedAgent && !isCreating) || isSaving}
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
              disabled={!selectedAgent && !isCreating}
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
                disabled={(!selectedAgent && !isCreating) || isSaving}
                rows={8}
                placeholder="留空则保留当前后端提示词。"
              />
            ) : null}
          </div>

          <div className="button-row">
            {isCreating ? (
              <>
                <button className="button button--secondary" type="button" onClick={handleCancelCreate} disabled={isSaving}>
                  取消
                </button>
                <button className="button button--primary" type="submit" disabled={isSaving || !form.name.trim()}>
                  {isSaving ? '创建中...' : '创建 Agent'}
                </button>
              </>
            ) : (
              <>
                <span className="helper-text">保存后，名称对应的 @ 提及词会同步更新。</span>
                <button className="button button--primary" type="submit" disabled={!selectedAgent || isSaving || !form.name.trim()}>
                  {isSaving ? '保存中...' : '保存配置'}
                </button>
              </>
            )}
          </div>
        </form>
      </div>
    </main>
  )
}

export default AgentAdmin
