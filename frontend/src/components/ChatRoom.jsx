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
          setErrorMessage(event.error || '房间报告了一个错误。')
          return
        default:
          return
      }
    }

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
        loadErrors.push(roomResult.reason?.message || '加载房间信息失败。')
      }

      if (messagesResult.status === 'fulfilled') {
        setMessages(messagesResult.value.messages ?? [])
      } else {
        loadErrors.push(messagesResult.reason?.message || '加载消息失败。')
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
          setErrorMessage('收到了无法解析的服务器消息。')
        }
      })

      socket.addEventListener('error', () => {
        if (!isCurrent) {
          return
        }

        setErrorMessage((current) => current || '实时连接出现错误。')
      })

      socket.addEventListener('close', () => {
        if (!isCurrent) {
          return
        }

        socketRef.current = null
        setConnectionState('disconnected')
      })
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
      setErrorMessage('请重新连接后再发送消息。')
      return false
    }

    socket.send(
      JSON.stringify({
        type: MESSAGE_EVENT,
        content,
      }),
    )
    setErrorMessage('')
    return true
  }

  const handleInsertMention = (mention) => {
    insertMentionRef.current?.(mention)
  }

  return (
    <main className="app-shell">
      <header className="page-header">
        <div>
          <p className="eyebrow">会议室</p>
          <h1>{room.name}</h1>
          <p className="section-copy">
            你以 <span className="participant-highlight">{participantName}</span> 身份加入。正常聊天不会触发 Agent，明确 @ 后它才会发言。
          </p>
          <div className="room-meta">
            <div className="meta-pill">
              <span className="meta-label">连接状态</span>
              <div className="status-row">
                <span className={`status-dot status-dot--${connectionState}`} />
                <span className="meta-value">{labelForConnectionState(connectionState)}</span>
              </div>
            </div>
            <div className="meta-pill">
              <span className="meta-label">房间 ID</span>
              <span className="meta-value room-id-value">{room.id}</span>
            </div>
            <div className="meta-pill">
              <span className="meta-label">消息数</span>
              <span className="meta-value">{messages.length}</span>
            </div>
          </div>
        </div>

        <button className="button button--secondary" type="button" onClick={onLeaveRoom}>
          离开房间
        </button>
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
          <div className="panel panel--conversation">
            <div className="workspace-header">
              <div>
                <p className="eyebrow eyebrow--subtle">对话</p>
                <h2>实时消息</h2>
              </div>
              <p className="helper-text conversation-summary">
                {connectionState === 'connected' ? '实时连接已建立，新消息会自动出现。' : '实时更新已暂停，等待重新连接。'}
              </p>
            </div>
            <MessageList messages={messages} />
          </div>

          <MessageComposer disabled={connectionState !== 'connected'} onInsertMentionRef={insertMentionRef} onSend={handleSendMessage} />
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
      return '已连接'
    case 'disconnected':
      return '已断开'
    default:
      return '连接中'
  }
}
