function MessageList({ messages }) {
  if (messages.length === 0) {
    return (
      <section className="message-panel message-panel--empty">
        <div className="empty-state">
          <p className="eyebrow eyebrow--subtle">对话</p>
          <h2 className="message-empty-title">暂无消息</h2>
          <p className="muted-text">开始对话，或 @ 一个 Agent 来获取帮助。</p>
        </div>
      </section>
    )
  }

  return (
    <section className="message-panel" aria-label="消息列表">
      <ul className="message-list">
        {messages.map((message) => (
          <li className={`message-card message-card--${message.senderType}`} key={message.id}>
            <div className="message-meta">
              <div className="message-author-group">
                <span className="message-author">{message.senderName}</span>
                <span className={`message-badge message-badge--${message.senderType}`}>{labelForSenderType(message.senderType)}</span>
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
      return '系统'
    default:
      return '用户'
  }
}

function formatMessageTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  return new Intl.DateTimeFormat('zh-CN', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export default MessageList
