import { useState } from 'react'

function JoinScreen({ errorMessage, isSubmitting, onCreateRoom, onJoinRoom }) {
  const [displayName, setDisplayName] = useState('')
  const [roomName, setRoomName] = useState('')
  const [roomId, setRoomId] = useState('')

  const trimmedDisplayName = displayName.trim()
  const trimmedRoomName = roomName.trim()
  const trimmedRoomId = roomId.trim()

  const handleCreateRoom = async (event) => {
    event.preventDefault()
    if (!trimmedDisplayName) {
      return
    }

    await onCreateRoom({
      displayName: trimmedDisplayName,
      roomName: trimmedRoomName,
    })
  }

  const handleJoinRoom = async (event) => {
    event.preventDefault()
    if (!trimmedDisplayName || !trimmedRoomId) {
      return
    }

    await onJoinRoom({
      displayName: trimmedDisplayName,
      roomId: trimmedRoomId,
    })
  }

  return (
    <main className="join-screen">
      <section className="join-card">
        <p className="eyebrow">AgentRoom</p>
        <h1>Join a room and talk with your team plus built-in agents.</h1>
        <p className="helper-text">
          Create a shared room, invite teammates in another window, and mention agents directly in chat.
        </p>

        <div className="join-grid">
          <form className="panel" onSubmit={handleCreateRoom}>
            <h2>Create a room</h2>
            <div className="field-group">
              <label htmlFor="create-display-name">Display name</label>
              <input
                id="create-display-name"
                autoFocus
                type="text"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="e.g. Alice"
                disabled={isSubmitting}
                maxLength={40}
              />
            </div>

            <div className="field-group">
              <label htmlFor="room-name">Room name</label>
              <input
                id="room-name"
                type="text"
                value={roomName}
                onChange={(event) => setRoomName(event.target.value)}
                placeholder="Leave blank for automatic room name"
                disabled={isSubmitting}
                maxLength={60}
              />
            </div>

            <div className="button-row">
              <button className="button button--primary" type="submit" disabled={isSubmitting || !trimmedDisplayName}>
                {isSubmitting ? 'Creating…' : 'Create room'}
              </button>
            </div>
          </form>

          <form className="panel" onSubmit={handleJoinRoom}>
            <h2>Join an existing room</h2>
            <div className="field-group">
              <label htmlFor="join-display-name">Display name</label>
              <input
                id="join-display-name"
                type="text"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="e.g. Alice"
                disabled={isSubmitting}
                maxLength={40}
              />
            </div>

            <div className="field-group">
              <label htmlFor="room-id">Room ID</label>
              <input
                id="room-id"
                type="text"
                value={roomId}
                onChange={(event) => setRoomId(event.target.value)}
                placeholder="room_123456"
                disabled={isSubmitting}
                maxLength={80}
              />
            </div>

            <div className="button-row">
              <button
                className="button button--secondary"
                type="submit"
                disabled={isSubmitting || !trimmedDisplayName || !trimmedRoomId}
              >
                {isSubmitting ? 'Joining…' : 'Join room'}
              </button>
            </div>
          </form>
        </div>

        {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      </section>
    </main>
  )
}

export default JoinScreen
