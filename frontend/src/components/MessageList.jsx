import { forwardRef } from 'react'

const MessageList = forwardRef(function MessageList({ currentParticipantName, messages, thinkingAgents = [] }, ref) {
  if (messages.length === 0 && thinkingAgents.length === 0) {
    return (
      <section className="message-panel message-panel--empty">
        <div className="empty-state empty-state--conversation">
          <p className="eyebrow eyebrow--subtle">对话</p>
          <h2 className="message-empty-title">开始一次协作会议</h2>
          <p className="muted-text">@产品经理 讨论需求，或上传会议文件后邀请 Agent 参与。</p>
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
        {thinkingAgents.map((agent) => (
          <li className="message-row message-row--agent" key={`thinking:${agent.id}`}>
            <span className="message-avatar message-avatar--agent" aria-hidden="true">
              {agent.name.charAt(0).toUpperCase()}
            </span>
            <article className="message-card message-card--agent message-card--thinking">
              <div className="message-meta">
                <div className="message-author-group">
                  <span className="message-author">{agent.name}</span>
                  <span className="message-badge message-badge--agent">Agent</span>
                </div>
              </div>
              <p className="message-content">
                <span className="typing-dot" />
                <span className="typing-dot" />
                <span className="typing-dot" />
                正在思考...
              </p>
            </article>
          </li>
        ))}
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
