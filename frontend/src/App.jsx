import { useEffect, useState } from 'react'
import { createRoom, getRoom } from './api/roomClient'
import {
  ROUTE_NAMES,
  navigateAgents,
  navigateHome,
  navigateRoom,
  parseCurrentRoute,
  resolveRoomSession,
  subscribeToNavigation,
} from './routing'
import AgentAdmin from './components/AgentAdmin'
import ChatRoom from './components/ChatRoom'
import JoinScreen from './components/JoinScreen'
import NotFound from './components/NotFound'
import RoomEntry from './components/RoomEntry'

const DEFAULT_ROOM_NAME = 'AgentRoom Meeting'

export default function App() {
  const [{ route, roomSession }, setNavigationState] = useState(() => getNavigationState())
  const [submitState, setSubmitState] = useState({
    isSubmitting: false,
    errorMessage: '',
  })

  useEffect(() => {
    return subscribeToNavigation(() => {
      setNavigationState(getNavigationState())
      setSubmitState({ isSubmitting: false, errorMessage: '' })
    })
  }, [])

  const handleCreateRoom = async ({ displayName, roomName, agentIds, passcode }) => {
    setSubmitState({ isSubmitting: true, errorMessage: '' })

    try {
      const response = await createRoom(roomName || DEFAULT_ROOM_NAME, agentIds, passcode)
      const nextRoomSession = {
        participantName: displayName,
        initialRoom: response.room,
        passcode,
      }
      setSubmitState({ isSubmitting: false, errorMessage: '' })
      navigateRoom(response.room.id, nextRoomSession)
    } catch (error) {
      setSubmitState({
        isSubmitting: false,
        errorMessage: error.message || '创建房间失败，请稍后重试。',
      })
    }
  }

  const handleJoinRoom = async ({ displayName, roomId, passcode }) => {
    setSubmitState({ isSubmitting: true, errorMessage: '' })

    try {
      const response = await getRoom(roomId, passcode)
      const nextRoomSession = {
        participantName: displayName,
        initialRoom: response.room,
        passcode,
      }
      setSubmitState({ isSubmitting: false, errorMessage: '' })
      navigateRoom(response.room.id, nextRoomSession)
    } catch (error) {
      setSubmitState({
        isSubmitting: false,
        errorMessage: normalizeJoinRoomError(error),
      })
    }
  }

  const handleLeaveRoom = () => {
    setSubmitState({ isSubmitting: false, errorMessage: '' })
    navigateHome()
  }

  if (route.name === ROUTE_NAMES.room) {
    if (!roomSession?.participantName) {
      return (
        <RoomEntry
          errorMessage={submitState.errorMessage}
          initialPasscode={roomSession?.passcode || route.passcode || ''}
          isSubmitting={submitState.isSubmitting}
          roomId={route.roomId}
          onBackHome={() => navigateHome()}
          onJoinRoom={handleJoinRoom}
        />
      )
    }

    return (
      <ChatRoom
        key={`${route.roomId}:${roomSession.participantName}`}
        initialRoom={roomSession.initialRoom ?? { id: route.roomId, name: '会议室' }}
        participantName={roomSession.participantName}
        roomId={route.roomId}
        roomPasscode={roomSession.passcode || ''}
        onLeaveRoom={handleLeaveRoom}
      />
    )
  }

  if (route.name === ROUTE_NAMES.agents) {
    return <AgentAdmin onBack={() => navigateHome()} />
  }

  if (route.name === ROUTE_NAMES.notFound) {
    return <NotFound onBackHome={() => navigateHome({ replace: true })} />
  }

  return (
    <JoinScreen
      errorMessage={submitState.errorMessage}
      isSubmitting={submitState.isSubmitting}
      onCreateRoom={handleCreateRoom}
      onJoinRoom={handleJoinRoom}
      onOpenAgentAdmin={() => navigateAgents()}
    />
  )
}

function getNavigationState() {
  const route = parseCurrentRoute()
  const roomSession =
    route.name === ROUTE_NAMES.room ? resolveRoomSession(route.roomId, route.participantName, route.passcode) : null

  return { route, roomSession }
}

function normalizeJoinRoomError(error) {
  const message = error?.message || ''
  if (message.toLowerCase().includes('room not found')) {
    return '房间不存在或已关闭，请检查房间 ID 是否完整。'
  }
  if (message.toLowerCase().includes('passcode')) {
    return '房间口令不正确，或该房间需要口令才能进入。'
  }
  return message || '加入房间失败，请稍后重试。'
}
