import { useCallback, useEffect, useRef, useState } from 'react'
import {
  createRoomSocket,
  deleteKnowledgeDocument,
  exportRoomMinutesMarkdown,
  generateRoomMinutes,
  getRoom,
  getRoomActivity,
  getRoomKnowledge,
  getMessages,
  uploadRoomKnowledge,
} from '../api/roomClient'
import { clearRoomSession, navigateHome, navigateRoom } from '../routing'
import AgentActivityPanel from './AgentActivityPanel'
import AgentRoster from './AgentRoster'
import FocusTimeline from './FocusTimeline'
import KnowledgePanel from './KnowledgePanel'
import MeetingMinutesPanel from './MeetingMinutesPanel'
import MessageComposer from './MessageComposer'
import MessageList from './MessageList'
import ParticipantList from './ParticipantList'
import ResizeHandle from './ResizeHandle'
import { mergeActivityEvent, normalizeActivityPayload } from './agentActivity'
import { buildParticipantJoinedNotice, mergeTimelineMessages } from './liveRoomNotices'
import { nextRouteAfterLiveTermination } from './roomAccess'
import '../chat-room.css'

const ROOM_SNAPSHOT_EVENT = 'room_snapshot'
const MESSAGE_EVENT = 'message'
const PARTICIPANT_JOINED_EVENT = 'participant_joined'
const PARTICIPANT_LEFT_EVENT = 'participant_left'
const ERROR_EVENT = 'error'
const FOCUS_UPDATE_EVENT = 'focus_update'
const AGENT_ACTIVITY_EVENT = 'agent_activity'
const ROOM_CLOSED_EVENT = 'room_closed'
const ROOM_ARCHIVED_EVENT = 'room_archived'

