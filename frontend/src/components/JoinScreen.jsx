import { useEffect, useMemo, useState } from 'react'
import { getAgents } from '../api/roomClient'

function JoinScreen({ errorMessage, isSubmitting, onCreateRoom, onJoinRoom, onOpenAgentAdmin }) {
  const [createDisplayName, setCreateDisplayName] = useState('')
  const [joinDisplayName, setJoinDisplayName] = useState('')
  const [roomName, setRoomName] = useState('')
  const [roomId, setRoomId] = useState('')
  const [createPasscode, setCreatePasscode] = useState('')
  const [dialogueMode, setDialogueMode] = useState('mention_fanout')
  const [joinPasscode, setJoinPasscode] = useState('')
  const [availableAgents, setAvailableAgents] = useState([])
  const [selectedAgentIds, setSelectedAgentIds] = useState(new Set())

  const trimmedCreateDisplayName = createDisplayName.trim()
  const trimmedJoinDisplayName = joinDisplayName.trim()
  const trimmedRoomName = roomName.trim()
  const trimmedRoomId = roomId.trim()
  const trimmedCreatePasscode = createPasscode.trim()
  const trimmedJoinPasscode = joinPasscode.trim()
  const selectedAgents = useMemo(
    () => availableAgents.filter((agent) => selectedAgentIds.has(agent.id)),
    [availableAgents, selectedAgentIds],
  )

  useEffect(() => {
    let isCurrent = true
    const loadAgents = async () => {
      try {
        const response = await getAgents()
        if (!isCurrent) {
          return
        }
        const enabledAgents = (response.agents ?? []).filter((agent) => agent.enabled !== false)
        setAvailableAgents(enabledAgents)
        setSelectedAgentIds(new Set(enabledAgents.map((agent) => agent.id)))
      } catch {
        // Keep the room entry flow available even when the roster cannot be loaded.
      }
    }
    void loadAgents()
    return () => {
      isCurrent = false
    }
  }, [])

  const handleAgentToggle = (agentId) => {
    setSelectedAgentIds((current) => {
      const next = new Set(current)
      if (next.has(agentId)) {
        next.delete(agentId)
      } else {
        next.add(agentId)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    setSelectedAgentIds(new Set(availableAgents.map((agent) => agent.id)))
  }

  const handleDeselectAll = () => {
    setSelectedAgentIds(new Set())
  }

  const handleCreateRoom = async (event) => {
    event.preventDefault()
    if (!trimmedCreateDisplayName) {
      return
    }

    await onCreateRoom({
      displayName: trimmedCreateDisplayName,
      roomName: trimmedRoomName,
      agentIds: Array.from(selectedAgentIds),
      passcode: trimmedCreatePasscode,
      dialogueMode,
    })
  }

  const handleJoinRoom = async (event) => {
    event.preventDefault()
    if (!trimmedJoinDisplayName || !trimmedRoomId) {
      return
    }

    await onJoinRoom({
      displayName: trimmedJoinDisplayName,
      roomId: trimmedRoomId,
      passcode: trimmedJoinPasscode,
    })
  }

  return (
    <main className="workbench workbench--entry">
      <header className="app-bar">
        <div className="brand-lockup">
          <span className="brand-mark">AR</span>
          <div>
            <strong>AgentRoom</strong>
            <span>人和 Agent 协作开会的工作台</span>
          </div>
        </div>
        <nav className="app-nav" aria-label="主导航">
          <span className="app-nav-item app-nav-item--active">会议入口</span>
          <button className="app-nav-item" type="button" onClick={onOpenAgentAdmin}>
            Agent 管理
          </button>
        </nav>
      </header>

      <section className="entry-hero">
        <div>
          <p className="eyebrow">协作会议</p>
          <h1>把角色 Agent 带进你的实时文本会议</h1>
          <p className="section-copy">
            创建房间、选择本次需要的 Agent，并把会议资料交给它们参考。讨论时用 `@角色名` 明确点名，让每个 Agent 在合适的时候参与。
          </p>
        </div>
        <div className="entry-summary" aria-label="能力概览">
          <span>{availableAgents.length} 个可用 Agent</span>
          <span>Markdown 知识文件</span>
          <span>实时会议</span>
        </div>
      </section>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}

      <div className="entry-grid">
        <form className="panel panel--primary-flow" onSubmit={handleCreateRoom}>
          <div className="panel-header panel-header--horizontal">
            <div>
              <p className="eyebrow eyebrow--subtle">推荐流程</p>
              <h2>创建会议室</h2>
              <p className="panel-copy">为新的讨论准备房间，并选择本次需要邀请的 Agent。</p>
            </div>
            <span className="panel-badge">主路径</span>
          </div>

          <div className="form-stack">
            <div className="field-group">
              <label htmlFor="create-display-name">你的显示名称</label>
              <input
                id="create-display-name"
                autoFocus
                type="text"
                value={createDisplayName}
                onChange={(event) => setCreateDisplayName(event.target.value)}
                placeholder="例如：小明"
                disabled={isSubmitting}
                maxLength={40}
              />
              {!trimmedCreateDisplayName ? <p className="field-hint field-hint--warning">请输入名称后创建房间。</p> : null}
            </div>

            <div className="field-group">
              <label htmlFor="room-name">会议名称</label>
              <input
                id="room-name"
                type="text"
                value={roomName}
                onChange={(event) => setRoomName(event.target.value)}
                placeholder="例如：需求评审会"
                disabled={isSubmitting}
                maxLength={60}
              />
              <p className="field-hint">留空时会使用默认会议名称。</p>
            </div>

            <div className="field-group">
              <label htmlFor="create-passcode">房间口令</label>
              <input
                id="create-passcode"
                type="password"
                value={createPasscode}
                onChange={(event) => setCreatePasscode(event.target.value)}
                placeholder="可选，用于限制加入房间"
                disabled={isSubmitting}
                maxLength={80}
              />
              <p className="field-hint">如果留空，这个房间不需要口令。</p>
            </div>

            <div className="field-group">
              <label htmlFor="dialogue-mode">Agent 对话模式</label>
              <select
                id="dialogue-mode"
                value={dialogueMode}
                onChange={(event) => setDialogueMode(event.target.value)}
                disabled={isSubmitting}
              >
                <option value="mention_fanout">点名单轮</option>
                <option value="guided_dialogue">引导多轮</option>
              </select>
              <p className="field-hint">点名单轮只让被 @ 的 Agent 各回复一次；引导多轮允许受限的 Agent 接力讨论。</p>
            </div>

            {availableAgents.length > 0 ? (
              <div className="agent-picker">
                <div className="agent-picker-header">
                  <div>
                    <label>选择本次 Agent</label>
                    <p>{selectedAgentIds.size === 0 ? '本次会议不邀请 Agent' : `已选择 ${selectedAgentIds.size}/${availableAgents.length} 个 Agent`}</p>
                  </div>
                  <div className="agent-select-actions">
                    <button className="button button--ghost button--compact" type="button" onClick={handleSelectAll} disabled={isSubmitting}>
                      全选
                    </button>
                    <button className="button button--ghost button--compact" type="button" onClick={handleDeselectAll} disabled={isSubmitting}>
                      清空
                    </button>
                  </div>
                </div>
                <div className="agent-chip-grid">
                  {availableAgents.map((agent) => {
                    const checked = selectedAgentIds.has(agent.id)
                    return (
                      <label key={agent.id} className={`agent-chip${checked ? ' agent-chip--selected' : ''}`}>
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => handleAgentToggle(agent.id)}
                          disabled={isSubmitting}
                        />
                        <span className="agent-chip-avatar">{agent.name.charAt(0).toUpperCase()}</span>
                        <span>
                          <strong>{agent.name}</strong>
                          <small>{agent.role}</small>
                        </span>
                      </label>
                    )
                  })}
                </div>
              </div>
            ) : null}
          </div>

          <div className="selected-agent-strip" aria-label="已选择 Agent">
            {selectedAgents.length === 0 ? (
              <span>未选择 Agent</span>
            ) : (
              selectedAgents.slice(0, 4).map((agent) => <span key={agent.id}>{agent.name}</span>)
            )}
            {selectedAgents.length > 4 ? <span>+{selectedAgents.length - 4}</span> : null}
          </div>

          <div className="button-row button-row--stack-end">
            <span className="helper-text">创建后会自动进入会议室。</span>
            <button className="button button--primary" type="submit" disabled={isSubmitting || !trimmedCreateDisplayName}>
              {isSubmitting ? '正在创建...' : '创建房间'}
            </button>
          </div>
        </form>

        <form className="panel panel--secondary-flow" onSubmit={handleJoinRoom}>
          <div className="panel-header panel-header--horizontal">
            <div>
              <p className="eyebrow eyebrow--subtle">已有房间</p>
              <h2>加入会议室</h2>
              <p className="panel-copy">输入房间 ID，以参会者身份进入已有讨论。</p>
            </div>
            <span className="panel-badge panel-badge--neutral">快捷加入</span>
          </div>

          <div className="form-stack">
            <div className="field-group">
              <label htmlFor="join-display-name">你的显示名称</label>
              <input
                id="join-display-name"
                type="text"
                value={joinDisplayName}
                onChange={(event) => setJoinDisplayName(event.target.value)}
                placeholder="例如：小明"
                disabled={isSubmitting}
                maxLength={40}
              />
            </div>

            <div className="field-group">
              <label htmlFor="room-id">房间 ID</label>
              <input
                id="room-id"
                type="text"
                value={roomId}
                onChange={(event) => setRoomId(event.target.value)}
                placeholder="room_123456"
                disabled={isSubmitting}
                maxLength={80}
              />
              <p className="field-hint">房间 ID 可从会议室右上角复制。</p>
            </div>

            <div className="field-group">
              <label htmlFor="join-passcode">房间口令</label>
              <input
                id="join-passcode"
                type="password"
                value={joinPasscode}
                onChange={(event) => setJoinPasscode(event.target.value)}
                placeholder="如果房间设置了口令，请输入"
                disabled={isSubmitting}
                maxLength={80}
              />
            </div>
          </div>

          <div className="button-row button-row--stack-end">
            <span className={`helper-text${trimmedJoinDisplayName && trimmedRoomId ? '' : ' helper-text--attention'}`}>
              {!trimmedJoinDisplayName ? '请先输入显示名称。' : trimmedRoomId ? '准备加入已有房间。' : '请输入房间 ID。'}
            </span>
            <button
              className="button button--secondary"
              type="submit"
              disabled={isSubmitting || !trimmedJoinDisplayName || !trimmedRoomId}
            >
              {isSubmitting ? '正在加入...' : '加入房间'}
            </button>
          </div>
        </form>
      </div>
    </main>
  )
}

export default JoinScreen
