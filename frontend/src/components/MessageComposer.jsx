import { useState } from 'react'

function MessageComposer({ disabled, onInsertMentionRef, onSend }) {
  const [content, setContent] = useState('')

  const handleSubmit = async (event) => {
    event.preventDefault()
    const nextContent = content.trim()
    if (!nextContent) {
      return
    }

    await onSend(nextContent)
    setContent('')
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
      <label className="composer-label" htmlFor="message-input">
        Message
      </label>
      <textarea
        id="message-input"
        className="composer-input"
        value={content}
        onChange={(event) => setContent(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Type a message. Use @AgentName to ask an agent to respond."
        rows={3}
        disabled={disabled}
      />
      <div className="composer-actions">
        <p className="helper-text muted-text">Press Enter to send. Shift+Enter adds a new line.</p>
        <button className="primary-button" type="submit" disabled={disabled || !content.trim()}>
          Send
        </button>
      </div>
    </form>
  )
}

export default MessageComposer
