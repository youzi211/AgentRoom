function ParticipantList({ participants }) {
  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>在线成员</h2>
        <span className="sidebar-count">{participants.length}</span>
      </div>

      {participants.length === 0 ? (
        <p className="empty-state sidebar-empty">暂无成员在线。</p>
      ) : (
        <ul className="sidebar-list">
          {participants.map((participant) => (
            <li className="sidebar-list-item" key={participant.id}>
              <div className="participant-identity">
                <div className="sidebar-avatar participant-avatar">{participant.name.charAt(0).toUpperCase()}</div>
                <div className="sidebar-copy">
                  <p className="sidebar-primary">{participant.name}</p>
                  <p className="sidebar-secondary">{formatJoinedAt(participant.joinedAt)} 加入</p>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function formatJoinedAt(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '刚刚'
  }

  return new Intl.DateTimeFormat('zh-CN', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export default ParticipantList
