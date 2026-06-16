import { useEffect, useMemo, useState } from 'react'
import { getRoom } from '../api/roomClient'
import { clearRoomSession } from '../routing'
import ChatRoom from './ChatRoom'
import NotFound from './NotFound'
import RoomEntry from './RoomEntry'
import RoomReadOnly from './RoomReadOnly'
import { resolveRoomSurface, ROOM_SURFACES } from './roomAccess'

function RoomGateway({
  errorMessage,
  isSubmitting,
  onBackHome,
  onJoinRoom,
  onLeaveRoom,
  roomId,
  roomSession,
  routePasscode = '',
}) {
  const participantName = roomSession?.participantName || ''
  const roomPasscode = roomSession?.passcode || routePasscode || ''
  const [loadState, setLoadState] = useState(() => ({
    roomResponse: roomSession?.initialRoom
      ? {
          room: roomSession.initialRoom,
          participants: [],
          agents: [],
        }
      : null,
    status: 'loading',
    errorMessage: '',
  }))

  useEffect(() => {
    let isCurrent = true

    const loadRoom = async () => {
      setLoadState((current) => ({
        roomResponse: current.roomResponse?.room?.id === roomId ? current.roomResponse : null,
        status: 'loading',
        errorMessage: '',
      }))

      try {
        const response = await getRoom(roomId, roomPasscode)
        if (!isCurrent) {
          return
        }

        setLoadState({
          roomResponse: response,
          status: 'ready',
          errorMessage: '',
        })
      } catch (error) {
        if (!isCurrent) {
          return
        }

        setLoadState({
          roomResponse: null,
          status: 'error',
          errorMessage: error.message || '加载会议信息失败，请稍后重试。',
        })
      }
    }

    void loadRoom()

    return () => {
      isCurrent = false
    }
  }, [roomId, roomPasscode])

  const roomResponse = loadState.roomResponse
  const room = roomResponse?.room || null
  const surface = useMemo(
    () =>
      resolveRoomSurface({
        roomStatus: room?.status || '',
        participantName,
        hasRoomData: Boolean(room),
      }),
    [participantName, room],
  )

  useEffect(() => {
    if (room && room.status && room.status !== 'active') {
      clearRoomSession(roomId)
    }
  }, [room, roomId])

  if (loadState.status === 'loading') {
    return (
      <main className="workbench workbench--center">
        <section className="panel direct-entry-panel">
          <div className="panel-header">
            <p className="eyebrow">会议链接</p>
            <h1>正在确认会议状态</h1>
            <p className="section-copy">我们正在检查这个会议是否仍在进行中，以及当前链接是否可以直接查看历史记录。</p>
          </div>
        </section>
      </main>
    )
  }

  if (loadState.status === 'error') {
    if (isArchivedAccessError(loadState.errorMessage)) {
      return (
        <RoomAccessDenied
          title="会议已归档"
          description="这个会议已经归档，普通参会者不能再打开历史页面。"
          onBackHome={onBackHome}
        />
      )
    }

    if (isRoomNotFoundError(loadState.errorMessage)) {
      return <NotFound onBackHome={onBackHome} />
    }

    return (
      <RoomEntry
        errorMessage={loadState.errorMessage || errorMessage}
        initialPasscode={roomPasscode}
        isSubmitting={isSubmitting}
        roomId={roomId}
        onBackHome={onBackHome}
        onJoinRoom={onJoinRoom}
      />
    )
  }

  switch (surface) {
    case ROOM_SURFACES.live:
      return (
        <ChatRoom
          key={`${roomId}:${participantName}`}
          initialRoom={room}
          participantName={participantName}
          roomId={roomId}
          roomPasscode={roomPasscode}
          onLeaveRoom={onLeaveRoom}
        />
      )
    case ROOM_SURFACES.readOnly:
      return <RoomReadOnly room={room} roomPasscode={roomPasscode} onBackHome={onBackHome} />
    case ROOM_SURFACES.denied:
      return (
        <RoomAccessDenied
          title="会议已归档"
          description="这个会议已经归档，只能由管理员在会议管理界面中查看。"
          onBackHome={onBackHome}
        />
      )
    case ROOM_SURFACES.entry:
    default:
      return (
        <RoomEntry
          errorMessage={errorMessage}
          initialPasscode={roomPasscode}
          isSubmitting={isSubmitting}
          roomId={roomId}
          onBackHome={onBackHome}
          onJoinRoom={onJoinRoom}
        />
      )
  }
}

function RoomAccessDenied({ title, description, onBackHome }) {
  return (
    <main className="workbench workbench--center">
      <section className="panel direct-entry-panel">
        <div className="panel-header panel-header--horizontal">
          <div>
            <p className="eyebrow">只读限制</p>
            <h1>{title}</h1>
            <p className="section-copy">{description}</p>
          </div>
          <button className="button button--primary" type="button" onClick={onBackHome}>
            返回入口
          </button>
        </div>
      </section>
    </main>
  )
}

function isArchivedAccessError(message) {
  return String(message || '').toLowerCase().includes('archived')
}

function isRoomNotFoundError(message) {
  return String(message || '').toLowerCase().includes('not found')
}

export default RoomGateway
