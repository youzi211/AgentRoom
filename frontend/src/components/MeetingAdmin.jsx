import { useCallback, useEffect, useState } from 'react'
import { archiveRoom, listRooms, reopenRoom, restoreRoom } from '../api/roomClient'
import MeetingRoomDetail from './MeetingRoomDetail'
import {
  actionsForRoomStatus,
  labelForRoomAction,
  labelForRoomStatus,
  STATUS_FILTERS,
  toneForRoomStatus,
} from './meetingAdminModel'

function formatDateTime(value) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
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

function MeetingAdmin() {
  const [rooms, setRooms] = useState([])
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [busyRoomId, setBusyRoomId] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [selectedRoom, setSelectedRoom] = useState(null)

  const loadRooms = useCallback(async (status) => {
    setIsLoading(true)
    setErrorMessage('')
    try {
      const response = await listRooms({ status })
      setRooms(response.rooms ?? [])
    } catch (error) {
      setErrorMessage(error.message || '加载会议列表失败，请稍后重试。')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadRooms(statusFilter)
  }, [loadRooms, statusFilter])

  const handleRoomAction = async (roomItem, action) => {
    if (!roomItem?.id) {
      return
    }
    if (action === 'detail') {
      setSelectedRoom(roomItem)
      return
    }

    setBusyRoomId(roomItem.id)
    setErrorMessage('')
    try {
      let response = null

      if (action === 'archive') {
        response = await archiveRoom(roomItem.id)
      } else if (action === 'reopen') {
        response = await reopenRoom(roomItem.id)
      } else if (action === 'restore') {
        response = await restoreRoom(roomItem.id)
      }

      if (response?.room) {
        setSelectedRoom((current) => (current?.id === roomItem.id ? response.room : current))
      }

      await loadRooms(statusFilter)
    } catch (error) {
      setErrorMessage(error.message || '更新会议状态失败，请稍后重试。')
    } finally {
      setBusyRoomId('')
    }
  }

  const handleCopyId = (roomId) => {
    try {
      void navigator.clipboard?.writeText(roomId)
    } catch {
      // Clipboard may be unavailable; ignore.
    }
  }

  return (
    <section className="admin-section">
      <section className="admin-hero">
        <div>
          <p className="eyebrow">会议记录</p>
          <h1>管理所有会议与纪要</h1>
          <p className="section-copy">
            浏览进行中、已关闭和已归档的会议，查看全部消息历史，并按新的生命周期语义执行归档、恢复会议或取消归档。
          </p>
        </div>
      </section>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}

      <div className="panel">
        <div className="panel-header panel-header--horizontal">
          <div className="panel-title-row">
            <h2>会议列表</h2>
            <span className="panel-badge panel-badge--neutral">{rooms.length}</span>
          </div>
          <div className="agent-select-actions" role="tablist" aria-label="会议状态筛选">
            {STATUS_FILTERS.map((filter) => (
              <button
                key={filter.value || 'all'}
                type="button"
                className={`button button--ghost button--compact${statusFilter === filter.value ? ' button--active' : ''}`}
                onClick={() => setStatusFilter(filter.value)}
              >
                {filter.label}
              </button>
            ))}
          </div>
        </div>

        {isLoading ? (
          <p className="sidebar-empty">正在加载...</p>
        ) : rooms.length === 0 ? (
          <p className="sidebar-empty">暂无会议记录。</p>
        ) : (
          <div className="meeting-table" role="table">
            <div className="meeting-table-head" role="row">
              <span role="columnheader">会议名称</span>
              <span role="columnheader">状态</span>
              <span role="columnheader">消息数</span>
              <span role="columnheader">创建时间</span>
              <span role="columnheader">最近消息</span>
              <span role="columnheader">操作</span>
            </div>
            {rooms.map((roomItem) => (
              <div className="meeting-table-row" role="row" key={roomItem.id}>
                <span className="meeting-cell-name" role="cell">
                  <strong>{roomItem.name}</strong>
                  <button type="button" className="meeting-room-id" title="复制房间 ID" onClick={() => handleCopyId(roomItem.id)}>
                    {roomItem.id}
                  </button>
                </span>
                <span role="cell">
                  <span className={`agent-state agent-state--${toneForRoomStatus(roomItem.status)}`}>
                    {labelForRoomStatus(roomItem.status)}
                  </span>
                </span>
                <span role="cell">{roomItem.messageCount ?? 0}</span>
                <span role="cell">{formatDateTime(roomItem.createdAt)}</span>
                <span role="cell">{formatDateTime(roomItem.lastMessageAt)}</span>
                <span className="meeting-cell-actions" role="cell">
                  {actionsForRoomStatus(roomItem.status).map((action) => (
                    <button
                      key={`${roomItem.id}:${action}`}
                      className={`button ${action === 'detail' ? 'button--ghost' : 'button--secondary'} button--compact`}
                      type="button"
                      onClick={() => handleRoomAction(roomItem, action)}
                      disabled={action !== 'detail' && busyRoomId === roomItem.id}
                    >
                      {labelForRoomAction(action)}
                    </button>
                  ))}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {selectedRoom ? (
        <MeetingRoomDetail
          busyRoomId={busyRoomId}
          onClose={() => setSelectedRoom(null)}
          onRoomAction={handleRoomAction}
          room={selectedRoom}
        />
      ) : null}
    </section>
  )
}

export default MeetingAdmin
