function AgentRoster({ agents, thinkingAgents = [], onInsertMention }) {
  const thinkingIDs = new Set(thinkingAgents.map((agent) => agent.id))

  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>可用 Agent</h2>
        <span className="sidebar-count">{agents.length}</span>
      </div>

      {agents.length === 0 ? (
        <p className="sidebar-empty">暂无可用 Agent，可在管理页启用后再创建房间。</p>
      ) : (
        <ul className="sidebar-list">
          {agents.map((agent) => (
            <li className="sidebar-list-item sidebar-list-item--stacked" key={agent.id}>
              <div className="agent-row">
                <div className="agent-identity">
                  <div className="sidebar-avatar agent-avatar">{agent.name.charAt(0).toUpperCase()}</div>
                  <div className="sidebar-copy">
                    <p className="sidebar-primary">{agent.name}</p>
                    <p className="sidebar-secondary">{thinkingIDs.has(agent.id) ? '正在思考' : agent.role}</p>
                  </div>
                </div>
                <button className="mention-button" type="button" onClick={() => onInsertMention(agent.mention)}>
                  {agent.mention}
                </button>
              </div>
              {agent.description ? <p className="agent-description">{agent.description}</p> : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export default AgentRoster
