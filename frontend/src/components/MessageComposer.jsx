import { useState } from 'react'

function MessageComposer({ disabled, onInsertMentionRef, onSend }) {
  const [content, setContent] = useState('')

  const handleSubmit = async (event) => {
    event.preventDefault()
    const nextContent = content.trim()
    if (!nextContent) {
      return
    }

    const didSend = await onSend(nextContent)
    if (didSend) {
      setContent('')
    }
  }

  const handleKeyDown = (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      void handleSubmit(event)
    }
  }

  const insertMention = (mention) => {
    setContent((current) => {
      if (!current.trim()) {
        return `${mention} `
      }
      if (current.endsWith(' ') || current.endsWith('\n')) {
        return `${current}${mention} `
      }
      return `${current} ${mention} `
    })
  }

  onInsertMentionRef.current = insertMention

  return (
    <form className="composer" onSubmit={handleSubmit}>
      <div className="composer-toolbar">
        <label className="composer-label" htmlFor="message-input">
          发送消息
        </label>
        <span className={`composer-status${disabled ? ' composer-status--disabled' : ''}`}>{disabled ? '等待重新连接' : '可以发送'}</span>
      </div>
      <textarea
        id="message-input"
        className="composer-input"
        value={content}
        onChange={(event) => setContent(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="输入消息，或点击左侧 Agent 的 @ 按钮插入提及。"
        rows={3}
        disabled={disabled}
      />
      <div className="composer-actions">
        <p className="helper-text muted-text">Enter 发送，Shift+Enter 换行。</p>
        <button className="button button--primary" type="submit" disabled={disabled || !content.trim()}>
          发送
        </button>
      </div>
    </form>
  )
}

export default MessageComposer
