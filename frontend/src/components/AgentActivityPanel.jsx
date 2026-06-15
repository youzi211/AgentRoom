import { descriptionForActivity, labelForActivityStatus } from './agentActivity'

function AgentActivityPanel({ activities = [], errorMessage = '', isLoading = false }) {
  const visibleActivities = activities.slice(0, 8)

  return (
    <section className="sidebar-section agent-activity-panel">
      <div className="sidebar-header">
        <h2>Agent 活动</h2>
        <span className="sidebar-count">{activities.length}</span>
      </div>
      {errorMessage ? <p className="agent-activity-notice agent-activity-notice--error">{errorMessage}</p> : null}
      {isLoading ? <p className="agent-activity-notice">正在加载 Agent 活动...</p> : null}
      {!isLoading && visibleActivities.length === 0 ? <p className="agent-activity-notice">暂无 Agent 活动</p> : null}
      {visibleActivities.length > 0 ? (
        <div className="agent-activity-list">
          {visibleActivities.map((activity) => (
            <article
              className={`agent-activity-item${activity.status === 'running' ? ' agent-activity-item--running' : ''}`}
              key={`${activity.kind}:${activity.id}`}
            >
              <div className="agent-activity-main">
                <strong>{titleForActivity(activity)}</strong>
                <span>{descriptionForActivity(activity)}</span>
              </div>
              <span className="agent-activity-status">{labelForActivityStatus(activity.status)}</span>
            </article>
          ))}
        </div>
      ) : null}
    </section>
  )
}

function titleForActivity(activity) {
  if (activity.kind === 'dialogue_run') {
    return '对话链路'
  }
  return activity.agentName || activity.agentID || 'Agent'
}

export default AgentActivityPanel
