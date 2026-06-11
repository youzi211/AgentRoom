import { useState } from 'react'
import { createRoom, getRoom } from './api/roomClient'
import ChatRoom from './components/ChatRoom'
import JoinScreen from './components/JoinScreen'

const DEFAULT_ROOM_NAME = 'AgentRoom Meeting'

export default function App() {
  const [session, setSession] = useState(null)
  const [submitState, setSubmitState] = useState({
    isSubmitting: false,
    errorMessage: '',
  })

  const handleCreateRoom = async ({ displayName, roomName }) => {
    setSubmitState({ isSubmitting: true, errorMessage: '' })

    try {
      const response = await createRoom(roomName || DEFAULT_ROOM_NAME)
      setSession({
        participantName: displayName,
        roomId: response.room.id,
        initialRoom: response.room,
      })
      setSubmitState({ isSubmitting: false, errorMessage: '' })
    } catch (error) {
      setSubmitState({
        isSubmitting: false,
        errorMessage: error.message || 'Failed to create a room.',
      })
    }
  }

  const handleJoinRoom = async ({ displayName, roomId }) => {
    setSubmitState({ isSubmitting: true, errorMessage: '' })

    try {
      const response = await getRoom(roomId)
      setSession({
        participantName: displayName,
        roomId: response.room.id,
        initialRoom: response.room,
      })
      setSubmitState({ isSubmitting: false, errorMessage: '' })
    } catch (error) {
      setSubmitState({
        isSubmitting: false,
        errorMessage: error.message || 'Failed to join the room.',
      })
    }
  }

  const handleLeaveRoom = () => {
    setSession(null)
    setSubmitState({ isSubmitting: false, errorMessage: '' })
  }

  if (session) {
    return (
      <ChatRoom
        key={`${session.roomId}:${session.participantName}`}
        initialRoom={session.initialRoom}
        participantName={session.participantName}
        roomId={session.roomId}
        onLeaveRoom={handleLeaveRoom}
      />
    )
  }

  return (
    <JoinScreen
      errorMessage={submitState.errorMessage}
      isSubmitting={submitState.isSubmitting}
      onCreateRoom={handleCreateRoom}
      onJoinRoom={handleJoinRoom}
    />
  )
}
