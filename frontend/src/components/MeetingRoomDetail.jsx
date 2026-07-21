import { useEffect, useMemo, useState } from 'react'
import { Alert, Badge, Button, Group, Modal, Paper, Stack, Text, Title } from '@mantine/core'
import { downloadMessageArtifact, exportRoomMinutesMarkdown, getMessages, getRoom } from '../api/roomClient'
import MessageList from './MessageList'
import MinutesHistory from './MinutesHistory'
import {
  actionsForRoomStatus,
  labelForRoomAction,
  labelForRoomStatus,
  toneForRoomStatus,
} from './meetingAdminModel'
import { downloadBlobFile, downloadMarkdownFile, minutesFilename } from './meetingMinutes'

function MeetingRoomDetail({ busyRoomId = '', onClose, onRoomAction, room }) {
  const [detailRoom, setDetailRoom] = useState(room)
  const [participants, setParticipants] = useState([])
  const [messages, setMessages] = useState([])
  const [hasMore, setHasMore] = useState(false)
  const [nextBefore, setNextBefore] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [minutesMarkdown, setMinutesMarkdown] = useState('')
  const [minutesNotice, setMinutesNotice] = useState('')
  const [minutesRoom, setMinutesRoom] = useState(null)

  useEffect(() => {
    setDetailRoom(room)
  }, [room])

  useEffect(() => {
    let isCurrent = true

    const loadRoomDetail = async () => {
      setIsLoading(true)
      setErrorMessage('')
      setMinutesNotice('')

      const [roomResult, messagesResult, minutesResult] = await Promise.allSettled([
        getRoom(room.id),
        getMessages(room.id, '', { limit: 50 }),
        exportRoomMinutesMarkdown(room.id),
      ])

      if (!isCurrent) {
        return
      }

      if (roomResult.status === 'fulfilled') {
        setDetailRoom((current) => ({ ...current, ...roomResult.value.room }))
        setParticipants(roomResult.value.participants ?? [])
      } else {
        setErrorMessage(roomResult.reason?.message || '加载会议详情失败，请稍后重试。')
      }

      if (messagesResult.status === 'fulfilled') {
        setMessages(messagesResult.value.messages ?? [])
        setHasMore(Boolean(messagesResult.value.hasMore))
        setNextBefore(messagesResult.value.nextBefore || '')
      } else if (!errorMessage) {
        setErrorMessage(messagesResult.reason?.message || '加载会议消息失败，请稍后重试。')
      }

      if (minutesResult.status === 'fulfilled') {
        setMinutesMarkdown(minutesResult.value || '')
      } else if (!isMinutesMissingError(minutesResult.reason?.message)) {
        setMinutesNotice(minutesResult.reason?.message || '加载纪要预览失败。')
      }

      setIsLoading(false)
    }

    void loadRoomDetail()

    return () => {
      isCurrent = false
    }
  }, [room.id])

  const lifecycleActions = useMemo(
    () => actionsForRoomStatus(detailRoom.status).filter((action) => action !== 'detail'),
    [detailRoom.status],
  )
  const ownerParticipant = participants.find((participant) => participant.id === detailRoom.ownerParticipantID)

  const handleLoadMore = async () => {
    if (!hasMore || !nextBefore || isLoadingMore) {
      return
    }

    setIsLoadingMore(true)
    setErrorMessage('')
    try {
      const response = await getMessages(detailRoom.id, '', { before: nextBefore, limit: 50 })
      setMessages((current) => [...(response.messages ?? []), ...current])
      setHasMore(Boolean(response.hasMore))
      setNextBefore(response.nextBefore || '')
    } catch (error) {
      setErrorMessage(error.message || '加载更早消息失败，请稍后重试。')
    } finally {
      setIsLoadingMore(false)
    }
  }

  const handleDownloadMinutes = async () => {
    try {
      const markdown = minutesMarkdown || (await exportRoomMinutesMarkdown(detailRoom.id))
      if (!markdown) {
        setMinutesNotice('当前还没有已保存的会议纪要。')
        return
      }
      downloadMarkdownFile(markdown, minutesFilename(detailRoom, detailRoom.id))
      setMinutesMarkdown(markdown)
      setMinutesNotice('会议纪要已经开始下载。')
    } catch (error) {
      setMinutesNotice(error.message || '导出会议纪要失败，请稍后重试。')
    }
  }

  const handleDownloadArtifact = async (message, artifact) => {
    try {
      const { blob, fileName } = await downloadMessageArtifact(detailRoom.id, message.id, artifact.id)
      downloadBlobFile(blob, fileName || artifact.fileName || 'report.md')
      setErrorMessage('')
    } catch (error) {
      setErrorMessage(error.message || '下载报告失败，请稍后重试。')
    }
  }

  return (
    <>
      <Modal
        opened
        onClose={onClose}
        title={`会议详情 / ${detailRoom.name}`}
        size="90%"
        centered
        classNames={{ content: 'delete-confirm-overlay--scrollable' }}
      >
        <Stack gap="md">
          <Text className="panel-copy">查看会议概览、完整消息历史和纪要操作。</Text>

          {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}
          {minutesNotice ? <Alert color="teal" variant="light">{minutesNotice}</Alert> : null}

          <div className="meeting-detail-grid">
            <Paper component="aside" className="meeting-detail-meta" withBorder radius="md" shadow="none">
              <div className="panel-title-row">
                <Title order={3}>概览</Title>
                <Badge className={`agent-state agent-state--${toneForRoomStatus(detailRoom.status)}`} color="teal" variant="light">
                  {labelForRoomStatus(detailRoom.status)}
                </Badge>
              </div>

              <div className="context-list">
                <div className="context-item">
                  <span>会议 ID</span>
                  <strong>{detailRoom.id}</strong>
                </div>
                <div className="context-item">
                  <span>创建时间</span>
                  <strong>{formatDateTime(detailRoom.createdAt)}</strong>
                </div>
                <div className="context-item">
                  <span>关闭时间</span>
                  <strong>{formatDateTime(detailRoom.closedAt) || '未关闭'}</strong>
                </div>
                <div className="context-item">
                  <span>关闭原因</span>
                  <strong>{labelForClosedReason(detailRoom.closedReason)}</strong>
                </div>
                <div className="context-item">
                  <span>当前房主</span>
                  <strong>{ownerParticipant?.name || detailRoom.ownerParticipantID || '无'}</strong>
                </div>
                <div className="context-item">
                  <span>消息总数</span>
                  <strong>{room.messageCount ?? messages.length}</strong>
                </div>
              </div>

              <Group className="meeting-detail-actions" gap="xs">
                {lifecycleActions.map((action) => (
                  <Button
                    key={action}
                    variant="light"
                    color="teal"
                    size="xs"
                    type="button"
                    disabled={busyRoomId === detailRoom.id}
                    onClick={() => onRoomAction(detailRoom, action)}
                  >
                    {labelForRoomAction(action)}
                  </Button>
                ))}
                <Button variant="subtle" color="gray" size="xs" type="button" onClick={() => setMinutesRoom(detailRoom)}>
                  纪要版本
                </Button>
                <Button variant="subtle" color="gray" size="xs" type="button" onClick={handleDownloadMinutes}>
                  导出纪要
                </Button>
              </Group>

              <section className="room-minutes-preview room-minutes-preview--admin">
                <div className="panel-title-row">
                  <Title order={3}>最新纪要</Title>
                  {minutesMarkdown ? <Badge color="gray" variant="light">Markdown</Badge> : null}
                </div>
                {minutesMarkdown ? (
                  <pre>{minutesMarkdown}</pre>
                ) : (
                  <Text className="sidebar-empty">当前还没有已保存的会议纪要。</Text>
                )}
              </section>
            </Paper>

            <Paper component="section" className="meeting-detail-messages" withBorder radius="md" shadow="none">
              <div className="panel-header panel-header--horizontal">
                <div>
                  <Text className="eyebrow eyebrow--subtle">消息历史</Text>
                  <Title order={3}>会议期间的全部消息</Title>
                </div>
                {hasMore ? (
                  <Button
                    className="room-history-load-more"
                    variant="subtle"
                    color="gray"
                    size="xs"
                    type="button"
                    onClick={handleLoadMore}
                    disabled={isLoadingMore}
                  >
                    {isLoadingMore ? '加载中...' : '查看更多早消息'}
                  </Button>
                ) : null}
              </div>

              {isLoading ? (
                <Text className="sidebar-empty">正在加载会议消息...</Text>
              ) : messages.length === 0 ? (
                <Text className="sidebar-empty">这个会议还没有消息记录。</Text>
              ) : (
                <div className="room-history-list">
                  <MessageList currentParticipantName="" messages={messages} onDownloadArtifact={handleDownloadArtifact} />
                </div>
              )}
            </Paper>
          </div>
        </Stack>
      </Modal>

      {minutesRoom ? <MinutesHistory room={minutesRoom} onClose={() => setMinutesRoom(null)} /> : null}
    </>
  )
}

function formatDateTime(value) {
  if (!value) {
    return ''
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function labelForClosedReason(reason) {
  switch (reason) {
    case 'manual':
      return '由房主主动关闭'
    case 'last_human_left':
      return '最后一位参会者离开后自动关闭'
    case 'admin_unarchive':
      return '管理员取消归档后保持关闭'
    default:
      return '未记录'
  }
}

function isMinutesMissingError(message) {
  return String(message || '').toLowerCase().includes('not found')
}

export default MeetingRoomDetail
