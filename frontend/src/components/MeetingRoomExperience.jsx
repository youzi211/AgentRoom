import { useCallback, useEffect, useMemo, useState } from 'react'
import { Badge, Button, Title } from '@mantine/core'
import {
  Bot,
  ChevronDown,
  ClipboardList,
  Copy,
  DoorOpen,
  Upload,
} from 'lucide-react'
import {
  deleteKnowledgeDocument,
  exportRoomMinutesMarkdown,
  generateRoomMinutes,
  getRoomKnowledge,
  uploadRoomKnowledge,
} from '../api/roomClient'
import AgentActivityPanel from './AgentActivityPanel'
import AgentRoster from './AgentRoster'
import FocusTimeline from './FocusTimeline'
import KnowledgePanel from './KnowledgePanel'
import MeetingMinutesPanel from './MeetingMinutesPanel'
import MessageComposer from './MessageComposer'
import { filterMessagesByKind } from './messageFilters'
import MessageList from './MessageList'

function MeetingRoomExperience({
  activityError,
  activityItems,
  activityLoading,
  agents,
  connectionState,
  copyState,
  focusPoints,
  insertMentionRef,
  messageListRef,
  messages,
  onCopyRoomID,
  onDownloadArtifact,
  onLeaveRoom,
  onSendMessage,
  participantName,
  participants,
  room,
  roomId,
  roomPasscode,
  thinkingAgents,
  visibleMessages,
}) {
  const [now, setNow] = useState(() => Date.now())
  const [messageFilter, setMessageFilter] = useState('all')

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])

  const elapsedLabel = useMemo(() => formatElapsed(room?.createdAt, now), [now, room?.createdAt])
  const displayedMessages = useMemo(
    () => filterMessagesByKind(visibleMessages, messageFilter),
    [messageFilter, visibleMessages],
  )
  const listRoomDocuments = useCallback(() => getRoomKnowledge(roomId, roomPasscode), [roomId, roomPasscode])
  const uploadRoomDocument = useCallback((file) => uploadRoomKnowledge(roomId, file), [roomId])
  const roomIdLabel = truncateRoomId(roomId)
  const activeAgentCount = agents.length
  const messageCount = displayedMessages.length

  return (
    <main className="meeting-room-v2">
      <header className="meeting-room-v2__topbar">
        <div className="meeting-room-v2__brand">
          <span className="meeting-room-v2__logo">AR</span>
          <strong>AgentRoom</strong>
        </div>

        <div className="meeting-room-v2__title">
          <Title order={1}>{room.name}</Title>
          <ChevronDown size={16} />
          <span className={`meeting-room-v2__status meeting-room-v2__status--${connectionState}`}>
            <span />
            {labelForConnectionState(connectionState)}
          </span>
          <time>{elapsedLabel}</time>
        </div>

        <div className="meeting-room-v2__top-actions">
          <span className="meeting-room-v2__room-id">房间 ID: {roomIdLabel}</span>
          <Button variant="default" size="xs" leftSection={<Copy size={15} />} onClick={onCopyRoomID}>
            {copyState === 'copied' ? '已复制' : '复制'}
          </Button>
          <Button variant="outline" color="red" size="xs" leftSection={<DoorOpen size={15} />} onClick={onLeaveRoom}>
            离开会议
          </Button>
        </div>
      </header>

      <div className="meeting-room-v2__grid">
        <aside className="meeting-room-v2__left">
          <section className="meeting-room-v2__panel meeting-room-v2__info">
            <PanelTitle icon={<ClipboardList size={18} />} title="会议信息" />
            <dl className="meeting-room-v2__info-list">
              <InfoItem label="会议名称" value={room.name} />
              <InfoItem label="房间 ID" value={roomId} />
              <InfoItem label="创建者" value="公开加入" />
              <InfoItem label="Agent 数" value={String(activeAgentCount)} />
              <div className="meeting-room-v2__info-item">
                <dt>会议模式</dt>
                <dd>
                  <Badge variant="light" color="teal">
                    {labelForDialogueMode(room.dialoguePolicy?.mode)}
                  </Badge>
                </dd>
              </div>
            </dl>
          </section>

          <section className="meeting-room-v2__panel meeting-room-v2__files">
            <KnowledgePanel
              title="会议文件"
              description=""
              emptyText="暂无会议文件。上传 .md 后，Agent 回答时会自动参考。"
              listDocuments={listRoomDocuments}
              onUploadDocument={uploadRoomDocument}
              onDeleteDocument={deleteKnowledgeDocument}
            />
            <div className="meeting-room-v2__file-share">
              <Upload size={18} />
              <div>
                <strong>文件已共享给所有 Agent</strong>
                <span>上传的知识文件将用于 Agent 协同讨论</span>
              </div>
            </div>
          </section>

          <section className="meeting-room-v2__panel meeting-room-v2__minutes">
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
          </section>
        </aside>

        <section className="meeting-room-v2__center">
          <div className="meeting-room-v2__discussion-head">
            <PanelTitle icon={<Bot size={18} />} title="实时讨论" />
            <div className="meeting-room-v2__filters">
              <Button
                variant={messageFilter === 'all' ? 'filled' : 'subtle'}
                color={messageFilter === 'all' ? 'teal' : 'gray'}
                size="xs"
                aria-pressed={messageFilter === 'all'}
                onClick={() => setMessageFilter('all')}
              >
                全部
              </Button>
              <Button
                variant={messageFilter === 'human' ? 'filled' : 'subtle'}
                color={messageFilter === 'human' ? 'teal' : 'gray'}
                size="xs"
                aria-pressed={messageFilter === 'human'}
                onClick={() => setMessageFilter('human')}
              >
                人类
              </Button>
              <Button
                variant={messageFilter === 'agent' ? 'filled' : 'subtle'}
                color={messageFilter === 'agent' ? 'teal' : 'gray'}
                size="xs"
                aria-pressed={messageFilter === 'agent'}
                onClick={() => setMessageFilter('agent')}
              >
                Agent
              </Button>
              <span>{messageCount} 条消息</span>
              <span>{thinkingAgents.length > 0 ? `${thinkingAgents.length} 个 Agent 正在响应` : '等待讨论'}</span>
            </div>
          </div>

          <MessageList
            ref={messageListRef}
            currentParticipantName={participantName}
            messages={displayedMessages}
            onDownloadArtifact={onDownloadArtifact}
            thinkingAgents={thinkingAgents}
          />
          <MessageComposer
            agents={agents}
            currentParticipantName={participantName}
            disabled={connectionState !== 'connected'}
            onInsertMentionRef={insertMentionRef}
            onSend={onSendMessage}
            participants={participants}
          />
        </section>

        <aside className="meeting-room-v2__right">
          <section className="meeting-room-v2__panel meeting-room-v2__agents">
            <AgentRoster agents={agents} thinkingAgents={thinkingAgents} onInsertMention={(mention) => insertMentionRef.current?.(mention)} />
          </section>
          <FocusTimeline focusPoints={focusPoints} />
          <AgentActivityPanel activities={activityItems} errorMessage={activityError} isLoading={activityLoading} />
        </aside>
      </div>
    </main>
  )
}

function PanelTitle({ icon, title }) {
  return (
    <div className="meeting-room-v2__panel-title">
      {icon}
      <h2>{title}</h2>
    </div>
  )
}

function InfoItem({ label, value }) {
  return (
    <div className="meeting-room-v2__info-item">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  )
}

function formatElapsed(value, now) {
  const startedAt = new Date(value).getTime()
  if (!Number.isFinite(startedAt)) {
    return '00:00:00'
  }
  const seconds = Math.max(0, Math.floor((now - startedAt) / 1000))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainingSeconds = seconds % 60
  return [hours, minutes, remainingSeconds].map((part) => String(part).padStart(2, '0')).join(':')
}

function truncateRoomId(roomId) {
  if (!roomId || roomId.length <= 22) {
    return roomId
  }
  return `${roomId.slice(0, 22)}_`
}

function labelForConnectionState(connectionState) {
  switch (connectionState) {
    case 'connected':
      return '进行中'
    case 'disconnected':
      return '已断开'
    default:
      return '连接中'
  }
}

function labelForDialogueMode(mode) {
  return mode === 'guided_dialogue' ? '单 Agent 对话' : '多 Agent 协作'
}

export default MeetingRoomExperience
