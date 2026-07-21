import { useEffect, useState } from 'react'
import { Alert, Button, Group, Paper, PasswordInput, Stack, Text, Title } from '@mantine/core'
import { clearStoredAdminKey, getStoredAdminKey, setStoredAdminKey, verifyAdminKey } from '../api/roomClient'
import { LogIn, Settings, Sparkles } from 'lucide-react'

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
    <main className="entry-dashboard admin-dashboard">
      <header className="entry-dashboard-header admin-dashboard-header">
        <div className="entry-dashboard-brand admin-dashboard-brand">
          <span className="entry-brand-symbol" aria-hidden="true">
            <Sparkles size={24} />
          </span>
          <strong>AgentRoom</strong>
        </div>
        <nav className="entry-dashboard-nav admin-dashboard-nav" aria-label="主导航">
          <span className="entry-dashboard-nav-item entry-dashboard-nav-item--active">
            <Settings size={17} />
            管理后台
          </span>
          <Button className="entry-dashboard-nav-item" type="button" variant="subtle" color="gray" leftSection={<LogIn size={17} />} onClick={onBackHome}>
            会议入口
          </Button>
        </nav>
      </header>

      <section className="admin-gate-layout">
        <Paper component="form" className="panel panel--primary-flow" withBorder radius="md" shadow="xs" onSubmit={handleSubmit}>
          <Stack gap="md">
            <div className="panel-header">
              <Title order={2}>进入管理后台</Title>
              <Text className="panel-copy">输入管理密钥（ADMIN_API_KEY）以管理会议记录、归档房间和 Agent 配置。</Text>
            </div>

            {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}

            <PasswordInput
                id="admin-key"
                label="管理密钥"
                description="密钥仅保存在本浏览器，用于随请求发送，不会写入构建产物。"
                autoFocus
                value={keyInput}
                onChange={(event) => setKeyInput(event.target.value)}
                placeholder="ADMIN_API_KEY"
                disabled={status === 'checking'}
                maxLength={200}
              />

            <Group justify="space-between" align="center">
              <Text className="helper-text">{status === 'checking' ? '正在校验已保存的密钥...' : '校验通过后进入后台。'}</Text>
              <Button color="teal" type="submit" disabled={status === 'checking'}>
              进入后台
              </Button>
            </Group>
          </Stack>
        </Paper>
      </section>
    </main>
  )
}

export default AdminGate
