import { useEffect, useState } from 'react'
import { exportRoomMinutesMarkdown, getMessages } from '../api/roomClient'
import MessageList from './MessageList'
import { downloadMarkdownFile, minutesFilename } from './meetingMinutes'

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

  return (
    <main className="workbench room-readonly">
      <section className="panel room-readonly-banner">
        <div>
          <p className="eyebrow">只读查看</p>
          <h1>{room.name}</h1>
          <p className="section-copy">
            会议已经关闭。你可以继续查看历史消息和纪要，但不能加入实时连接、发送消息或恢复会议。
          </p>
        </div>
        <div className="panel-badge-group">
          <span className="panel-badge panel-badge--neutral">已关闭</span>
          <button className="button button--secondary" type="button" onClick={onBackHome}>
            返回入口
          </button>
        </div>
      </section>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      {minutesNotice ? <p className="banner banner--success">{minutesNotice}</p> : null}

      <div className="room-readonly-grid">
        <aside className="panel room-readonly-side">
          <div className="panel-header">
            <div>
              <h2>会议信息</h2>
              <p className="panel-copy">关闭后的会议保留历史消息与纪要，方便参会者只读回看。</p>
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
              <h2>会议纪要</h2>
              <button className="button button--secondary button--compact" type="button" onClick={handleDownloadMinutes}>
                下载 Markdown
              </button>
            </div>
            {minutesMarkdown ? (
              <pre>{minutesMarkdown}</pre>
            ) : (
              <p className="sidebar-empty">当前还没有已保存的会议纪要。</p>
            )}
          </section>
        </aside>

        <section className="panel room-history-panel">
          <div className="panel-header panel-header--horizontal">
            <div>
              <p className="eyebrow eyebrow--subtle">会议历史</p>
              <h2>关闭前的全部消息</h2>
            </div>
            {hasMore ? (
              <button
                className="button button--ghost button--compact room-history-load-more"
                type="button"
                onClick={handleLoadMore}
                disabled={isLoadingMore}
              >
                {isLoadingMore ? '加载中...' : '查看更早消息'}
              </button>
            ) : null}
          </div>

          {isLoading ? (
            <p className="sidebar-empty">正在加载历史消息...</p>
          ) : messages.length === 0 ? (
            <p className="sidebar-empty">这个会议还没有消息记录。</p>
          ) : (
            <div className="room-history-list">
              <MessageList currentParticipantName="" messages={messages} />
            </div>
          )}
        </section>
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
