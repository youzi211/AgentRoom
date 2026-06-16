import { useEffect, useState } from 'react'
import {
  exportRoomMinutesMarkdown,
  generateRoomMinutes,
  getMinutesHistory,
  saveRoomMinutes,
} from '../api/roomClient'
import { downloadMarkdownFile, minutesFilename, normalizeMinutesPayload } from './meetingMinutes'

function sourceLabel(source) {
  return source === 'manual' ? '手动编辑' : 'AI 生成'
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

// MinutesHistory is an admin modal that lists persisted minutes versions for a
// room, regenerates an AI version, and saves manual edits as a new version.
function MinutesHistory({ room, onClose }) {
  const [versions, setVersions] = useState([])
  const [selectedId, setSelectedId] = useState('')
  const [draft, setDraft] = useState('')
  const [status, setStatus] = useState('loading')
  const [notice, setNotice] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

  const loadHistory = async (preferLatest = false) => {
    setErrorMessage('')
    try {
      const response = await getMinutesHistory(room.id)
      const list = response.minutes ?? []
      setVersions(list)
      if (list.length > 0) {
        const next = preferLatest ? list[0] : list.find((item) => item.id === selectedId) ?? list[0]
        setSelectedId(next.id)
        setDraft(next.content ?? '')
      } else {
        setSelectedId('')
        setDraft('')
      }
      setStatus('idle')
    } catch (error) {
      setErrorMessage(error.message || '加载纪要历史失败。')
      setStatus('idle')
    }
  }

  useEffect(() => {
    void loadHistory(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [room.id])

  const handleSelectVersion = (version) => {
    setSelectedId(version.id)
    setDraft(version.content ?? '')
    setNotice('')
  }

  const handleGenerate = async () => {
    setStatus('generating')
    setNotice('')
    setErrorMessage('')
    try {
      await generateRoomMinutes(room.id)
      await loadHistory(true)
      setNotice('已生成新的 AI 纪要版本。')
    } catch (error) {
      setErrorMessage(error.message || '生成纪要失败。')
      setStatus('idle')
    }
  }

  const handleSave = async () => {
    if (!draft.trim()) {
      setErrorMessage('纪要内容不能为空。')
      return
    }
    setStatus('saving')
    setNotice('')
    setErrorMessage('')
    try {
      await saveRoomMinutes(room.id, draft.trim())
      await loadHistory(true)
      setNotice('已保存为新的手动版本。')
    } catch (error) {
      setErrorMessage(error.message || '保存纪要失败。')
      setStatus('idle')
    }
  }

  const handleExport = async () => {
    try {
      const markdown = normalizeMinutesPayload(await exportRoomMinutesMarkdown(room.id)) || draft
      downloadMarkdownFile(markdown, minutesFilename(room, room.id))
      setNotice('Markdown 已开始下载。')
    } catch (error) {
      downloadMarkdownFile(draft, minutesFilename(room, room.id))
      setNotice(`${error.message || '后端导出接口暂时不可用。'} 已导出当前编辑内容。`)
    }
  }

  const busy = status === 'generating' || status === 'saving'

  return (
    <div className="delete-confirm-overlay delete-confirm-overlay--scrollable" role="dialog" aria-modal="true">
      <div className="minutes-history-card">
        <div className="panel-header panel-header--horizontal">
          <div>
            <h2>会议纪要 · {room.name}</h2>
            <p className="panel-copy">查看历史版本、重新生成 AI 纪要，或编辑后保存为新版本。</p>
          </div>
          <button className="button button--ghost button--compact" type="button" onClick={onClose}>
            关闭
          </button>
        </div>

        {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}
        {notice ? <p className="banner banner--success">{notice}</p> : null}

        <div className="minutes-history-body">
          <aside className="minutes-history-versions">
            <div className="panel-title-row">
              <h3>版本</h3>
              <span className="panel-badge panel-badge--neutral">{versions.length}</span>
            </div>
            {status === 'loading' ? (
              <p className="sidebar-empty">加载中...</p>
            ) : versions.length === 0 ? (
              <p className="sidebar-empty">暂无纪要。点击「生成纪要」创建第一版。</p>
            ) : (
              <ul className="minutes-version-list">
                {versions.map((version) => (
                  <li key={version.id}>
                    <button
                      type="button"
                      className={`minutes-version-button${version.id === selectedId ? ' minutes-version-button--active' : ''}`}
                      onClick={() => handleSelectVersion(version)}
                    >
                      <span className="minutes-version-title">v{version.version} · {sourceLabel(version.source)}</span>
                      <span className="minutes-version-time">{formatDateTime(version.createdAt)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </aside>

          <div className="minutes-history-editor">
            <textarea
              className="text-input text-input--prompt"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              rows={16}
              placeholder="纪要 Markdown 内容..."
              disabled={busy}
            />
            <div className="button-row">
              <button className="button button--secondary button--compact" type="button" onClick={handleGenerate} disabled={busy}>
                {status === 'generating' ? '生成中...' : '生成纪要'}
              </button>
              <button className="button button--secondary button--compact" type="button" onClick={handleExport} disabled={busy || !draft.trim()}>
                导出 Markdown
              </button>
              <button className="button button--primary button--compact" type="button" onClick={handleSave} disabled={busy || !draft.trim()}>
                {status === 'saving' ? '保存中...' : '保存为新版本'}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default MinutesHistory
