import { forwardRef } from 'react'

const MessageList = forwardRef(function MessageList({ currentParticipantName, messages }, ref) {
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
      <ul className="message-list" ref={ref}>
        {messages.map((message) => {
          const messageRole = roleForMessage(message, currentParticipantName)

          return (
            <li className={`message-row message-row--${messageRole}`} key={message.id}>
              {messageRole !== 'system' ? (
                <span className={`message-avatar message-avatar--${messageRole}`} aria-hidden="true">
                  {avatarTextForMessage(message, messageRole)}
                </span>
              ) : null}
              <article className={`message-card message-card--${messageRole}`}>
                <div className="message-meta">
                  <div className="message-author-group">
                    <span className="message-author">{message.senderName}</span>
                    <span className={`message-badge message-badge--${messageRole}`}>{labelForMessageRole(messageRole)}</span>
                  </div>
                  <time className="message-time" dateTime={message.createdAt}>
                    {formatMessageTime(message.createdAt)}
                  </time>
                </div>
                <p className="message-content">{message.content}</p>
              </article>
            </li>
          )
        })}
      </ul>
    </section>
  )
})

function roleForMessage(message, currentParticipantName) {
  if (message.senderType === 'agent') {
    return 'agent'
  }
  if (message.senderType === 'system') {
    return 'system'
  }
  if (message.senderType === 'human' && message.senderName === currentParticipantName) {
    return 'own'
  }
  return 'other'
}

function labelForMessageRole(messageRole) {
  switch (messageRole) {
    case 'own':
      return '我'
    case 'agent':
      return 'Agent'
    case 'system':
      return '系统'
    default:
      return '成员'
  }
}

function avatarTextForMessage(message, messageRole) {
  if (messageRole === 'own') {
    return '我'
  }
  return message.senderName?.trim().charAt(0).toUpperCase() || '?'
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
