import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  createAgent,
  deleteAgent,
  deleteKnowledgeDocument,
  getAgentKnowledge,
  getAgents,
  updateAgent,
  uploadAgentKnowledge,
} from '../api/roomClient'
import KnowledgePanel from './KnowledgePanel'

const EMPTY_FORM = {
  name: '',
  role: '',
  description: '',
  systemPrompt: '',
  enabled: true,
}

const CREATE_MODE = '__create__'

function AgentAdmin({ onBack, embedded = false }) {
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
        setNotice('Agent 已创建。它可以被加入新的会议室。')
      } else if (selectedAgent) {
        const updatedAgent = await updateAgent(selectedAgent.id, {
          name: form.name.trim(),
          role: form.role.trim(),
          description: form.description.trim(),
          systemPrompt: form.systemPrompt.trim(),
          enabled: form.enabled,
        })
        setAgents((current) => current.map((agent) => (agent.id === updatedAgent.id ? updatedAgent : agent)))
        setNotice('Agent 配置已保存。新建房间会使用最新配置，已存在房间中的 Agent 快照不会自动改变。')
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
      setNotice(`Agent「${deleteTarget.name}」已删除。已存在房间中的快照不受影响。`)
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

  const containerClass = embedded ? 'admin-section' : 'workbench workbench--admin'

  return (
    <main className={containerClass}>
      {embedded ? null : (
        <header className="app-bar">
          <div className="brand-lockup">
            <span className="brand-mark">AR</span>
            <div>
              <strong>Agent 管理</strong>
              <span>配置会议室里的预定义角色</span>
            </div>
          </div>
          <nav className="app-nav" aria-label="管理导航">
            <span className="app-nav-item app-nav-item--active">Agent 配置</span>
            <button className="app-nav-item" type="button" onClick={onBack}>
              会议入口
            </button>
          </nav>
        </header>
      )}

      <section className="admin-hero">
        <div>
          <p className="eyebrow">管理控制台</p>
          <h1>管理 Agent、角色模板与专属知识库</h1>
          <p className="section-copy">
            这里维护的是可被新会议选择的角色模板。停用后，该 Agent 不会出现在新房间的可选列表，也不会响应新的 @
            提及。
          </p>
        </div>
        <button className="button button--primary" type="button" onClick={handleStartCreate} disabled={isSaving}>
          新增 Agent
        </button>
      </section>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      {notice ? <p className="banner banner--success">{notice}</p> : null}

      {deleteTarget ? (
        <div className="delete-confirm-overlay" role="dialog" aria-modal="true">
          <div className="delete-confirm-card">
            <h2>确认删除 Agent</h2>
            <p>
              确定要删除 Agent <strong>{deleteTarget.name}</strong> 吗？此操作不可撤销。
            </p>
            <p className="helper-text">
              已有房间中该 Agent 的快照不会被删除，历史消息也不受影响，但新建房间将不再包含这个 Agent。
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
            <p className="panel-copy">选择一个 Agent 后，在右侧编辑它的职责、角色模板和知识库。</p>
          </div>

          {isLoading ? (
            <p className="sidebar-empty">正在加载...</p>
          ) : agents.length === 0 ? (
            <p className="sidebar-empty">暂无 Agent。点击右上角新增第一个角色。</p>
          ) : (
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
                    <span className="admin-agent-role">{agent.role || '未设置角色标签'}</span>
                  </button>
                  <button
                    className="agent-delete-button"
                    type="button"
                    title="删除这个 Agent"
                    onClick={() => handleDeleteRequest(agent)}
                    disabled={isSaving}
                  >
                    删除
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>

        <form className="panel admin-editor" onSubmit={handleSave}>
          <div className="panel-header panel-header--horizontal">
            <div>
              <h2>{isCreating ? '新增 Agent' : selectedAgent ? selectedAgent.name : '选择 Agent'}</h2>
              <p className="panel-copy">名称会生成 @ 提及词；角色模板留空时，系统只使用代码内置的会议规则。</p>
            </div>
            <div className="panel-badge-group">
              {selectedAgent ? <span className="panel-badge">{selectedAgent.mention}</span> : null}
              {isCreating ? <span className="panel-badge">新建</span> : null}
            </div>
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
              <span className="collapse-toggle-label">角色模板</span>
              <span className="collapse-toggle-hint">
                {form.systemPrompt ? '已自定义角色模板' : '使用默认角色模板'}
              </span>
              <span className={`collapse-chevron${showSystemPrompt ? ' collapse-chevron--open' : ''}`} aria-hidden="true">
                ▲
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
                placeholder="创建时留空表示不追加额外角色模板；更新时留空会保留当前模板。"
              />
            ) : null}
          </div>

          <div className="agent-knowledge-section">
            <KnowledgePanel
              key={selectedAgent?.id || CREATE_MODE}
              title="Agent 知识库"
              description="上传 Markdown 文档，只有当前 Agent 在会议中发言时会参考这些知识。"
              disabled={isCreating || !selectedAgent}
              emptyText={
                isCreating
                  ? '创建 Agent 后即可上传知识文档。'
                  : '暂无知识文档。上传 .md 后，这个 Agent 会在回答时参考它们。'
              }
              listDocuments={selectedAgent ? () => getAgentKnowledge(selectedAgent.id) : null}
              onUploadDocument={selectedAgent ? (file) => uploadAgentKnowledge(selectedAgent.id, file) : null}
              onDeleteDocument={deleteKnowledgeDocument}
            />
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
