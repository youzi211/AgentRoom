import { useCallback, useEffect, useRef, useState } from 'react'
import { Avatar, Button, Paper, Text, Textarea } from '@mantine/core'
import {
  buildMentionTargets,
  filterMentionTargets,
  findMentionQuery,
  replaceMentionQuery,
} from './messageComposerModel'

function MessageComposer({ agents = [], participants = [], currentParticipantName = '', disabled, onInsertMentionRef, onSend }) {
  const [content, setContent] = useState('')
  const [showAutocomplete, setShowAutocomplete] = useState(false)
  const [autocompleteFilter, setAutocompleteFilter] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [cursorPosition, setCursorPosition] = useState(0)
  const textareaRef = useRef(null)
  const autocompleteRef = useRef(null)
  const mentionTargets = buildMentionTargets({
    agents,
    participants,
    currentParticipantName,
  })
  const filteredMentionTargets = filterMentionTargets(mentionTargets, autocompleteFilter)
  const hasVisibleAutocomplete = showAutocomplete && filteredMentionTargets.length > 0

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
    if (hasVisibleAutocomplete) {
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        setSelectedIndex((prev) => Math.min(prev + 1, filteredMentionTargets.length - 1))
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
        return
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault()
        if (filteredMentionTargets[selectedIndex]) {
          insertMentionFromAutocomplete(filteredMentionTargets[selectedIndex].mention)
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

    const nextFilter = findMentionQuery(newValue, newCursorPosition)
    if (nextFilter !== null) {
      setAutocompleteFilter(nextFilter)
      setShowAutocomplete(true)
      setSelectedIndex(0)
      return
    }

    setShowAutocomplete(false)
  }

  const insertMentionFromAutocomplete = useCallback((mention) => {
    const nextState = replaceMentionQuery(content, cursorPosition, mention)
    setContent(nextState.content)
    setCursorPosition(nextState.cursorPosition)
    setShowAutocomplete(false)

    setTimeout(() => {
      if (textareaRef.current) {
        textareaRef.current.focus()
        textareaRef.current.setSelectionRange(nextState.cursorPosition, nextState.cursorPosition)
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
        <Textarea
          ref={textareaRef}
          id="message-input"
          className="composer-input"
          value={content}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="输入消息，使用 @ 提及 Agent 参与讨论。"
          rows={3}
          disabled={disabled}
          autosize={false}
        />
        {showAutocomplete && filteredMentionTargets.length > 0 && (
          <Paper className="autocomplete-popup" ref={autocompleteRef} withBorder radius="md" shadow="md">
            <ul className="autocomplete-list">
              {filteredMentionTargets.map((target, index) => (
                <li
                  key={`${target.kind}:${target.id}`}
                  className={`autocomplete-item${index === selectedIndex ? ' autocomplete-item--selected' : ''}`}
                  onClick={() => insertMentionFromAutocomplete(target.mention)}
                  onMouseEnter={() => setSelectedIndex(index)}
                >
                  <Avatar className="autocomplete-avatar" radius="sm" color="teal">{target.name.charAt(0).toUpperCase()}</Avatar>
                  <div className="autocomplete-info">
                    <Text className="autocomplete-name">{target.name}</Text>
                    <Text className="autocomplete-role">{target.role}</Text>
                  </div>
                  <Text component="span" className="autocomplete-mention">{target.mention}</Text>
                </li>
              ))}
            </ul>
          </Paper>
        )}
      </div>
      <div className="composer-actions">
        <Text component="span" className={`composer-status${disabled ? ' composer-status--disabled' : ''}`}>
          {disabled ? '连接不可用，正在等待恢复' : 'Enter 发送，Shift + Enter 换行'}
        </Text>
        <Button color="teal" type="submit" disabled={disabled || !content.trim()}>
          发送
        </Button>
      </div>
    </form>
  )
}

export default MessageComposer
