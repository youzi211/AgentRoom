import { Avatar, Badge, Button, Paper, Text } from '@mantine/core'

function AgentRoster({ agents, thinkingAgents = [], onInsertMention }) {
  const thinkingIDs = new Set(thinkingAgents.map((agent) => agent.id))

  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>可用 Agent</h2>
        <Badge className="sidebar-count" color="teal" variant="light">{agents.length}</Badge>
      </div>

      {agents.length === 0 ? (
        <Text className="sidebar-empty">暂无可用 Agent，可在管理页启用后再创建房间。</Text>
      ) : (
        <ul className="sidebar-list">
          {agents.map((agent) => (
            <Paper component="li" className="sidebar-list-item sidebar-list-item--stacked" key={agent.id} withBorder radius="md" shadow="none">
              <div className="agent-row">
                <div className="agent-identity">
                  <Avatar className="sidebar-avatar agent-avatar" radius="sm" color="teal">{agent.name.charAt(0).toUpperCase()}</Avatar>
                  <div className="sidebar-copy">
                    <Text className="sidebar-primary">{agent.name}</Text>
                    <Text className="sidebar-secondary">{thinkingIDs.has(agent.id) ? '正在思考' : agent.role}</Text>
                  </div>
                </div>
                <Button className="mention-button" variant="light" color="teal" size="xs" type="button" onClick={() => onInsertMention(agent.mention)}>
                  {agent.mention}
                </Button>
              </div>
              {agent.description ? <Text className="agent-description">{agent.description}</Text> : null}
            </Paper>
          ))}
        </ul>
      )}
    </section>
  )
}

export default AgentRoster
