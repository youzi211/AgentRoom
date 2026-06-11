function MessageList({ messages }) {
  if (messages.length === 0) {
    return (
      <section className="message-panel message-panel--empty">
        <div className="empty-state">
          <h2>No messages yet</h2>
          <p className="muted-text">Start the discussion or mention an agent to ask for input.</p>
        </div>
      </section>
    )
  }

  return (
    <section className="message-panel" aria-label="Messages">
      <ul className="message-list">
        {messages.map((message) => (
          <li className={`message-card message-card--${message.senderType}`} key={message.id}>
            <div className="message-meta">
              <div className="message-author-group">
                <span className="message-author">{message.senderName}</span>
                <span className={`message-badge message-badge--${message.senderType}`}>
                  {labelForSenderType(message.senderType)}
                </span>
              </div>
              <time className="message-time" dateTime={message.createdAt}>
                {formatMessageTime(message.createdAt)}
              </time>
            </div>
            <p className="message-content">{message.content}</p>
          </li>
        ))}
      </ul>
    </section>
  )
}

function labelForSenderType(senderType) {
  switch (senderType) {
    case 'agent':
      return 'Agent'
    case 'system':
      return 'System'
    default:
      return 'Human'
  }
}

function formatMessageTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  return new Intl.DateTimeFormat(undefined, {
    hour: 'numeric',
    minute: '2-digit',
  }).format(date)
}

export default MessageList
