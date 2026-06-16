import { useEffect, useMemo, useState } from 'react'
import { exportRoomMinutesMarkdown, getMessages, getRoom } from '../api/roomClient'
import MessageList from './MessageList'
import MinutesHistory from './MinutesHistory'
import {
  actionsForRoomStatus,
  labelForRoomAction,
  labelForRoomStatus,
  toneForRoomStatus,
} from './meetingAdminModel'
import { downloadMarkdownFile, minutesFilename } from './meetingMinutes'

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

  return (
    <>
      <div className="delete-confirm-overlay delete-confirm-overlay--scrollable" role="dialog" aria-modal="true">
        <section className="meeting-detail-card">
          <div className="panel-header panel-header--horizontal">
            <div>
              <p className="eyebrow">会议详情</p>
              <h2>{detailRoom.name}</h2>
              <p className="panel-copy">查看会议概览、完整消息历史和纪要操作。</p>
            </div>
            <button className="button button--ghost button--compact" type="button" onClick={onClose}>
              关闭
            </button>
          </div>

          {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
          {minutesNotice ? <p className="banner banner--success">{minutesNotice}</p> : null}

          <div className="meeting-detail-grid">
            <aside className="meeting-detail-meta">
              <div className="panel-title-row">
                <h3>概览</h3>
                <span className={`agent-state agent-state--${toneForRoomStatus(detailRoom.status)}`}>
                  {labelForRoomStatus(detailRoom.status)}
                </span>
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

              <div className="meeting-detail-actions">
                {lifecycleActions.map((action) => (
                  <button
                    key={action}
                    className="button button--secondary button--compact"
                    type="button"
                    disabled={busyRoomId === detailRoom.id}
                    onClick={() => onRoomAction(detailRoom, action)}
                  >
                    {labelForRoomAction(action)}
                  </button>
                ))}
                <button className="button button--ghost button--compact" type="button" onClick={() => setMinutesRoom(detailRoom)}>
                  纪要版本
                </button>
                <button className="button button--ghost button--compact" type="button" onClick={handleDownloadMinutes}>
                  导出纪要
                </button>
              </div>

              <section className="room-minutes-preview room-minutes-preview--admin">
                <div className="panel-title-row">
                  <h3>最新纪要</h3>
                  {minutesMarkdown ? <span className="panel-badge panel-badge--neutral">Markdown</span> : null}
                </div>
                {minutesMarkdown ? (
                  <pre>{minutesMarkdown}</pre>
                ) : (
                  <p className="sidebar-empty">当前还没有已保存的会议纪要。</p>
                )}
              </section>
            </aside>

            <section className="meeting-detail-messages">
              <div className="panel-header panel-header--horizontal">
                <div>
                  <p className="eyebrow eyebrow--subtle">消息历史</p>
                  <h3>会议期间的全部消息</h3>
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
                <p className="sidebar-empty">正在加载会议消息...</p>
              ) : messages.length === 0 ? (
                <p className="sidebar-empty">这个会议还没有消息记录。</p>
              ) : (
                <div className="room-history-list">
                  <MessageList currentParticipantName="" messages={messages} />
                </div>
              )}
            </section>
          </div>
        </section>
      </div>

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
