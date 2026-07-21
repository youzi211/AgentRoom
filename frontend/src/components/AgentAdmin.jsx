import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Avatar,
  Badge,
  Button,
  Collapse,
  Group,
  Modal,
  Paper,
  Select,
  Stack,
  Switch,
  Text,
  Textarea,
  TextInput,
  Title,
} from '@mantine/core'
import {
  createAgent,
  deleteAgent,
  deleteKnowledgeDocument,
  getAgentKnowledge,
  getAgents,
  getAgentTemplates,
  listModelProfiles,
  updateAgent,
  uploadAgentKnowledge,
} from '../api/roomClient'
import { ChevronDown, LogIn, Plus, Settings, Sparkles, Trash2 } from 'lucide-react'
import KnowledgePanel from './KnowledgePanel'
import {
  buildAgentSavePayload,
  describeAgentModelBinding,
  profileOptionsForRuntime,
  reconcileModelProfileBinding,
  runtimeDefaultModelLabel,
} from './modelProfileSelection'

const EMPTY_FORM = {
  name: '',
  role: '',
  description: '',
  systemPrompt: '',
  enabled: true,
  runtime: 'llm',
  modelProfileID: '',
}

const CREATE_MODE = '__create__'

function AgentAdmin({ onBack, embedded = false }) {
  const [agents, setAgents] = useState([])
  const [agentTemplates, setAgentTemplates] = useState([])
  const [modelProfiles, setModelProfiles] = useState([])
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [selectedAgentId, setSelectedAgentId] = useState('')
  const [form, setForm] = useState(EMPTY_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [notice, setNotice] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState(null)

  const isCreating = selectedAgentId === CREATE_MODE

  const selectedAgent = useMemo(
    () => (isCreating ? null : agents.find((agent) => agent.id === selectedAgentId) ?? null),
    [agents, selectedAgentId, isCreating],
  )

  const templateOptions = useMemo(
    () => agentTemplates.map((template) => ({
      value: template.id,
      label: `${template.name} / ${template.role}`,
    })),
    [agentTemplates],
  )

  const modelOptions = useMemo(() => [
    { value: '', label: runtimeDefaultModelLabel(form.runtime) },
    ...profileOptionsForRuntime(modelProfiles, form.runtime),
  ], [modelProfiles, form.runtime])
  const isModelBindingInvalid = Boolean(
    form.modelProfileID && !modelOptions.some((option) => option.value === form.modelProfileID),
  )

  useEffect(() => {
    let isCurrent = true

    const loadAgents = async () => {
      setIsLoading(true)
      setErrorMessage('')
      try {
        const [response, templateResponse, profileResponse] = await Promise.all([getAgents(), getAgentTemplates(), listModelProfiles()])
        if (!isCurrent) {
          return
        }
        const nextAgents = response.agents ?? []
        const nextTemplates = templateResponse.templates ?? []
        setAgents(nextAgents)
        setAgentTemplates(nextTemplates)
        setModelProfiles(profileResponse.profiles ?? [])
        setSelectedTemplateId(nextTemplates[0]?.id || '')
        setSelectedAgentId((current) => current || nextAgents[0]?.id || '')
      } catch (error) {
        if (isCurrent) {
          setErrorMessage(error.message || '加载 Agent 列表失败。')
        }
      } finally {
        if (isCurrent) {
          setIsLoading(false)
        }
      }
    }

    void loadAgents()

    return () => {
      isCurrent = false
    }
  }, [])

  useEffect(() => {
    if (isCreating) {
      setForm(EMPTY_FORM)
      setNotice('')
      setShowSystemPrompt(false)
      return
    }

    if (!selectedAgent) {
      setForm(EMPTY_FORM)
      return
    }

    setForm({
      name: selectedAgent.name ?? '',
      role: selectedAgent.role ?? '',
      description: selectedAgent.description ?? '',
      systemPrompt: selectedAgent.systemPrompt ?? '',
      enabled: selectedAgent.enabled !== false,
      runtime: selectedAgent.runtime || 'llm',
      modelProfileID: selectedAgent.modelProfileID || '',
    })
    setNotice('')
    setShowSystemPrompt(false)
  }, [selectedAgent, isCreating])

  const handleFieldChange = useCallback((field, value) => {
    setForm((current) => ({ ...current, [field]: value }))
  }, [])

  const handleStartCreate = () => {
    setSelectedAgentId(CREATE_MODE)
  }

  const handleCancelCreate = () => {
    setSelectedAgentId(agents[0]?.id || '')
  }

  const handleApplyTemplate = () => {
    const template = agentTemplates.find((candidate) => candidate.id === selectedTemplateId)
    if (!template) {
      return
    }
    setSelectedAgentId(CREATE_MODE)
    setForm({
      name: template.name ?? '',
      role: template.role ?? '',
      description: template.description ?? '',
      systemPrompt: template.systemPrompt ?? '',
      enabled: true,
      runtime: 'llm',
      modelProfileID: '',
    })
    setShowSystemPrompt(false)
    setNotice('已套用角色模板，可在保存前继续编辑。')
  }

  const handleSave = async (event) => {
    event.preventDefault()
    if (!form.name.trim() || isModelBindingInvalid) {
      return
    }

    setIsSaving(true)
    setErrorMessage('')
    setNotice('')

    try {
      if (isCreating) {
        const created = await createAgent(buildAgentSavePayload(form))
        const nextAgents = [...agents, created]
        setAgents(nextAgents)
        setSelectedAgentId(created.id)
        setNotice('Agent 已创建。它可以被加入新的会议室。')
      } else if (selectedAgent) {
        const updatedAgent = await updateAgent(selectedAgent.id, buildAgentSavePayload(form))
        setAgents((current) => current.map((agent) => (agent.id === updatedAgent.id ? updatedAgent : agent)))
        setNotice('Agent 配置已保存。新建房间会使用最新配置，已存在房间中的 Agent 快照不会自动改变。')
      }
    } catch (error) {
      setErrorMessage(error.message || (isCreating ? '创建 Agent 失败。' : '保存 Agent 配置失败。'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleDeleteRequest = (agent) => {
    setDeleteTarget(agent)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) {
      return
    }

    setIsSaving(true)
    setErrorMessage('')
    setNotice('')

    try {
      await deleteAgent(deleteTarget.id)
      const nextAgents = agents.filter((agent) => agent.id !== deleteTarget.id)
      setAgents(nextAgents)
      if (selectedAgentId === deleteTarget.id) {
        setSelectedAgentId(nextAgents[0]?.id || '')
      }
      setNotice(`Agent「${deleteTarget.name}」已删除。已存在房间中的快照不受影响。`)
    } catch (error) {
      setErrorMessage(error.message || '删除 Agent 失败。')
    } finally {
      setIsSaving(false)
      setDeleteTarget(null)
    }
  }

  const handleDeleteCancel = () => {
    setDeleteTarget(null)
  }

  const containerClass = embedded ? 'admin-section' : 'workbench workbench--admin'
  const isEditorDisabled = (!selectedAgent && !isCreating) || isSaving

  return (
    <main className={containerClass}>
      {embedded ? null : (
        <header className="entry-dashboard-header admin-dashboard-header">
          <div className="entry-dashboard-brand admin-dashboard-brand">
            <Avatar className="entry-brand-symbol" radius="sm" color="teal" aria-hidden="true">
              <Sparkles size={24} />
            </Avatar>
            <strong>AgentRoom</strong>
          </div>
          <Group component="nav" className="entry-dashboard-nav admin-dashboard-nav" gap="xs" aria-label="管理导航">
            <Button className="entry-dashboard-nav-item entry-dashboard-nav-item--active" variant="light" color="teal" leftSection={<Settings size={17} />}>
              Agent 配置
            </Button>
            <Button className="entry-dashboard-nav-item" type="button" variant="subtle" color="gray" leftSection={<LogIn size={17} />} onClick={onBack}>
              会议入口
            </Button>
          </Group>
        </header>
      )}

      <section className="admin-hero">
        <div>
          <Text className="eyebrow">管理控制台</Text>
          <Title order={1}>管理 Agent、角色模板与专属知识库</Title>
          <Text className="section-copy">
            这里维护的是可被新会议选择的角色模板。停用后，该 Agent 不会出现在新房间的可选列表，也不会响应新的 @ 提及。
          </Text>
        </div>
        <Button color="teal" type="button" leftSection={<Plus size={17} />} onClick={handleStartCreate} disabled={isSaving}>
          新增 Agent
        </Button>
      </section>

      <Stack gap="sm">
        {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}
        {notice ? <Alert color="teal" variant="light">{notice}</Alert> : null}
      </Stack>

      <Modal opened={Boolean(deleteTarget)} onClose={handleDeleteCancel} title="确认删除 Agent" centered>
        <Stack gap="md">
          <Text>
            确定要删除 Agent <strong>{deleteTarget?.name}</strong> 吗？此操作不可撤销。
          </Text>
          <Text className="helper-text">
            已有房间中该 Agent 的快照不会被删除，历史消息也不受影响，但新建房间将不再包含这个 Agent。
          </Text>
          <Group justify="flex-end">
            <Button variant="default" type="button" onClick={handleDeleteCancel} disabled={isSaving}>
              取消
            </Button>
            <Button color="red" type="button" onClick={handleDeleteConfirm} disabled={isSaving}>
              {isSaving ? '删除中...' : '确认删除'}
            </Button>
          </Group>
        </Stack>
      </Modal>

      <div className="admin-layout">
        <Paper component="aside" className="panel agent-admin-list" withBorder radius="md" shadow="xs">
          <Stack gap="md">
            <div className="panel-header">
              <div className="panel-title-row">
                <Title order={2}>Agent 列表</Title>
                <Badge color="gray" variant="light">{agents.length}</Badge>
              </div>
              <Text className="panel-copy">选择一个 Agent 后，在右侧编辑它的职责、角色模板和知识库。</Text>
            </div>

            {isLoading ? (
              <Text className="sidebar-empty">正在加载...</Text>
            ) : agents.length === 0 ? (
              <Text className="sidebar-empty">暂无 Agent。点击右上角新增第一个角色。</Text>
            ) : (
              <Stack component="ul" className="admin-agent-list" gap="xs">
                {agents.map((agent) => (
                  <Paper component="li" className="admin-agent-list-item" key={agent.id} withBorder radius="md" shadow="none">
                    <Button
                      className="admin-agent-button"
                      type="button"
                      variant={agent.id === selectedAgentId ? 'light' : 'subtle'}
                      color={agent.id === selectedAgentId ? 'teal' : 'gray'}
                      fullWidth
                      onClick={() => setSelectedAgentId(agent.id)}
                    >
                      <span className="admin-agent-button-content">
                        <span className="admin-agent-name">{agent.name}</span>
                        <Badge className={`agent-runtime-badge agent-runtime-badge--${agent.runtime || 'llm'}`} color="gray" variant="light">
                          {agent.source ? `${agent.source} / ` : ''}
                          {agent.runtime || 'llm'}
                        </Badge>
                        <Badge className={`agent-state ${agent.enabled === false ? 'agent-state--off' : ''}`} color={agent.enabled === false ? 'gray' : 'teal'} variant="light">
                          {agent.enabled === false ? '停用' : '启用'}
                        </Badge>
                        <Badge color="indigo" variant="light">
                          {describeAgentModelBinding(agent, modelProfiles)}
                        </Badge>
                        <span className="admin-agent-role">{agent.role || '未设置角色标签'}</span>
                      </span>
                    </Button>
                    <Button
                      className="agent-delete-button"
                      type="button"
                      title="删除这个 Agent"
                      variant="subtle"
                      color="red"
                      size="xs"
                      onClick={() => handleDeleteRequest(agent)}
                      disabled={isSaving}
                    >
                      删除
                    </Button>
                  </Paper>
                ))}
              </Stack>
            )}
          </Stack>
        </Paper>

        <Paper component="form" className="panel admin-editor" withBorder radius="md" shadow="xs" onSubmit={handleSave}>
          <Stack gap="md">
            <div className="panel-header panel-header--horizontal">
              <div>
                <Title order={2}>{isCreating ? '新增 Agent' : selectedAgent ? selectedAgent.name : '选择 Agent'}</Title>
                <Text className="panel-copy">名称会生成 @ 提及词；角色模板留空时，系统只使用代码内置的会议规则。</Text>
              </div>
              <Group className="panel-badge-group" gap="xs">
                {selectedAgent ? <Badge>{selectedAgent.mention}</Badge> : null}
                {selectedAgent ? <Badge color="gray" variant="light">{selectedAgent.runtime || 'llm'}</Badge> : null}
                {selectedAgent?.source ? <Badge color="gray" variant="light">{selectedAgent.source}</Badge> : null}
                {isCreating ? <Badge color="teal" variant="light">新建</Badge> : null}
              </Group>
            </div>

            <Paper className="agent-template-picker" withBorder radius="md" shadow="none">
              <div>
                <Text component="label" htmlFor="agent-template" fw={600}>从角色模板开始</Text>
                <Text className="field-hint">模板只会预填表单，保存前仍可调整名称、职责和提示词。</Text>
              </div>
              <Group className="agent-template-controls" gap="xs">
                <Select
                  id="agent-template"
                  value={selectedTemplateId}
                  data={templateOptions}
                  onChange={(value) => setSelectedTemplateId(value || '')}
                  disabled={isSaving || agentTemplates.length === 0}
                />
                <Button variant="light" color="teal" type="button" onClick={handleApplyTemplate} disabled={isSaving || agentTemplates.length === 0}>
                  套用模板
                </Button>
              </Group>
            </Paper>

            <Paper className="toggle-row" withBorder radius="md" shadow="none">
              <div>
                <Text className="toggle-title">当前状态：{form.enabled ? '已启用' : '已停用'}</Text>
                <Text className="field-hint">关闭后，这个 Agent 不会出现在会议室侧栏，也不会被触发。</Text>
              </div>
              <Switch
                checked={form.enabled}
                disabled={isEditorDisabled}
                onChange={(event) => handleFieldChange('enabled', event.currentTarget.checked)}
              />
            </Paper>

            <div className="admin-form-grid">
              <Select
                label="Runtime"
                data={[{ value: 'llm', label: 'Go Agent' }, { value: 'deepagent', label: 'DeepAgent' }]}
                value={form.runtime}
                onChange={(value) => {
                  const runtime = value || 'llm'
                  setForm((current) => ({
                    ...current,
                    runtime,
                    modelProfileID: reconcileModelProfileBinding(modelProfiles, current.modelProfileID, runtime),
                  }))
                }}
                disabled={isEditorDisabled}
              />
              <Select
                label="模型 Profile"
                data={modelOptions}
                value={form.modelProfileID}
                onChange={(value) => handleFieldChange('modelProfileID', value || '')}
                disabled={isEditorDisabled}
                description="未选择时继承该 Runtime 的默认模型；建房时会快照为具体 Profile。切换 Runtime 会清除不兼容绑定。"
              />
              {isModelBindingInvalid ? (
                <Alert color="orange" variant="light">
                  当前绑定已停用、缺失或与 Runtime 不兼容。请选择一个可用 Profile，或改为使用 Runtime 默认模型。
                </Alert>
              ) : null}
              <TextInput
                id="agent-name"
                label="Agent 名称"
                value={form.name}
                onChange={(event) => handleFieldChange('name', event.target.value)}
                disabled={isEditorDisabled}
                maxLength={40}
                required
              />

              <TextInput
                id="agent-role"
                label="角色英文名 / 职位标签"
                value={form.role}
                onChange={(event) => handleFieldChange('role', event.target.value)}
                disabled={isEditorDisabled}
                maxLength={80}
              />
            </div>

            <Textarea
              id="agent-description"
              label="会议职责说明"
              value={form.description}
              onChange={(event) => handleFieldChange('description', event.target.value)}
              disabled={isEditorDisabled}
              rows={3}
              maxLength={240}
            />

            <div className="field-group">
              <Button
                className="collapse-toggle"
                type="button"
                variant="subtle"
                color="gray"
                aria-expanded={showSystemPrompt}
                rightSection={<ChevronDown className={showSystemPrompt ? 'collapse-chevron--open' : ''} size={16} />}
                onClick={() => setShowSystemPrompt((current) => !current)}
                disabled={!selectedAgent && !isCreating}
              >
                <span className="collapse-toggle-label">角色模板</span>
                <span className="collapse-toggle-hint">
                  {form.systemPrompt ? '已自定义角色模板' : '使用默认角色模板'}
                </span>
              </Button>
              <Collapse in={showSystemPrompt}>
                <Textarea
                  id="agent-system-prompt"
                  value={form.systemPrompt}
                  onChange={(event) => handleFieldChange('systemPrompt', event.target.value)}
                  disabled={isEditorDisabled}
                  rows={8}
                  placeholder="创建时留空表示不追加额外角色模板；更新时留空会保留当前模板。"
                />
              </Collapse>
            </div>

            <div className="agent-knowledge-section">
              <KnowledgePanel
                key={selectedAgent?.id || CREATE_MODE}
                title="Agent 知识库"
                description="上传 Markdown 文档，只有当前 Agent 在会议中发言时会参考这些知识。"
                disabled={isCreating || !selectedAgent}
                emptyText={
                  isCreating
                    ? '创建 Agent 后即可上传知识文档。'
                    : '暂无知识文档。上传 .md 后，这个 Agent 会在回答时参考它们。'
                }
                listDocuments={selectedAgent ? () => getAgentKnowledge(selectedAgent.id) : null}
                onUploadDocument={selectedAgent ? (file) => uploadAgentKnowledge(selectedAgent.id, file) : null}
                onDeleteDocument={deleteKnowledgeDocument}
              />
            </div>

            <Group justify="flex-end" className="button-row">
              {isCreating ? (
                <>
                  <Button variant="default" type="button" onClick={handleCancelCreate} disabled={isSaving}>
                    取消
                  </Button>
                  <Button color="teal" type="submit" disabled={isSaving || !form.name.trim() || isModelBindingInvalid}>
                    {isSaving ? '创建中...' : '创建 Agent'}
                  </Button>
                </>
              ) : (
                <>
                  <Text className="helper-text">保存后，名称对应的 @ 提及词会同步更新。</Text>
                  <Button color="teal" type="submit" disabled={!selectedAgent || isSaving || !form.name.trim() || isModelBindingInvalid}>
                    {isSaving ? '保存中...' : '保存配置'}
                  </Button>
                </>
              )}
            </Group>
          </Stack>
        </Paper>
      </div>
    </main>
  )
}

export default AgentAdmin
