import { useEffect, useMemo, useState } from 'react'
import { Alert, Badge, Button, Group, Modal, Paper, PasswordInput, Select, Stack, Switch, Text, TextInput, Title } from '@mantine/core'
import { Ban, CheckCircle2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import {
  createModelProfile,
  deleteModelProfile,
  disableModelProfile,
  listModelProfiles,
  setDefaultModelProfile,
  testDraftModelProfile,
  testSavedModelProfile,
  updateModelProfile,
} from '../api/roomClient'
import {
  createEmptyModelProfile,
  createModelProfilePayload,
  draftModelProfileTestPayload,
  updateModelProfilePayload,
  usesSavedProfileForTest,
} from './modelProfileForm'

export default function ModelProfileAdmin() {
  const [profiles, setProfiles] = useState([])
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(() => createEmptyModelProfile())
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [testResult, setTestResult] = useState(null)
  const [profileTestResults, setProfileTestResults] = useState({})

  const groups = useMemo(() => [
    { scope: 'go', title: 'Go 模型', description: '供 Go Agent、焦点提取和会议纪要使用。' },
    { scope: 'deepagent', title: 'DeepAgent 模型', description: '运行时仅注入单次 Python 子进程环境。' },
  ], [])

  const reload = async () => {
    setBusy(true)
    try { const response = await listModelProfiles(); setProfiles(response.profiles || []); setError('') }
    catch (requestError) { setError(requestError.message || '加载模型配置失败') }
    finally { setBusy(false) }
  }

  useEffect(() => { void reload() }, [])

  const openCreate = (scope = 'go') => { setEditing('new'); setForm(createEmptyModelProfile(scope)); setTestResult(null); setError('') }
  const openEdit = (profile) => { setEditing(profile.id); setForm({ ...createEmptyModelProfile(profile.runtimeScope), ...profile, apiKey: '', clearAPIKey: false }); setTestResult(null); setError('') }
  const close = () => { if (!busy) setEditing(null) }

  const save = async (event) => {
    event.preventDefault()
    if (editing !== 'new' && form.hasAPIKey && form.apiKey && !window.confirm('保存后将替换现有 API Key。确定继续吗？')) return
    if (editing !== 'new' && form.clearAPIKey && !window.confirm('清除 API Key 后，该 Profile 可能无法连接模型。确定继续吗？')) return
    if (editing !== 'new' && form.enabled === false && !window.confirm('停用后，新房间不能再选择该 Profile；已有显式快照调用也会失败。确定继续吗？')) return
    setBusy(true); setError(''); setNotice('')
    try {
      if (editing === 'new') {
        await createModelProfile(createModelProfilePayload(form))
        setNotice('模型 Profile 已创建。')
      } else {
        await updateModelProfile(editing, updateModelProfilePayload(form))
        setNotice(form.clearAPIKey
          ? '模型 Profile 已保存，API Key 已清除。'
          : form.hasAPIKey && form.apiKey
            ? '模型 Profile 已保存，API Key 已替换。'
            : '模型 Profile 已保存，现有 API Key 保持不变。')
      }
      setEditing(null); await reload()
    } catch (requestError) { setError(requestError.message || '保存模型 Profile 失败') }
    finally { setBusy(false) }
  }

  const testConnection = async () => {
    setBusy(true); setTestResult({ pending: true }); setError('')
    try {
      const testsSavedVersion = usesSavedProfileForTest(editing, form)
      const result = testsSavedVersion
        ? await testSavedModelProfile(editing)
        : await testDraftModelProfile(draftModelProfileTestPayload(form))
      setTestResult({ ...result, testsSavedVersion })
    } catch (requestError) { setTestResult({ ok: false, error: requestError.message || '连接测试失败' }) }
    finally { setBusy(false) }
  }

  const testSaved = async (profile) => {
    setProfileTestResults((current) => ({ ...current, [profile.id]: { pending: true } }))
    try {
      const result = await testSavedModelProfile(profile.id)
      setProfileTestResults((current) => ({ ...current, [profile.id]: result }))
    } catch (requestError) {
      setProfileTestResults((current) => ({ ...current, [profile.id]: { ok: false, error: requestError.message || '连接测试失败' } }))
    }
  }

  const makeDefault = async (profile) => {
    if (!window.confirm(`确定将“${profile.name}”设为 ${profile.runtimeScope} Runtime 的默认模型吗？`)) return
    setBusy(true); setError(''); setNotice('')
    try { await setDefaultModelProfile(profile.id); setNotice(`${profile.name} 已设为 ${profile.runtimeScope} 默认模型。`); await reload() }
    catch (requestError) { setError(requestError.message || '设置默认模型失败。') }
    finally { setBusy(false) }
  }

  const disable = async (profile) => {
    if (!window.confirm(`确定停用“${profile.name}”吗？已有房间若显式引用它，后续调用会失败。`)) return
    setBusy(true); setError(''); setNotice('')
    try { await disableModelProfile(profile.id); setNotice(`${profile.name} 已停用。`); await reload() }
    catch (requestError) { setError(requestError.message || '停用失败。若它是当前默认模型，请先设置替代默认值。') }
    finally { setBusy(false) }
  }

  const remove = async (profile) => {
    if (!window.confirm(`确定删除模型 Profile“${profile.name}”吗？此操作不可撤销。`)) return
    setBusy(true); setError(''); setNotice('')
    try { await deleteModelProfile(profile.id); setNotice('模型 Profile 已删除。'); await reload() }
    catch (requestError) {
      const detail = requestError.message || ''
      setError(detail.includes('referenced')
        ? '该 Profile 正被默认槽位、全局 Agent 或房间快照引用，无法删除。请先解除引用；历史房间快照引用不能自动清除。'
        : detail || '删除模型 Profile 失败。')
    } finally { setBusy(false) }
  }

  return (
    <section className="admin-section">
      <section className="admin-hero">
        <div><Text className="eyebrow">模型配置</Text><Title order={1}>统一管理 Go 与 DeepAgent 模型</Title><Text className="section-copy">支持多个 OpenAI-compatible Chat Completions Profile。密钥保存后不可读回；连接测试会产生一次真实模型请求。</Text></div>
        <Button color="teal" leftSection={<Plus size={17} />} onClick={() => openCreate('go')}>新增 Profile</Button>
      </section>
      <Stack gap="sm">{error ? <Alert color="red">{error}</Alert> : null}{notice ? <Alert color="teal">{notice}</Alert> : null}</Stack>
      <Stack gap="lg">
        {groups.map((group) => {
          const scoped = profiles.filter((profile) => profile.runtimeScope === group.scope)
          return <Paper key={group.scope} className="panel" withBorder radius="md" p="lg">
            <Group justify="space-between" mb="md"><div><Title order={2}>{group.title}</Title><Text c="dimmed">{group.description}</Text></div><Button variant="light" onClick={() => openCreate(group.scope)}>新增</Button></Group>
            {scoped.length === 0 ? <Alert color="blue">尚无数据库默认 Profile；当前调用会使用非敏感的环境变量迁移兜底（如已配置）。页面不会读取或展示环境中的密钥。</Alert> : <Stack gap="sm">{scoped.map((profile) => <Paper key={profile.id} withBorder p="md" radius="md">
              <Group justify="space-between" align="flex-start"><div><Group gap="xs"><Text fw={700}>{profile.name}</Text>{profile.isDefault ? <Badge color="teal">默认</Badge> : null}<Badge color={profile.enabled ? 'blue' : 'gray'}>{profile.enabled ? '启用' : '停用'}</Badge><Badge color={profile.hasAPIKey ? 'green' : 'yellow'}>{profile.hasAPIKey ? `密钥已配置 ${profile.apiKeyHint || ''}` : '无密钥'}</Badge></Group><Text size="sm" mt="xs">{profile.modelName}</Text><Text size="xs" c="dimmed">{profile.baseURL}</Text></div>
                <Group gap="xs"><Button size="xs" variant="default" onClick={() => openEdit(profile)} disabled={busy}>编辑</Button><Button size="xs" variant="light" leftSection={<RefreshCw size={14} />} onClick={() => testSaved(profile)} loading={profileTestResults[profile.id]?.pending} disabled={busy || !profile.enabled}>测试</Button>{!profile.isDefault && profile.enabled ? <Button size="xs" variant="light" leftSection={<CheckCircle2 size={14} />} onClick={() => makeDefault(profile)} disabled={busy}>设为默认</Button> : null}{profile.enabled && !profile.isDefault ? <Button size="xs" color="orange" variant="subtle" leftSection={<Ban size={14} />} onClick={() => disable(profile)} disabled={busy}>停用</Button> : null}<Button size="xs" color="red" variant="subtle" leftSection={<Trash2 size={14} />} onClick={() => remove(profile)} disabled={busy}>删除</Button></Group>
              </Group>
              {profileTestResults[profile.id] && !profileTestResults[profile.id].pending ? <Alert mt="sm" color={profileTestResults[profile.id].ok ? 'teal' : 'red'}>{profileTestResults[profile.id].ok ? `连接成功，延迟 ${profileTestResults[profile.id].latencyMS}ms${profileTestResults[profile.id].model ? `，响应模型 ${profileTestResults[profile.id].model}` : ''}` : profileTestResults[profile.id].error || '连接测试失败'}</Alert> : null}
            </Paper>)}</Stack>}
          </Paper>
        })}
      </Stack>
      <Modal opened={editing !== null} onClose={close} title={editing === 'new' ? '新增模型 Profile' : '编辑模型 Profile'} size="lg">
        <form onSubmit={save}><Stack>
          <TextInput label="名称" required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
          <Select label="Runtime" disabled={editing !== 'new'} data={[{ value: 'go', label: 'Go' }, { value: 'deepagent', label: 'DeepAgent' }]} value={form.runtimeScope} onChange={(value) => setForm({ ...form, runtimeScope: value || 'go' })} />
          <TextInput label="API Base URL" description="保存时规范化为以 /v1 结尾，不要填写 /chat/completions。" required value={form.baseURL} onChange={(event) => setForm({ ...form, baseURL: event.target.value })} />
          <TextInput label="模型 ID" required value={form.modelName} onChange={(event) => setForm({ ...form, modelName: event.target.value })} />
          <PasswordInput label="API Key" description={editing !== 'new' && form.hasAPIKey ? `留空保留现有密钥 ${form.apiKeyHint || ''}` : '密钥将使用服务端主密钥加密保存'} value={form.apiKey} onChange={(event) => setForm({ ...form, apiKey: event.target.value, clearAPIKey: false })} />
          {editing !== 'new' && form.hasAPIKey ? <Switch label="清除已保存的 API Key" checked={form.clearAPIKey} onChange={(event) => setForm({ ...form, clearAPIKey: event.currentTarget.checked, apiKey: '' })} /> : null}
          <Switch label="启用" description={form.isDefault && editing !== 'new' ? '当前默认 Profile 不能直接停用；请先设置替代默认值。' : undefined} checked={form.enabled} disabled={Boolean(form.isDefault && editing !== 'new')} onChange={(event) => setForm({ ...form, enabled: event.currentTarget.checked, isDefault: event.currentTarget.checked ? form.isDefault : false })} />
          {editing === 'new' ? <Switch label={`创建后设为 ${form.runtimeScope} 默认模型`} checked={form.isDefault} onChange={(event) => setForm({ ...form, isDefault: event.currentTarget.checked, enabled: event.currentTarget.checked ? true : form.enabled })} /> : null}
          {testResult ? <Alert color={testResult.pending ? 'blue' : testResult.ok ? 'teal' : 'red'}>{testResult.pending ? '正在测试连接…' : testResult.ok ? `连接成功，延迟 ${testResult.latencyMS}ms${testResult.model ? `，响应模型 ${testResult.model}` : ''}` : testResult.error}</Alert> : null}
          {testResult?.testsSavedVersion ? <Alert color="blue">本次使用服务端已保存的 Base URL、模型 ID 与密钥进行测试。若要测试未保存的 URL 或模型变更，请同时输入用于测试的新 API Key，或先保存变更。</Alert> : null}
          <Text size="xs" c="dimmed">测试连接会向配置的模型发送最小真实请求，可能产生少量费用。</Text>
          <Group justify="space-between"><Button type="button" variant="light" onClick={testConnection} loading={busy}>测试连接</Button><Group><Button type="button" variant="default" onClick={close}>取消</Button><Button type="submit" color="teal" loading={busy}>保存</Button></Group></Group>
        </Stack></form>
      </Modal>
    </section>
  )
}
