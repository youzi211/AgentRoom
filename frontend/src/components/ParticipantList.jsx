function ParticipantList({ participants }) {
  return (
    <section className="sidebar-section">
      <div className="sidebar-header">
        <h2>Participants</h2>
        <span className="sidebar-count">{participants.length}</span>
      </div>

      {participants.length === 0 ? (
        <p className="empty-state sidebar-empty">No one is connected yet.</p>
      ) : (
        <ul className="sidebar-list">
          {participants.map((participant) => (
            <li className="sidebar-list-item" key={participant.id}>
              <div>
                <p className="sidebar-primary">{participant.name}</p>
                <p className="sidebar-secondary">Joined {formatJoinedAt(participant.joinedAt)}</p>
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
    return 'just now'
  }

  return new Intl.DateTimeFormat(undefined, {
    hour: 'numeric',
    minute: '2-digit',
  }).format(date)
}

export default ParticipantList
