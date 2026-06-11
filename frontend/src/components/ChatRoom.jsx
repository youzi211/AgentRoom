import { useEffect, useRef, useState } from 'react'
import { createRoomSocket, getMessages, getRoom } from '../api/roomClient'
import AgentRoster from './AgentRoster'
import MessageComposer from './MessageComposer'
import MessageList from './MessageList'
import ParticipantList from './ParticipantList'
import '../chat-room.css'

const ROOM_SNAPSHOT_EVENT = 'room_snapshot'
const MESSAGE_EVENT = 'message'
const PARTICIPANT_JOINED_EVENT = 'participant_joined'
const PARTICIPANT_LEFT_EVENT = 'participant_left'
const ERROR_EVENT = 'error'

export default function ChatRoom({ initialRoom, participantName, roomId, onLeaveRoom }) {
  const [room, setRoom] = useState(initialRoom)
  const [participants, setParticipants] = useState([])
  const [agents, setAgents] = useState([])
  const [messages, setMessages] = useState([])
  const [connectionState, setConnectionState] = useState('connecting')
  const [errorMessage, setErrorMessage] = useState('')
  const socketRef = useRef(null)
  const insertMentionRef = useRef(() => {})

  useEffect(() => {
    let isCurrent = true

    const connectToRoom = async () => {
      setConnectionState('connecting')

      const [roomResult, messagesResult] = await Promise.allSettled([getRoom(roomId), getMessages(roomId)])
      if (!isCurrent) {
        return
      }

      const loadErrors = []

      if (roomResult.status === 'fulfilled') {
        setRoom(roomResult.value.room)
        setParticipants(roomResult.value.participants ?? [])
        setAgents(roomResult.value.agents ?? [])
      } else {
        loadErrors.push(roomResult.reason?.message || 'Failed to load room details.')
      }

      if (messagesResult.status === 'fulfilled') {
        setMessages(messagesResult.value.messages ?? [])
      } else {
        loadErrors.push(messagesResult.reason?.message || 'Failed to load messages.')
      }

      if (loadErrors.length > 0) {
        setErrorMessage(loadErrors.join(' '))
      }

      const socket = createRoomSocket(roomId, participantName)
      socketRef.current = socket

      socket.addEventListener('open', () => {
        if (!isCurrent) {
          return
        }

        setConnectionState('connected')
        setErrorMessage('')
      })

      socket.addEventListener('message', (event) => {
        if (!isCurrent) {
          return
        }

        try {
          const payload = JSON.parse(event.data)
          handleServerEvent(payload)
        } catch {
          setErrorMessage('Received an unreadable server event.')
        }
      })

      socket.addEventListener('error', () => {
        if (!isCurrent) {
          return
        }

        setErrorMessage((current) => current || 'The live room connection hit an error.')
      })

      socket.addEventListener('close', () => {
        if (!isCurrent) {
          return
        }

        socketRef.current = null
        setConnectionState('disconnected')
      })
    }

    const handleServerEvent = (event) => {
      switch (event.type) {
        case ROOM_SNAPSHOT_EVENT:
          if (event.room) {
            setRoom(event.room)
          }
          setParticipants(event.participants ?? [])
          setAgents(event.agents ?? [])
          setMessages(event.messages ?? [])
          return
        case PARTICIPANT_JOINED_EVENT:
          if (event.participant) {
            setParticipants((current) => upsertById(current, event.participant))
          }
          return
        case PARTICIPANT_LEFT_EVENT:
          if (event.participantID) {
            setParticipants((current) => current.filter((participant) => participant.id !== event.participantID))
          }
          return
        case MESSAGE_EVENT:
          if (event.message) {
            setMessages((current) => upsertById(current, event.message))
          }
          return
        case ERROR_EVENT:
          setErrorMessage(event.error || 'The room reported an error.')
          return
        default:
          return
      }
    }

    void connectToRoom()

    return () => {
      isCurrent = false

      const socket = socketRef.current
      socketRef.current = null

      if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        socket.close()
      }
    }
  }, [participantName, roomId])

  const handleSendMessage = async (content) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setErrorMessage('Reconnect to the room before sending another message.')
      return
    }

    socket.send(
      JSON.stringify({
        type: MESSAGE_EVENT,
        content,
      }),
    )
    setErrorMessage('')
  }

  const handleInsertMention = (mention) => {
    insertMentionRef.current?.(mention)
  }

  return (
    <main className="app-shell">
      <header className="chat-header">
        <div>
          <p className="eyebrow">AgentRoom</p>
          <h1>{room.name}</h1>
          <p className="helper-text">You joined as {participantName}.</p>
          <div className="room-meta">
            <span className="meta-pill">
              <span className={`status-dot status-dot--${connectionState}`} />
              {labelForConnectionState(connectionState)}
            </span>
            <span className="meta-pill">Room ID: {room.id}</span>
            <span className="meta-pill">Messages: {messages.length}</span>
          </div>
        </div>

        <div className="button-row chat-header-actions">
          <button className="button button--secondary" type="button" onClick={onLeaveRoom}>
            Leave room
          </button>
        </div>
      </header>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}

      <div className="chat-layout">
        <aside className="sidebar">
          <div className="sidebar-card">
            <ParticipantList participants={participants} />
          </div>
          <div className="sidebar-card">
            <AgentRoster agents={agents} onInsertMention={handleInsertMention} />
          </div>
        </aside>

        <section className="workspace">
          <div className="panel">
            <div className="composer-header">
              <h2>Conversation</h2>
              <p className="helper-text conversation-summary">
                {connectionState === 'connected'
                  ? 'Live updates are connected.'
                  : 'Live updates are paused until the socket reconnects.'}
              </p>
            </div>
            <MessageList messages={messages} />
          </div>

          <MessageComposer
            disabled={connectionState !== 'connected'}
            onInsertMentionRef={insertMentionRef}
            onSend={handleSendMessage}
          />
        </section>
      </div>
    </main>
  )
}

function upsertById(items, nextItem) {
  const existingIndex = items.findIndex((item) => item.id === nextItem.id)
  if (existingIndex === -1) {
    return [...items, nextItem]
  }

  const nextItems = [...items]
  nextItems[existingIndex] = nextItem
  return nextItems
}

function labelForConnectionState(connectionState) {
  switch (connectionState) {
    case 'connected':
      return 'Connected'
    case 'disconnected':
      return 'Disconnected'
    default:
      return 'Connecting'
  }
}
