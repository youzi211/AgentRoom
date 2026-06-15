import { useMemo, useState } from 'react'
import { buildLocalMeetingMinutesMarkdown, downloadMarkdownFile, minutesFilename, normalizeMinutesPayload } from './meetingMinutes'

function MeetingMinutesPanel({
  agents,
  exportMinutesMarkdown,
  focusPoints,
  generateMinutes,
  messages,
  participants,
  room,
  roomId,
}) {
  const [minutesMarkdown, setMinutesMarkdown] = useState('')
  const [status, setStatus] = useState('idle')
  const [notice, setNotice] = useState('')

  const localMarkdown = useMemo(
    () => buildLocalMeetingMinutesMarkdown({ room, roomId, participants, agents, focusPoints, messages }),
    [agents, focusPoints, messages, participants, room, roomId],
  )
  const previewMarkdown = minutesMarkdown || localMarkdown
  const canExport = messages.length > 0 || focusPoints.length > 0 || Boolean(minutesMarkdown)

  const handleGenerateMinutes = async () => {
    setStatus('generating')
    setNotice('')

    try {
      const payload = await generateMinutes()
      const markdown = normalizeMinutesPayload(payload)
      if (!markdown) {
        throw new Error('后端没有返回可展示的纪要内容。')
      }

      setMinutesMarkdown(markdown)
      setNotice('已生成会议纪要，可以继续导出 Markdown。')
      setStatus('idle')
    } catch (error) {
      setMinutesMarkdown(localMarkdown)
      setNotice(`${error.message || '后端纪要接口暂时不可用。'} 已切换为本地草稿。`)
      setStatus('idle')
    }
  }

  const handleExportMarkdown = async () => {
    if (!canExport) {
      setNotice('暂时没有可导出的会议内容。')
      return
    }

    setStatus('exporting')
    setNotice('')

    try {
      const markdown = normalizeMinutesPayload(await exportMinutesMarkdown()) || previewMarkdown
      downloadMarkdownFile(markdown, minutesFilename(room, roomId))
      setNotice('Markdown 已开始下载。')
      setStatus('idle')
    } catch (error) {
      downloadMarkdownFile(previewMarkdown, minutesFilename(room, roomId))
      setNotice(`${error.message || '后端导出接口暂时不可用。'} 已导出本地草稿。`)
      setStatus('idle')
    }
  }

  return (
    <section className="sidebar-section minutes-panel">
      <div className="sidebar-header">
        <h2>会议产物</h2>
        <span className="sidebar-count">MD</span>
      </div>
      <p className="sidebar-note">生成会议纪要，并导出 Markdown 文件用于归档或继续编辑。</p>
      <div className="minutes-actions">
        <button className="button button--secondary button--compact" type="button" onClick={handleGenerateMinutes} disabled={status !== 'idle'}>
          {status === 'generating' ? '生成中...' : '生成纪要'}
        </button>
        <button
          className="button button--primary button--compact"
          type="button"
          onClick={handleExportMarkdown}
          disabled={status !== 'idle' || !canExport}
        >
          {status === 'exporting' ? '导出中...' : '导出 Markdown'}
        </button>
      </div>
      {notice ? <p className="minutes-notice">{notice}</p> : null}
      {previewMarkdown ? (
        <details className="minutes-preview">
          <summary>预览草稿</summary>
          <pre>{previewMarkdown}</pre>
        </details>
      ) : null}
    </section>
  )
}

export default MeetingMinutesPanel
