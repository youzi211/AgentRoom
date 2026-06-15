import { useCallback, useEffect, useRef, useState } from 'react'

function MessageComposer({ agents = [], disabled, onInsertMentionRef, onSend }) {
  const [content, setContent] = useState('')
  const [showAutocomplete, setShowAutocomplete] = useState(false)
  const [autocompleteFilter, setAutocompleteFilter] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [cursorPosition, setCursorPosition] = useState(0)
  const textareaRef = useRef(null)
  const autocompleteRef = useRef(null)

  const filteredAgents = agents.filter((agent) =>
    agent.name.toLowerCase().includes(autocompleteFilter.toLowerCase()) ||
    agent.mention.toLowerCase().includes(autocompleteFilter.toLowerCase())
  )

  const handleSubmit = async (event) => {
    event.preventDefault()
    const nextContent = content.trim()
    if (!nextContent) {
      return
    }

    const didSend = await onSend(nextContent)
    if (didSend) {
      setContent('')
      setShowAutocomplete(false)
    }
  }

  const handleKeyDown = (event) => {
    if (showAutocomplete) {
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        setSelectedIndex((prev) => Math.min(prev + 1, filteredAgents.length - 1))
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
        return
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault()
        if (filteredAgents[selectedIndex]) {
          insertMentionFromAutocomplete(filteredAgents[selectedIndex].mention)
        }
        return
      }
      if (event.key === 'Escape') {
        event.preventDefault()
        setShowAutocomplete(false)
        return
      }
    }

    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      void handleSubmit(event)
    }
  }

  const handleChange = (event) => {
    const newValue = event.target.value
    const newCursorPosition = event.target.selectionStart
    setContent(newValue)
    setCursorPosition(newCursorPosition)

    // Check for @ trigger
    const textBeforeCursor = newValue.substring(0, newCursorPosition)
    const atIndex = textBeforeCursor.lastIndexOf('@')

    if (atIndex !== -1) {
      const textAfterAt = textBeforeCursor.substring(atIndex + 1)
      // Only show autocomplete if @ is at start or preceded by whitespace
      const charBeforeAt = atIndex > 0 ? textBeforeCursor[atIndex - 1] : ' '
      if (charBeforeAt === ' ' || charBeforeAt === '\n' || atIndex === 0) {
        // Check if there's no space between @ and cursor
        if (!textAfterAt.includes(' ') && !textAfterAt.includes('\n')) {
          setAutocompleteFilter(textAfterAt)
          setShowAutocomplete(true)
          setSelectedIndex(0)
          return
        }
      }
    }
    setShowAutocomplete(false)
  }

  const insertMentionFromAutocomplete = useCallback((mention) => {
    const textBeforeCursor = content.substring(0, cursorPosition)
    const atIndex = textBeforeCursor.lastIndexOf('@')
    const textAfterCursor = content.substring(cursorPosition)

    const newContent = `${textBeforeCursor.substring(0, atIndex)}${mention} ${textAfterCursor}`
    setContent(newContent)
    setShowAutocomplete(false)

    // Focus and set cursor position after mention
    setTimeout(() => {
      if (textareaRef.current) {
        const newCursorPos = atIndex + mention.length + 1
        textareaRef.current.focus()
        textareaRef.current.setSelectionRange(newCursorPos, newCursorPos)
      }
    }, 0)
  }, [content, cursorPosition])

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

  // Close autocomplete on click outside
  useEffect(() => {
    const handleClickOutside = (event) => {
      if (autocompleteRef.current && !autocompleteRef.current.contains(event.target)) {
        setShowAutocomplete(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <form className="composer" onSubmit={handleSubmit}>
      <div className="composer-input-wrapper">
        <textarea
          ref={textareaRef}
          id="message-input"
          className="composer-input"
          value={content}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="输入消息，使用 @ 提及 Agent 参与讨论。"
          rows={3}
          disabled={disabled}
        />
        {showAutocomplete && filteredAgents.length > 0 && (
          <div className="autocomplete-popup" ref={autocompleteRef}>
            <ul className="autocomplete-list">
              {filteredAgents.map((agent, index) => (
                <li
                  key={agent.id}
                  className={`autocomplete-item${index === selectedIndex ? ' autocomplete-item--selected' : ''}`}
                  onClick={() => insertMentionFromAutocomplete(agent.mention)}
                  onMouseEnter={() => setSelectedIndex(index)}
                >
                  <span className="autocomplete-avatar">{agent.name.charAt(0).toUpperCase()}</span>
                  <div className="autocomplete-info">
                    <span className="autocomplete-name">{agent.name}</span>
                    <span className="autocomplete-role">{agent.role}</span>
                  </div>
                  <span className="autocomplete-mention">{agent.mention}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
      <div className="composer-actions">
        <span className={`composer-status${disabled ? ' composer-status--disabled' : ''}`}>
          {disabled ? '连接不可用，正在等待恢复' : 'Enter 发送，Shift + Enter 换行'}
        </span>
        <button className="button button--primary" type="submit" disabled={disabled || !content.trim()}>
          发送
        </button>
      </div>
    </form>
  )
}

export default MessageComposer
