import { forwardRef } from 'react'
import { Avatar, Badge, Button, Paper, Text, Title } from '@mantine/core'

const MessageList = forwardRef(function MessageList({ currentParticipantName, messages, onDownloadArtifact, thinkingAgents = [] }, ref) {
  if (messages.length === 0 && thinkingAgents.length === 0) {
    return (
      <Paper component="section" className="message-panel message-panel--empty" withBorder radius="md" shadow="none">
        <div className="empty-state empty-state--conversation">
          <Text className="eyebrow eyebrow--subtle">对话</Text>
          <Title order={2} className="message-empty-title">开始一次协作会议</Title>
          <Text className="muted-text">@产品经理 讨论需求，或上传会议文件后邀请 Agent 参与。</Text>
        </div>
      </Paper>
    )
  }

  return (
    <Paper component="section" className="message-panel" aria-label="消息列表" withBorder radius="md" shadow="none">
      <ul className="message-list" ref={ref}>
        {messages.map((message) => {
          const messageRole = roleForMessage(message, currentParticipantName)
          const knowledgeSources = messageRole === 'agent' ? formatKnowledgeSources(message.knowledgeSources) : []

          return (
            <li className={`message-row message-row--${messageRole}`} key={message.id}>
              {messageRole !== 'system' ? (
                <Avatar className={`message-avatar message-avatar--${messageRole}`} radius="sm" color={messageRole === 'agent' ? 'teal' : 'gray'} aria-hidden="true">
                  {avatarTextForMessage(message, messageRole)}
                </Avatar>
              ) : null}
              <Paper component="article" className={`message-card message-card--${messageRole}`} withBorder radius="md" shadow="none">
                <div className="message-meta">
                  <div className="message-author-group">
                    <Text component="span" className="message-author">{message.senderName}</Text>
                    <Badge className={`message-badge message-badge--${messageRole}`} color={messageRole === 'agent' ? 'teal' : 'gray'} variant="light">
                      {labelForMessageRole(messageRole)}
                    </Badge>
                  </div>
                  <time className="message-time" dateTime={message.createdAt}>
                    {formatMessageTime(message.createdAt)}
                  </time>
                </div>
                <Text className="message-content">{message.content}</Text>
                {message.artifacts?.length > 0 ? (
                  <div className="message-artifacts" aria-label="报告文件">
                    {message.artifacts.map((artifact) => (
                      <Button
                        className="message-artifact-button"
                        key={artifact.id}
                        type="button"
                        variant="light"
                        color="teal"
                        size="xs"
                        onClick={() => onDownloadArtifact?.(message, artifact)}
                      >
                        <span className="message-artifact-icon" aria-hidden="true">MD</span>
                        <span>{artifact.title || artifact.fileName || '下载报告'}</span>
                        <span className="message-artifact-action">下载报告</span>
                      </Button>
                    ))}
                  </div>
                ) : null}
                {knowledgeSources.length > 0 ? (
                  <div className="message-sources" aria-label="知识来源">
                    <Text component="span" className="message-sources-label">参考：</Text>
                    {knowledgeSources.map((source) => (
                      <Badge className="message-source-chip" key={source} color="gray" variant="light">
                        {source}
                      </Badge>
                    ))}
                  </div>
                ) : null}
              </Paper>
            </li>
          )
        })}
        {thinkingAgents.map((agent) => (
          <li className="message-row message-row--agent" key={`thinking:${agent.id}`}>
            <Avatar className="message-avatar message-avatar--agent" radius="sm" color="teal" aria-hidden="true">
              {agent.name.charAt(0).toUpperCase()}
            </Avatar>
            <Paper component="article" className="message-card message-card--agent message-card--thinking" withBorder radius="md" shadow="none">
              <div className="message-meta">
                <div className="message-author-group">
                  <Text component="span" className="message-author">{agent.name}</Text>
                  <Badge className="message-badge message-badge--agent" color="teal" variant="light">Agent</Badge>
                </div>
              </div>
              <Text className="message-content">
                <span className="typing-dot" />
                <span className="typing-dot" />
                <span className="typing-dot" />
                正在思考...
              </Text>
            </Paper>
          </li>
        ))}
      </ul>
    </Paper>
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

function formatKnowledgeSources(sources = []) {
  const names = []
  const seen = new Set()

  for (const source of sources) {
    const name = source?.documentName || source?.documentId
    if (!name || seen.has(name)) {
      continue
    }
    seen.add(name)
    names.push(name)
  }

  return names
}

export default MessageList
