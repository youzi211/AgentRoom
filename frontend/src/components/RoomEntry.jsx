import { useState } from 'react'

function RoomEntry({ errorMessage, initialPasscode = '', isSubmitting, roomId, onBackHome, onJoinRoom }) {
  const [displayName, setDisplayName] = useState('')
  const [passcode, setPasscode] = useState(initialPasscode)
  const trimmedDisplayName = displayName.trim()
  const trimmedPasscode = passcode.trim()

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (!trimmedDisplayName) {
      return
    }

    await onJoinRoom({
      displayName: trimmedDisplayName,
      roomId,
      passcode: trimmedPasscode,
    })
  }

  return (
    <main className="workbench workbench--center">
      <section className="panel direct-entry-panel">
        <div className="panel-header panel-header--horizontal">
          <div>
            <p className="eyebrow">加入会议室</p>
            <h1>输入昵称后进入房间</h1>
            <p className="section-copy">
              这是一个可分享的会议链接。为了让成员和 Agent 正确识别你的发言，请先填写本次会议里的显示名称。
            </p>
          </div>
          <button className="button button--secondary" type="button" onClick={onBackHome}>
            返回入口
          </button>
        </div>

        <form className="form-stack room-entry-form" onSubmit={handleSubmit}>
          <div className="field-group">
            <label htmlFor="direct-room-id">房间 ID</label>
            <input id="direct-room-id" type="text" value={roomId} readOnly />
          </div>

          <div className="field-group">
            <label htmlFor="direct-display-name">显示名称</label>
            <input
              id="direct-display-name"
              autoFocus
              type="text"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              placeholder="例如：小明"
              disabled={isSubmitting}
              maxLength={40}
            />
            <p className="field-hint">这个名称会显示在会议消息和成员列表中。</p>
          </div>

          <div className="field-group">
            <label htmlFor="direct-passcode">房间口令</label>
            <input
              id="direct-passcode"
              type="password"
              value={passcode}
              onChange={(event) => setPasscode(event.target.value)}
              placeholder="如果房间设置了口令，请在这里输入"
              disabled={isSubmitting}
              maxLength={80}
            />
            <p className="field-hint">没有口令的房间可以留空。</p>
          </div>

          <div className="button-row">
            <span className="helper-text">加入后会加载房间已有消息。</span>
            <button className="button button--primary" type="submit" disabled={isSubmitting || !trimmedDisplayName}>
              {isSubmitting ? '正在加入...' : '进入会议室'}
            </button>
          </div>
        </form>

        {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
      </section>
    </main>
  )
}

export default RoomEntry