export default function ChatRoom({ initialRoom, participantName, roomId, roomPasscode, onLeaveRoom }) {
  const [room, setRoom] = useState(initialRoom)
  const [participants, setParticipants] = useState([])
  const [agents, setAgents] = useState([])
  const [messages, setMessages] = useState([])
  const [liveNotices, setLiveNotices] = useState([])
  const [connectionState, setConnectionState] = useState('connecting')
  const [errorMessage, setErrorMessage] = useState('')
  const [copyState, setCopyState] = useState('idle')
  const [thinkingAgents, setThinkingAgents] = useState([])
  const [activityItems, setActivityItems] = useState([])
  const [activityLoading, setActivityLoading] = useState(true)
  const [activityError, setActivityError] = useState('')
  const [focusPoints, setFocusPoints] = useState([])
  const [selfParticipantID, setSelfParticipantID] = useState('')
  const [transferTargetID, setTransferTargetID] = useState('')
  const [leftPanelWidth, setLeftPanelWidth] = useState(270)
  const [rightPanelWidth, setRightPanelWidth] = useState(320)
  const socketRef = useRef(null)
  const terminatedRef = useRef(false)
  const insertMentionRef = useRef(() => {})
  const messageListRef = useRef(null)
  const transferableParticipants = participants.filter((participant) => participant.id !== selfParticipantID)
  const ownerParticipant = participants.find((participant) => participant.id === room.ownerParticipantID)
  const isOwner = Boolean(selfParticipantID && room.ownerParticipantID === selfParticipantID)
  const visibleMessages = mergeTimelineMessages(messages, liveNotices)

  useEffect(() => {
    if (!transferableParticipants.some((participant) => participant.id === transferTargetID)) {
      setTransferTargetID(transferableParticipants[0]?.id || '')
    }
  }, [transferTargetID, transferableParticipants])

  const closeCurrentSocket = useCallback(() => {
    const socket = socketRef.current
    socketRef.current = null

    if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
      socket.close()
    }
  }, [])

  const handleLiveTermination = useCallback(
    (status, nextRoom = null) => {
      terminatedRef.current = true
      setConnectionState('disconnected')
      clearRoomSession(roomId)

      const nextRoute = nextRouteAfterLiveTermination({
        status,
        roomId,
        passcode: roomPasscode,
      })

      if (nextRoute.route === 'room') {
        navigateRoom(
          roomId,
          {
            initialRoom: nextRoom,
            passcode: roomPasscode,
          },
          { replace: true },
        )
        return
      }

      navigateHome({ replace: true })
    },
    [roomId, roomPasscode],
  )

  useEffect(() => {
    let isCurrent = true
    terminatedRef.current = false

    const handleServerEvent = (event) => {
      switch (event.type) {
        case ROOM_SNAPSHOT_EVENT:
          if (event.room) {
            setRoom(event.room)
          }
          if (event.participantID) {
            setSelfParticipantID(event.participantID)
          }
          setParticipants(event.participants ?? [])
          setAgents(event.agents ?? [])
          setMessages(event.messages ?? [])
          setErrorMessage('')
          return
        case PARTICIPANT_JOINED_EVENT:
          if (event.participant) {
            setParticipants((current) => upsertById(current, event.participant))
            const notice = buildParticipantJoinedNotice({
              participant: event.participant,
              currentParticipantID: selfParticipantID,
            })
            if (notice) {
              setLiveNotices((current) => upsertById(current, notice))
            }
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
            if (event.message.senderType === 'agent') {
              setThinkingAgents((current) => current.filter((agent) => agent.id !== event.message.senderID))
            }
          }
          return
        case FOCUS_UPDATE_EVENT:
          if (event.focusPoints && event.focusPoints.length > 0) {
            setFocusPoints((current) => [...current, ...event.focusPoints])
          }
          return
        case AGENT_ACTIVITY_EVENT:
          if (event.activity) {
            setActivityItems((current) => mergeActivityEvent(current, event.activity))
            if (event.activity.kind === 'agent_run' && event.activity.agentID) {
              if (event.activity.phase === 'started') {
                setThinkingAgents((current) => mergeAgents(current, [agentFromActivity(event.activity)]))
              }
              if (event.activity.phase === 'finished') {
                setThinkingAgents((current) => current.filter((agent) => agent.id !== event.activity.agentID))
              }
            }
          }
          return
        case ROOM_CLOSED_EVENT:
          if (event.room) {
            setRoom(event.room)
          }
          setThinkingAgents([])
          closeCurrentSocket()
          handleLiveTermination('closed', event.room || null)
          return
        case ROOM_ARCHIVED_EVENT:
          if (event.room) {
            setRoom(event.room)
          }
          setThinkingAgents([])
          closeCurrentSocket()
          handleLiveTermination('archived', event.room || null)
          return
        case ERROR_EVENT:
          setErrorMessage(event.error || '房间返回了一条错误消息。')
          setThinkingAgents([])
          return
        default:
          return
      }
    }

    const connectToRoom = async () => {
      setConnectionState('connecting')
      setActivityLoading(true)
      setActivityError('')

      const [roomResult, messagesResult, activityResult] = await Promise.allSettled([
        getRoom(roomId, roomPasscode),
        getMessages(roomId, roomPasscode),
        getRoomActivity(roomId, roomPasscode),
      ])
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

      if (activityResult.status === 'fulfilled') {
        setActivityItems(normalizeActivityPayload(activityResult.value))
      } else {
        setActivityError(activityResult.reason?.message || '加载 Agent 活动失败。')
      }
      setActivityLoading(false)

      if (loadErrors.length > 0) {
        setErrorMessage(loadErrors.join(' '))
      }

      const socket = createRoomSocket(roomId, participantName, roomPasscode)
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
          setErrorMessage('收到了无法解析的服务端消息。')
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
        if (terminatedRef.current) {
          return
        }
        setConnectionState('disconnected')
      })
    }

    void connectToRoom()

    return () => {
      isCurrent = false
      closeCurrentSocket()
    }
  }, [closeCurrentSocket, handleLiveTermination, participantName, roomId, roomPasscode])

  useEffect(() => {
    const listEl = messageListRef.current
    if (!listEl) {
      return
    }
    listEl.scrollTop = listEl.scrollHeight
  }, [messages, liveNotices, thinkingAgents])

  const handleSendMessage = async (content) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setErrorMessage('请在连接恢复后再发送消息。')
      return false
    }

    const mentioned = agents.filter((agent) => content.includes(agent.mention))
    if (mentioned.length > 0) {
      setThinkingAgents((current) => mergeAgents(current, mentioned))
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

  const sendRoomControlEvent = (payload) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      setErrorMessage('请在连接恢复后再操作会议。')
      return false
    }

    socket.send(JSON.stringify(payload))
    setErrorMessage('')
    return true
  }

  const handleCloseRoom = () => {
    if (!isOwner) {
      return
    }

    if (!window.confirm('确认要关闭会议吗？关闭后只能只读查看历史。')) {
      return
    }

    sendRoomControlEvent({ type: 'close_room' })
  }

  const handleTransferOwner = () => {
    if (!isOwner || !transferTargetID) {
      return
    }

    sendRoomControlEvent({
      type: 'transfer_owner',
      participantID: transferTargetID,
    })
  }

  const handleInsertMention = (mention) => {
    insertMentionRef.current?.(mention)
  }

  const handleCopyRoomID = async () => {
    try {
      await navigator.clipboard.writeText(roomId)
      setCopyState('copied')
      window.setTimeout(() => setCopyState('idle'), 1600)
    } catch {
      setCopyState('failed')
      window.setTimeout(() => setCopyState('idle'), 1600)
    }
  }

  return (
    <main className="chat-workbench">
      <header className="chat-topbar">
        <div className="chat-room-meta">
          <div className="brand-mark brand-mark--small">AR</div>
          <div>
            <h1 className="chat-topbar-title">{room.name}</h1>
            <div className="chat-topbar-subtitle">
              <span className={`status-dot status-dot--${connectionState}`} />
              <span>{labelForConnectionState(connectionState)}</span>
              <span>{`房间 ID：${roomId}`}</span>
              <span title={descriptionForDialogueMode(room.dialoguePolicy?.mode)}>
                {`Agent 模式：${labelForDialogueMode(room.dialoguePolicy?.mode)}`}
              </span>
            </div>
          </div>
        </div>
        <div className="chat-topbar-actions">
          <span className="chat-identity">{`以 ${participantName} 加入`}</span>
          <button className="button button--secondary button--compact" type="button" onClick={handleCopyRoomID}>
            {copyState === 'copied' ? '已复制' : copyState === 'failed' ? '复制失败' : '复制房间 ID'}
          </button>
          <button className="button button--ghost button--compact" type="button" onClick={onLeaveRoom}>
            离开
          </button>
        </div>
      </header>

      {errorMessage ? <p className="banner banner--error banner--compact">{errorMessage}</p> : null}

      <div className="chat-layout">
        <aside className="chat-sidebar chat-context-panel" style={{ width: leftPanelWidth, minWidth: leftPanelWidth }}>
          <section className="sidebar-section meeting-context-summary">
            <div className="sidebar-header">
              <h2>会议上下文</h2>
              <span className="sidebar-count">Live</span>
            </div>
            <div className="context-list">
              <div className="context-item">
                <span>会议室</span>
                <strong>{room.name}</strong>
              </div>
              <div className="context-item">
                <span>房间 ID</span>
                <strong>{roomId}</strong>
              </div>
              <div className="context-item">
                <span>Agent 模式</span>
                <strong>{labelForDialogueMode(room.dialoguePolicy?.mode)}</strong>
              </div>
              <div className="context-item">
                <span>参会角色</span>
                <strong>{`${participants.length} 人 / ${agents.length} 个 Agent`}</strong>
              </div>
            </div>
          </section>

          <section className="sidebar-section meeting-owner-panel">
            <div className="sidebar-header">
              <h2>会议控制</h2>
              {isOwner ? <span className="meeting-owner-badge">房主</span> : null}
            </div>
            <div className="context-list">
              <div className="context-item">
                <span>当前房主</span>
                <strong>{ownerParticipant?.name || (isOwner ? participantName : '暂无')}</strong>
              </div>
            </div>
            <p className="sidebar-note">
              {isOwner
                ? '你可以关闭会议，或把房主权限转交给其他在线参会者。'
                : `当前由 ${ownerParticipant?.name || '其他成员'} 拥有关闭会议与转交权限。`}
            </p>
            {isOwner ? (
              <div className="meeting-owner-actions">
                <select
                  className="text-input meeting-owner-select"
                  value={transferTargetID}
                  onChange={(event) => setTransferTargetID(event.target.value)}
                  disabled={connectionState !== 'connected' || transferableParticipants.length === 0}
                >
                  <option value="">选择新的房主</option>
                  {transferableParticipants.map((participant) => (
                    <option key={participant.id} value={participant.id}>
                      {participant.name}
                    </option>
                  ))}
                </select>
                <div className="meeting-owner-action-row">
                  <button
                    className="button button--secondary button--compact"
                    type="button"
                    onClick={handleTransferOwner}
                    disabled={connectionState !== 'connected' || !transferTargetID}
                  >
                    转交房主
                  </button>
                  <button
                    className="button button--danger button--compact"
                    type="button"
                    onClick={handleCloseRoom}
                    disabled={connectionState !== 'connected'}
                  >
                    关闭会议
                  </button>
                </div>
              </div>
            ) : null}
          </section>

          <ParticipantList participants={participants} />
          <MeetingMinutesPanel
            agents={agents}
            exportMinutesMarkdown={() => exportRoomMinutesMarkdown(roomId, roomPasscode)}
            focusPoints={focusPoints}
            generateMinutes={() => generateRoomMinutes(roomId, roomPasscode)}
            messages={messages}
            participants={participants}
            room={room}
            roomId={roomId}
          />
          <KnowledgePanel
            title="会议文件"
            description="所有参会 Agent 都会参考这里的 Markdown 资料。"
            emptyText="暂时还没有会议文件。上传 .md 后，Agent 回答时会自动参考。"
            listDocuments={() => getRoomKnowledge(roomId, roomPasscode)}
            onUploadDocument={(file) => uploadRoomKnowledge(roomId, file)}
            onDeleteDocument={deleteKnowledgeDocument}
          />
        </aside>

        <ResizeHandle
          direction="horizontal"
          onResize={setLeftPanelWidth}
          minWidth={200}
          maxWidth={400}
          size={leftPanelWidth}
        />

        <section className="conversation-workspace">
          <div className="conversation-heading">
            <div>
              <p className="eyebrow eyebrow--subtle">实时讨论</p>
              <h2>会议记录与决策流</h2>
            </div>
            <div className="conversation-toolbar">
              <span>{`${visibleMessages.length} 条消息`}</span>
              <span>{thinkingAgents.length > 0 ? `${thinkingAgents.length} 个 Agent 正在思考` : '等待讨论'}</span>
            </div>
          </div>
          <MessageList
            ref={messageListRef}
            currentParticipantName={participantName}
            messages={visibleMessages}
            thinkingAgents={thinkingAgents}
          />
          <MessageComposer
            agents={agents}
            currentParticipantName={participantName}
            disabled={connectionState !== 'connected'}
            onInsertMentionRef={insertMentionRef}
            onSend={handleSendMessage}
            participants={participants}
          />
        </section>

        <ResizeHandle
          direction="horizontal"
          invertDelta
          onResize={setRightPanelWidth}
          minWidth={200}
          maxWidth={450}
          size={rightPanelWidth}
        />

        <aside className="chat-sidebar agent-workbench-panel" style={{ width: rightPanelWidth, minWidth: rightPanelWidth }}>
          <div className="agent-roster-region">
            <AgentRoster agents={agents} thinkingAgents={thinkingAgents} onInsertMention={handleInsertMention} />
          </div>
          <FocusTimeline focusPoints={focusPoints} />
          <AgentActivityPanel activities={activityItems} errorMessage={activityError} isLoading={activityLoading} />
        </aside>
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

function mergeAgents(current, nextAgents) {
  const byID = new Map(current.map((agent) => [agent.id, agent]))
  nextAgents.forEach((agent) => byID.set(agent.id, agent))
  return Array.from(byID.values())
}

function agentFromActivity(activity) {
  const name = activity.agentName || activity.agentID || 'Agent'
  return {
    id: activity.agentID,
    name,
    mention: `@${name}`,
    role: '',
  }
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

function labelForDialogueMode(mode) {
  return mode === 'guided_dialogue' ? '引导多轮' : '点名单轮'
}

function descriptionForDialogueMode(mode) {
  return mode === 'guided_dialogue'
    ? '被点到的 Agent 会先回复，并允许继续 @ 其他 Agent。'
    : '用户直接 @ 的 Agent 会先回复；如果 Agent 明确 @ 了同伴，也会按房间策略继续接话。'
}
