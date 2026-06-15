import { useCallback, useEffect, useState } from 'react'
import { archiveRoom, listRooms, restoreRoom } from '../api/roomClient'
import MinutesHistory from './MinutesHistory'

const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: 'active', label: '进行中' },
  { value: 'archived', label: '已归档' },
]

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
  const [minutesRoom, setMinutesRoom] = useState(null)

  const loadRooms = useCallback(async (status) => {
    setIsLoading(true)
    setErrorMessage('')
    try {
      const response = await listRooms({ status })
      setRooms(response.rooms ?? [])
    } catch (error) {
      setErrorMessage(error.message || '加载会议列表失败。')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadRooms(statusFilter)
  }, [loadRooms, statusFilter])

  const handleToggleArchive = async (roomItem) => {
    setBusyRoomId(roomItem.id)
    setErrorMessage('')
    try {
      if (roomItem.status === 'archived') {
        await restoreRoom(roomItem.id)
      } else {
        await archiveRoom(roomItem.id)
      }
      await loadRooms(statusFilter)
    } catch (error) {
      setErrorMessage(error.message || '更新会议状态失败。')
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
          <p className="section-copy">浏览历史会议、查看与编辑会议纪要、归档不再活跃的房间。归档后的房间将变为只读，不再接受新发言。</p>
        </div>
      </section>

      {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}

      <div className="panel">
        <div className="panel-header panel-header--horizontal">
          <div className="panel-title-row">
            <h2>会议列表</h2>
            <span className="panel-badge panel-badge--neutral">{rooms.length}</span>
          </div>
          <div className="agent-select-actions" role="tablist" aria-label="状态筛选">
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
                  <span className={`agent-state ${roomItem.status === 'archived' ? 'agent-state--off' : ''}`}>
                    {roomItem.status === 'archived' ? '已归档' : '进行中'}
                  </span>
                </span>
                <span role="cell">{roomItem.messageCount ?? 0}</span>
                <span role="cell">{formatDateTime(roomItem.createdAt)}</span>
                <span role="cell">{formatDateTime(roomItem.lastMessageAt)}</span>
                <span className="meeting-cell-actions" role="cell">
                  <button className="button button--ghost button--compact" type="button" onClick={() => setMinutesRoom(roomItem)}>
                    纪要
                  </button>
                  <button
                    className="button button--secondary button--compact"
                    type="button"
                    onClick={() => handleToggleArchive(roomItem)}
                    disabled={busyRoomId === roomItem.id}
                  >
                    {roomItem.status === 'archived' ? '恢复' : '归档'}
                  </button>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {minutesRoom ? <MinutesHistory room={minutesRoom} onClose={() => setMinutesRoom(null)} /> : null}
    </section>
  )
}

export default MeetingAdmin
