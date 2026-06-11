function AgentRoster({ agents, onInsertMention }) {
  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>可用 Agent</h2>
        <span className="sidebar-count">{agents.length}</span>
      </div>

      {agents.length === 0 ? (
        <p className="empty-state sidebar-empty">暂无启用的 Agent。</p>
      ) : (
        <ul className="sidebar-list">
          {agents.map((agent) => (
            <li className="sidebar-list-item sidebar-list-item--stacked" key={agent.id}>
              <div className="agent-row">
                <div className="agent-identity">
                  <div className="sidebar-avatar agent-avatar">{agent.name.charAt(0).toUpperCase()}</div>
                  <div className="sidebar-copy">
                    <p className="sidebar-primary">{agent.name}</p>
                    <p className="sidebar-secondary">{agent.role}</p>
                  </div>
                </div>
                <button className="mention-button" type="button" onClick={() => onInsertMention(agent.mention)}>
                  {agent.mention}
                </button>
              </div>
              <p className="agent-description">{agent.description}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export default AgentRoster
