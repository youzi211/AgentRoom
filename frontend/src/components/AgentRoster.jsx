function AgentRoster({ agents, onInsertMention }) {
  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>Agents</h2>
        <span className="sidebar-count">{agents.length}</span>
      </div>

      {agents.length === 0 ? (
        <p className="empty-state sidebar-empty">No agents are available.</p>
      ) : (
        <ul className="sidebar-list">
          {agents.map((agent) => (
            <li className="sidebar-list-item sidebar-list-item--stacked" key={agent.id}>
              <div className="agent-row">
                <div>
                  <p className="sidebar-primary">{agent.name}</p>
                  <p className="sidebar-secondary">{agent.role}</p>
                </div>
                <button
                  className="mention-button"
                  type="button"
                  onClick={() => onInsertMention(agent.mention)}
                >
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
