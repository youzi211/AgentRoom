import { useEffect, useState } from 'react'
import { Alert, Badge, Button, Group, Paper, Text, Title } from '@mantine/core'
import { downloadMessageArtifact, exportRoomMinutesMarkdown, getMessages } from '../api/roomClient'
import MessageList from './MessageList'
import { downloadBlobFile, downloadMarkdownFile, minutesFilename } from './meetingMinutes'

function RoomReadOnly({ room, roomPasscode = '', onBackHome }) {
  const [messages, setMessages] = useState([])
  const [hasMore, setHasMore] = useState(false)
  const [nextBefore, setNextBefore] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [minutesMarkdown, setMinutesMarkdown] = useState('')
  const [minutesNotice, setMinutesNotice] = useState('')

  useEffect(() => {
    let isCurrent = true

    const loadRoomHistory = async () => {
      setIsLoading(true)
      setErrorMessage('')
      setMinutesNotice('')

      const [messagesResult, minutesResult] = await Promise.allSettled([
        getMessages(room.id, roomPasscode, { limit: 50 }),
        exportRoomMinutesMarkdown(room.id, roomPasscode),
      ])

      if (!isCurrent) {
        return
      }

      if (messagesResult.status === 'fulfilled') {
        setMessages(messagesResult.value.messages ?? [])
        setHasMore(Boolean(messagesResult.value.hasMore))
        setNextBefore(messagesResult.value.nextBefore || '')
      } else {
        setErrorMessage(messagesResult.reason?.message || '加载会议消息失败，请稍后重试。')
      }

      if (minutesResult.status === 'fulfilled') {
        setMinutesMarkdown(minutesResult.value || '')
      } else if (!isMinutesMissingError(minutesResult.reason?.message)) {
        setMinutesNotice(minutesResult.reason?.message || '加载纪要预览失败。')
      }

      setIsLoading(false)
    }

    void loadRoomHistory()

    return () => {
      isCurrent = false
    }
  }, [room.id, roomPasscode])

  const handleLoadMore = async () => {
    if (!hasMore || !nextBefore || isLoadingMore) {
      return
    }

    setIsLoadingMore(true)
    setErrorMessage('')
    try {
      const response = await getMessages(room.id, roomPasscode, { before: nextBefore, limit: 50 })
      setMessages((current) => [...(response.messages ?? []), ...current])
      setHasMore(Boolean(response.hasMore))
      setNextBefore(response.nextBefore || '')
    } catch (error) {
      setErrorMessage(error.message || '加载更早的消息失败，请稍后重试。')
    } finally {
      setIsLoadingMore(false)
    }
  }

  const handleDownloadMinutes = async () => {
    try {
      const markdown = minutesMarkdown || (await exportRoomMinutesMarkdown(room.id, roomPasscode))
      if (!markdown) {
        setMinutesNotice('当前还没有可下载的会议纪要。')
        return
      }
      downloadMarkdownFile(markdown, minutesFilename(room, room.id))
      setMinutesMarkdown(markdown)
      setMinutesNotice('会议纪要已经开始下载。')
    } catch (error) {
      setMinutesNotice(error.message || '导出会议纪要失败，请稍后重试。')
    }
  }

  const handleDownloadArtifact = async (message, artifact) => {
    try {
      const { blob, fileName } = await downloadMessageArtifact(room.id, message.id, artifact.id, roomPasscode)
      downloadBlobFile(blob, fileName || artifact.fileName || 'report.md')
      setErrorMessage('')
    } catch (error) {
      setErrorMessage(error.message || '下载报告失败，请稍后重试。')
    }
  }

  return (
    <main className="workbench room-readonly">
      <Paper component="section" className="panel room-readonly-banner" withBorder radius="md" shadow="xs">
        <div>
          <Text className="eyebrow">只读查看</Text>
          <Title order={1}>{room.name}</Title>
          <Text className="section-copy">
            会议已经关闭。你可以继续查看历史消息和纪要，但不能加入实时连接、发送消息或恢复会议。
          </Text>
        </div>
        <Group className="panel-badge-group" gap="xs">
          <Badge color="gray" variant="light">已关闭</Badge>
          <Button variant="default" type="button" onClick={onBackHome}>
            返回入口
          </Button>
        </Group>
      </Paper>

      {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}
      {minutesNotice ? <Alert color="teal" variant="light">{minutesNotice}</Alert> : null}

      <div className="room-readonly-grid">
        <Paper component="aside" className="panel room-readonly-side" withBorder radius="md" shadow="xs">
          <div className="panel-header">
            <div>
              <Title order={2}>会议信息</Title>
              <Text className="panel-copy">关闭后的会议保留历史消息与纪要，方便参会者只读回看。</Text>
            </div>
          </div>

          <div className="context-list">
            <div className="context-item">
              <span>会议 ID</span>
              <strong>{room.id}</strong>
            </div>
            <div className="context-item">
              <span>创建时间</span>
              <strong>{formatDateTime(room.createdAt)}</strong>
            </div>
            <div className="context-item">
              <span>关闭时间</span>
              <strong>{formatDateTime(room.closedAt) || '未记录'}</strong>
            </div>
            <div className="context-item">
              <span>关闭原因</span>
              <strong>{labelForClosedReason(room.closedReason)}</strong>
            </div>
          </div>

          <section className="room-minutes-preview">
            <div className="panel-title-row">
              <Title order={2}>会议纪要</Title>
              <Button variant="light" color="teal" size="xs" type="button" onClick={handleDownloadMinutes}>
                下载 Markdown
              </Button>
            </div>
            {minutesMarkdown ? (
              <pre>{minutesMarkdown}</pre>
            ) : (
              <Text className="sidebar-empty">当前还没有已保存的会议纪要。</Text>
            )}
          </section>
        </Paper>

        <Paper component="section" className="panel room-history-panel" withBorder radius="md" shadow="xs">
          <div className="panel-header panel-header--horizontal">
            <div>
              <Text className="eyebrow eyebrow--subtle">会议历史</Text>
              <Title order={2}>关闭前的全部消息</Title>
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
            <Text className="sidebar-empty">正在加载历史消息...</Text>
          ) : messages.length === 0 ? (
            <Text className="sidebar-empty">这个会议还没有消息记录。</Text>
          ) : (
            <div className="room-history-list">
              <MessageList currentParticipantName="" messages={messages} onDownloadArtifact={handleDownloadArtifact} />
            </div>
          )}
        </Paper>
      </div>
    </main>
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

export default RoomReadOnly
