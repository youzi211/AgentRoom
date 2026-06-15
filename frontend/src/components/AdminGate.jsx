import { useEffect, useState } from 'react'
import { clearStoredAdminKey, getStoredAdminKey, setStoredAdminKey, verifyAdminKey } from '../api/roomClient'

// AdminGate guards the admin console. It verifies the stored admin key against
// the backend before rendering children. When the backend has no ADMIN_API_KEY
// configured, /admin/verify succeeds for any key and the gate passes through.
function AdminGate({ children, onBackHome }) {
  const [status, setStatus] = useState('checking')
  const [keyInput, setKeyInput] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let isCurrent = true

    const check = async () => {
      if (!getStoredAdminKey()) {
        if (isCurrent) {
          setStatus('locked')
        }
        return
      }
      try {
        await verifyAdminKey()
        if (isCurrent) {
          setStatus('authed')
        }
      } catch {
        clearStoredAdminKey()
        if (isCurrent) {
          setStatus('locked')
        }
      }
    }

    void check()
    return () => {
      isCurrent = false
    }
  }, [])

  const handleSubmit = async (event) => {
    event.preventDefault()
    setErrorMessage('')
    setStoredAdminKey(keyInput)
    try {
      await verifyAdminKey()
      setStatus('authed')
    } catch (error) {
      clearStoredAdminKey()
      setErrorMessage(error.message || '管理密钥校验失败，请检查后重试。')
    }
  }

  if (status === 'authed') {
    return children
  }

  return (
    <main className="workbench workbench--entry">
      <header className="app-bar">
        <div className="brand-lockup">
          <span className="brand-mark">AR</span>
          <div>
            <strong>管理后台</strong>
            <span>会议记录与 Agent 管理</span>
          </div>
        </div>
        <nav className="app-nav" aria-label="主导航">
          <button className="app-nav-item" type="button" onClick={onBackHome}>
            会议入口
          </button>
        </nav>
      </header>

      <section className="entry-grid entry-grid--single">
        <form className="panel panel--primary-flow" onSubmit={handleSubmit}>
          <div className="panel-header">
            <h2>进入管理后台</h2>
            <p className="panel-copy">输入管理密钥（ADMIN_API_KEY）以管理会议记录、归档房间和 Agent 配置。</p>
          </div>

          {errorMessage ? <p className="banner banner--error">{errorMessage}</p> : null}

          <div className="form-stack">
            <div className="field-group">
              <label htmlFor="admin-key">管理密钥</label>
              <input
                id="admin-key"
                autoFocus
                type="password"
                value={keyInput}
                onChange={(event) => setKeyInput(event.target.value)}
                placeholder="ADMIN_API_KEY"
                disabled={status === 'checking'}
                maxLength={200}
              />
              <p className="field-hint">密钥仅保存在本浏览器，用于随请求发送，不会写入构建产物。</p>
            </div>
          </div>

          <div className="button-row button-row--stack-end">
            <span className="helper-text">{status === 'checking' ? '正在校验已保存的密钥...' : '校验通过后进入后台。'}</span>
            <button className="button button--primary" type="submit" disabled={status === 'checking'}>
              进入后台
            </button>
          </div>
        </form>
      </section>
    </main>
  )
}

export default AdminGate
