import { useEffect, useState } from 'react'
import { getAgents } from '../api/roomClient'

function JoinScreen({ errorMessage, isSubmitting, onCreateRoom, onJoinRoom, onOpenAgentAdmin }) {
  const [createDisplayName, setCreateDisplayName] = useState('')
  const [joinDisplayName, setJoinDisplayName] = useState('')
  const [roomName, setRoomName] = useState('')
  const [roomId, setRoomId] = useState('')
  const [availableAgents, setAvailableAgents] = useState([])
  const [selectedAgentIds, setSelectedAgentIds] = useState(new Set())

  const trimmedCreateDisplayName = createDisplayName.trim()
  const trimmedJoinDisplayName = joinDisplayName.trim()
  const trimmedRoomName = roomName.trim()
  const trimmedRoomId = roomId.trim()
  const createHint = trimmedCreateDisplayName ? '创建成功后会自动进入会议室。' : '请输入显示名称后创建房间。'
  const joinHint = !trimmedJoinDisplayName
    ? '请输入显示名称后加入房间。'
    : trimmedRoomId
      ? '加入后会加载房间已有消息。'
      : '请粘贴房间 ID 后加入房间。'

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
        // Agent list is optional; creation still works with all agents as fallback
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
    })
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <div className="topbar">
          <div>
            <p className="eyebrow">AgentRoom</p>
            <h1>人与智能体协作的轻量会议工作台</h1>
            <p className="section-copy">
              创建一个文本会议室，让团队成员和预定义角色 Agent 在同一条上下文里协作。Agent 默认保持安静，被明确 @ 时才参与讨论。
            </p>
          </div>
          <button className="button button--ghost" type="button" onClick={onOpenAgentAdmin}>
            管理 Agent
          </button>
        </div>

        <div className="join-grid">
          <form className="panel panel--form panel--accent" onSubmit={handleCreateRoom}>
            <div className="panel-header">
              <div className="panel-title-row">
                <h2>创建房间</h2>
                <span className="panel-badge">新会议</span>
              </div>
              <p className="panel-copy">输入你的显示名称即可创建房间，随后把房间 ID 发给其他参与者。</p>
            </div>

            <div className="field-group">
              <label htmlFor="create-display-name">显示名称</label>
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
              <p className="field-hint">这个名称会显示在会议消息和成员列表中。</p>
            </div>

            <div className="field-group">
              <label htmlFor="room-name">房间名称</label>
              <input
                id="room-name"
                type="text"
                value={roomName}
                onChange={(event) => setRoomName(event.target.value)}
                placeholder="例如：产品评审会"
                disabled={isSubmitting}
                maxLength={60}
              />
              <p className="field-hint">可选。留空时后端会自动生成默认房间名。</p>
            </div>

            {availableAgents.length > 0 ? (
              <div className="field-group">
                <div className="agent-select-header">
                  <label>参会 Agent</label>
                  <div className="agent-select-actions">
                    <button className="button button--ghost button--compact" type="button" onClick={handleSelectAll} disabled={isSubmitting}>
                      全选
                    </button>
                    <button className="button button--ghost button--compact" type="button" onClick={handleDeselectAll} disabled={isSubmitting}>
                      清空
                    </button>
                  </div>
                </div>
                <div className="agent-checkbox-grid">
                  {availableAgents.map((agent) => (
                    <label key={agent.id} className="agent-checkbox-item">
                      <input
                        type="checkbox"
                        checked={selectedAgentIds.has(agent.id)}
                        onChange={() => handleAgentToggle(agent.id)}
                        disabled={isSubmitting}
                      />
                      <span className="agent-checkbox-name">{agent.name}</span>
                      <span className="agent-checkbox-role">{agent.role}</span>
                    </label>
                  ))}
                </div>
                <p className="field-hint">
                  {selectedAgentIds.size === 0
                    ? '未选择 Agent，本次会议将只包含人类成员。'
                    : `已选 ${selectedAgentIds.size} 个 Agent 加入房间。`}
                </p>
              </div>
            ) : null}

            <div className="button-row button-row--stack-end">
              <span className={`helper-text${trimmedCreateDisplayName ? '' : ' helper-text--attention'}`}>{createHint}</span>
              <button className="button button--primary" type="submit" disabled={isSubmitting || !trimmedCreateDisplayName}>
                {isSubmitting ? '创建中...' : '创建房间'}
              </button>
            </div>
          </form>

          <form className="panel panel--form" onSubmit={handleJoinRoom}>
            <div className="panel-header">
              <div className="panel-title-row">
                <h2>加入房间</h2>
                <span className="panel-badge panel-badge--neutral">继续讨论</span>
              </div>
              <p className="panel-copy">粘贴已有房间 ID，使用你的显示名称加入实时会议。</p>
            </div>

            <div className="field-group">
              <label htmlFor="join-display-name">显示名称</label>
              <input
                id="join-display-name"
                type="text"
                value={joinDisplayName}
                onChange={(event) => setJoinDisplayName(event.target.value)}
                placeholder="例如：小红"
                disabled={isSubmitting}
                maxLength={40}
              />
              <p className="field-hint">建议使用团队成员容易识别的名称。</p>
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
              <p className="field-hint">房间 ID 区分大小写，请完整粘贴。</p>
            </div>

            <div className="button-row button-row--stack-end">
              <span className={`helper-text${trimmedJoinDisplayName && trimmedRoomId ? '' : ' helper-text--attention'}`}>{joinHint}</span>
              <button
                className="button button--secondary"
                type="submit"
                disabled={isSubmitting || !trimmedJoinDisplayName || !trimmedRoomId}
              >
                {isSubmitting ? '加入中...' : '加入房间'}
              </button>
            </div>
          </form>
        </div>

        {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      </section>
    </main>
  )
}

export default JoinScreen
