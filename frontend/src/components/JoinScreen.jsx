import { useEffect, useState } from 'react'
import {
  Alert,
  Badge,
  Box,
  Button,
  Checkbox,
  Group,
  Paper,
  PasswordInput,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Title,
  UnstyledButton,
} from '@mantine/core'
import {
  ArrowRight,
  BarChart3,
  BookOpen,
  CalendarDays,
  Check,
  Copy,
  Database,
  Eye,
  FileText,
  Hash,
  Lock,
  LogIn,
  MessageSquare,
  Plus,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Users,
} from 'lucide-react'
import { getAgentRoleSets, getAgents, listRecentRooms } from '../api/roomClient'

const TEMPLATE_AGENT_MATCHERS = {
  product_manager: ['pm', '产品经理', 'Product Manager'],
  architect: ['architect', '架构师', 'Architect'],
  qa_reviewer: ['qa', '质量评审', 'QA Reviewer'],
  risk_reviewer: ['risk', '风险评审', 'Risk Reviewer'],
  meeting_scribe: ['secretary', '会议纪要', 'Meeting Scribe'],
}

const DIALOGUE_MODE_OPTIONS = [
  {
    label: (
      <span className="entry-segment-label">
        <Users size={16} />
        <span>
          <strong>多 Agent 协作</strong>
          <small>多角色协同讨论与产出</small>
        </span>
      </span>
    ),
    value: 'mention_fanout',
  },
  {
    label: (
      <span className="entry-segment-label">
        <MessageSquare size={16} />
        <span>
          <strong>单 Agent 对话</strong>
          <small>与单一 Agent 深度对话</small>
        </span>
      </span>
    ),
    value: 'guided_dialogue',
  },
]

const CAPABILITY_ITEMS = [
  {
    icon: Users,
    title: '角色化 Agent 协作',
    copy: '不同角色分工协作，专业高效',
    tone: 'teal',
  },
  {
    icon: BookOpen,
    title: '知识与工具驱动',
    copy: '连接知识来源，沉淀会议成果',
    tone: 'blue',
  },
  {
    icon: MessageSquare,
    title: '实时对话与决策',
    copy: '多方对话，快速达成共识',
    tone: 'amber',
  },
  {
    icon: ShieldCheck,
    title: '安全可控',
    copy: '企业级权限与数据保护',
    tone: 'teal',
  },
]

const ENTRY_STATS = [
  { icon: Users, label: '进行中', value: '6', tone: 'teal' },
  { icon: CalendarDays, label: '今日会议', value: '12', tone: 'blue' },
  { icon: FileText, label: '知识来源', value: '24', tone: 'amber' },
  { icon: Database, label: 'Agent 角色', value: '18', tone: 'teal' },
]

function JoinScreen({ errorMessage, isSubmitting, onCreateRoom, onJoinRoom, onOpenAgentAdmin }) {
  const [createDisplayName, setCreateDisplayName] = useState('')
  const [joinDisplayName, setJoinDisplayName] = useState('')
  const [roomName, setRoomName] = useState('')
  const [roomId, setRoomId] = useState('')
  const [createPasscode, setCreatePasscode] = useState('')
  const [dialogueMode, setDialogueMode] = useState('mention_fanout')
  const [joinPasscode, setJoinPasscode] = useState('')
  const [showAdvancedCreate, setShowAdvancedCreate] = useState(false)
  const [availableAgents, setAvailableAgents] = useState([])
  const [roleSets, setRoleSets] = useState([])
  const [selectedAgentIds, setSelectedAgentIds] = useState(new Set())
  const [recentRooms, setRecentRooms] = useState([])
  const [recentRoomsState, setRecentRoomsState] = useState({
    isLoading: true,
    errorMessage: '',
  })

  const trimmedCreateDisplayName = createDisplayName.trim()
  const trimmedJoinDisplayName = joinDisplayName.trim()
  const trimmedRoomName = roomName.trim()
  const trimmedRoomId = roomId.trim()
  const trimmedCreatePasscode = createPasscode.trim()
  const trimmedJoinPasscode = joinPasscode.trim()
  useEffect(() => {
    let isCurrent = true
    const loadAgents = async () => {
      try {
        const [response, roleSetResponse] = await Promise.all([getAgents(), getAgentRoleSets()])
        if (!isCurrent) {
          return
        }
        const enabledAgents = (response.agents ?? []).filter((agent) => agent.enabled !== false)
        setAvailableAgents(enabledAgents)
        setRoleSets(roleSetResponse.roleSets ?? [])
        setSelectedAgentIds(new Set(enabledAgents.map((agent) => agent.id)))
      } catch {
        // Keep the room entry flow available even when the roster cannot be loaded.
      }
    }
    void loadAgents()
    return () => {
      isCurrent = false
    }
  }, [])

  useEffect(() => {
    let isCurrent = true
    const loadRecentRooms = async () => {
      setRecentRoomsState({ isLoading: true, errorMessage: '' })
      try {
        const response = await listRecentRooms({ limit: 3 })
        if (!isCurrent) {
          return
        }
        setRecentRooms(response?.rooms ?? [])
        setRecentRoomsState({ isLoading: false, errorMessage: '' })
      } catch (error) {
        if (!isCurrent) {
          return
        }
        setRecentRooms([])
        setRecentRoomsState({
          isLoading: false,
          errorMessage: error.message || '会议列表暂时不可用。',
        })
      }
    }
    void loadRecentRooms()
    return () => {
      isCurrent = false
    }
  }, [])

  const handleAgentToggle = (agentId) => {
    setSelectedAgentIds((current) => {
      const next = new Set(current)
      if (next.has(agentId)) {
        next.delete(agentId)
      } else {
        next.add(agentId)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    setSelectedAgentIds(new Set(availableAgents.map((agent) => agent.id)))
  }

  const handleDeselectAll = () => {
    setSelectedAgentIds(new Set())
  }

  const handleApplyRoleSet = (roleSet) => {
    const templateIDs = roleSet.templateIDs ?? []
    const next = new Set()
    for (const agent of availableAgents) {
      if (templateIDs.some((templateID) => matchesTemplate(agent, templateID))) {
        next.add(agent.id)
      }
    }
    setSelectedAgentIds(next)
  }

  const handleCreateRoom = async (event) => {
    event.preventDefault()
    if (!trimmedCreateDisplayName) {
      return
    }

    await onCreateRoom({
      displayName: trimmedCreateDisplayName,
      roomName: trimmedRoomName,
      agentIds: Array.from(selectedAgentIds),
      passcode: trimmedCreatePasscode,
      dialogueMode,
    })
  }

  const handleJoinRoom = async (event) => {
    event.preventDefault()
    if (!trimmedJoinDisplayName || !trimmedRoomId) {
      return
    }

    await onJoinRoom({
      displayName: trimmedJoinDisplayName,
      roomId: trimmedRoomId,
      passcode: trimmedJoinPasscode,
    })
  }

  const handleJoinExistingRoom = async (roomItem) => {
    if (!trimmedJoinDisplayName || !roomItem?.id) {
      setRoomId(roomItem?.id || '')
      return
    }
    await onJoinRoom({
      displayName: trimmedJoinDisplayName,
      roomId: roomItem.id,
      passcode: trimmedJoinPasscode,
    })
  }

  const handleCopyRoomId = (roomItem) => {
    setRoomId(roomItem.id)
    try {
      void navigator.clipboard?.writeText(roomItem.id)
    } catch {
      // Clipboard can be unavailable in some browser contexts.
    }
  }

  return (
    <main className="entry-dashboard">
      <header className="entry-dashboard-header">
        <Group className="entry-dashboard-brand" gap="sm" wrap="nowrap">
          <span className="entry-brand-symbol" aria-hidden="true">
            <Sparkles size={24} />
          </span>
          <strong>AgentRoom</strong>
        </Group>
        <nav className="entry-dashboard-nav" aria-label="主导航">
          <span className="entry-dashboard-nav-item entry-dashboard-nav-item--active">
            <BarChart3 size={17} />
            会议入口
          </span>
          <button className="entry-dashboard-nav-item" type="button" onClick={onOpenAgentAdmin}>
            <Settings size={17} />
            管理后台
          </button>
        </nav>
      </header>

      {errorMessage ? (
        <Alert className="entry-dashboard-alert" color="red" variant="light" radius="md">
          {errorMessage}
        </Alert>
      ) : null}

      <div className="entry-dashboard-layout">
        <aside className="entry-dashboard-aside">
          <section className="entry-dashboard-hero">
            <Title order={1}>AgentRoom</Title>
            <Text className="entry-dashboard-subtitle">AI 会议工作台</Text>
            <Text className="entry-dashboard-copy">
              创建或加入会议室，与角色化 Agent 协同讨论，结合知识库与工具，高效完成会议目标。
            </Text>
          </section>

          <Stack className="entry-capability-list" gap={24}>
            {CAPABILITY_ITEMS.map((item) => {
              const Icon = item.icon
              return (
                <div className="entry-capability-item" key={item.title}>
                  <span className={`entry-icon-bubble entry-icon-bubble--${item.tone}`}>
                    <Icon size={22} />
                  </span>
                  <div>
                    <strong>{item.title}</strong>
                    <span>{item.copy}</span>
                  </div>
                </div>
              )
            })}
          </Stack>

          <Paper className="entry-stats-card" withBorder radius="md" shadow="xs">
            {ENTRY_STATS.map((item) => {
              const Icon = item.icon
              return (
                <div className="entry-stat-item" key={item.label}>
                  <Icon className={`entry-stat-icon entry-stat-icon--${item.tone}`} size={20} />
                  <span>{item.label}</span>
                  <strong>{item.value}</strong>
                </div>
              )
            })}
          </Paper>
        </aside>

        <section className="entry-dashboard-main">
          <div className="entry-dashboard-forms">
            <Paper
              component="form"
              className="entry-panel entry-create-panel"
              onSubmit={handleCreateRoom}
              withBorder
              radius="md"
              shadow="xs"
            >
              <Stack gap="md" h="100%">
                <Group align="center" gap="sm" mb={2}>
                  <span className="entry-card-icon entry-card-icon--teal">
                    <Plus size={20} />
                  </span>
                  <Title order={2}>创建会议室</Title>
                </Group>

                <TextInput
                  id="create-display-name"
                  autoFocus
                  label="你的显示名称"
                  leftSection={<Users size={17} />}
                  value={createDisplayName}
                  onChange={(event) => setCreateDisplayName(event.currentTarget.value)}
                  placeholder="请输入你的显示名称"
                  disabled={isSubmitting}
                  maxLength={40}
                />

                <TextInput
                  id="room-name"
                  label="会议名称"
                  leftSection={<CalendarDays size={17} />}
                  value={roomName}
                  onChange={(event) => setRoomName(event.currentTarget.value)}
                  placeholder="请输入会议名称"
                  disabled={isSubmitting}
                  maxLength={60}
                />

                <Text className="entry-field-label">Agent 对话模式</Text>
                <SegmentedControl
                  id="dialogue-mode"
                  className="entry-mode-control"
                  value={dialogueMode}
                  onChange={setDialogueMode}
                  data={DIALOGUE_MODE_OPTIONS}
                  disabled={isSubmitting}
                  fullWidth
                />

                {showAdvancedCreate ? (
                  <PasswordInput
                    id="create-passcode"
                    label="房间口令"
                    leftSection={<Lock size={17} />}
                    value={createPasscode}
                    onChange={(event) => setCreatePasscode(event.currentTarget.value)}
                    placeholder="可选，用于限制加入房间"
                    disabled={isSubmitting}
                    maxLength={80}
                  />
                ) : null}

                <Box className="entry-agent-section">
                  <Group align="center" justify="space-between" mb="xs">
                    <Box>
                      <Text component="label" className="entry-field-label">选择本次 Agent</Text>
                      <Text size="sm" c="dimmed">
                        {selectedAgentIds.size === 0
                          ? '本次会议不邀请 Agent'
                          : `已选择 ${selectedAgentIds.size}/${availableAgents.length} 个 Agent`}
                      </Text>
                    </Box>
                    <Group className="agent-select-actions" gap={4}>
                      <Button type="button" size="xs" variant="subtle" color="teal" onClick={handleSelectAll} disabled={isSubmitting}>
                        全选
                      </Button>
                      <Button type="button" size="xs" variant="subtle" color="gray" onClick={handleDeselectAll} disabled={isSubmitting}>
                        清空
                      </Button>
                    </Group>
                  </Group>

                  {roleSets.length > 0 ? (
                    <div className="role-set-shortcuts entry-role-tabs" aria-label="推荐角色组">
                      {roleSets.map((roleSet) => (
                        <UnstyledButton
                          className="role-set-button"
                          type="button"
                          key={roleSet.id}
                          onClick={() => handleApplyRoleSet(roleSet)}
                          disabled={isSubmitting}
                        >
                          <strong>{roleSet.name}</strong>
                        </UnstyledButton>
                      ))}
                    </div>
                  ) : null}

                  {availableAgents.length > 0 ? (
                    <SimpleGrid className="agent-chip-grid entry-agent-grid" cols={{ base: 1, sm: 2, lg: 3 }} spacing="xs">
                      {availableAgents.map((agent) => {
                        const checked = selectedAgentIds.has(agent.id)
                        return (
                          <Paper
                            key={agent.id}
                            component="label"
                            className={`agent-chip entry-agent-chip${checked ? ' agent-chip--selected' : ''}`}
                            withBorder
                            radius="sm"
                          >
                            <Checkbox
                              checked={checked}
                              onChange={() => handleAgentToggle(agent.id)}
                              disabled={isSubmitting}
                              aria-label={`${agent.name} ${agent.role}`}
                            />
                            <span className="agent-chip-avatar">{agent.name.charAt(0).toUpperCase()}</span>
                            <Box className="entry-agent-text">
                              <Text component="strong">{agent.name}</Text>
                              <Text component="small">{agent.role}</Text>
                              <Badge className="agent-runtime-inline" size="xs" variant="light" color={agent.runtime === 'deepagent' ? 'blue' : 'gray'}>
                                {agent.runtime || 'llm'}
                              </Badge>
                            </Box>
                          </Paper>
                        )
                      })}
                    </SimpleGrid>
                  ) : (
                    <Text className="entry-muted-state">暂无可用 Agent，仍可创建普通会议室。</Text>
                  )}
                </Box>

                <Group className="entry-panel-footer" justify="space-between" align="center">
                  <Button
                    type="button"
                    variant="default"
                    leftSection={<SlidersHorizontal size={17} />}
                    aria-expanded={showAdvancedCreate}
                    onClick={() => setShowAdvancedCreate((current) => !current)}
                  >
                    高级设置
                  </Button>
                  <Button type="submit" color="teal" rightSection={<ArrowRight size={17} />} disabled={isSubmitting || !trimmedCreateDisplayName}>
                    {isSubmitting ? '正在创建...' : '创建会议室'}
                  </Button>
                </Group>
              </Stack>
            </Paper>

            <Paper
              component="form"
              className="entry-panel entry-join-panel"
              onSubmit={handleJoinRoom}
              withBorder
              radius="md"
              shadow="xs"
            >
              <Stack gap="lg" h="100%">
                <Group align="center" gap="sm">
                  <span className="entry-card-icon entry-card-icon--blue">
                    <LogIn size={20} />
                  </span>
                  <Title order={2}>加入会议室</Title>
                </Group>

                <TextInput
                  id="room-id"
                  label="房间 ID"
                  leftSection={<Hash size={17} />}
                  value={roomId}
                  onChange={(event) => setRoomId(event.currentTarget.value)}
                  placeholder="请输入房间 ID"
                  disabled={isSubmitting}
                  maxLength={80}
                />

                <PasswordInput
                  id="join-passcode"
                  label="房间口令"
                  leftSection={<Lock size={17} />}
                  value={joinPasscode}
                  onChange={(event) => setJoinPasscode(event.currentTarget.value)}
                  placeholder="请输入房间口令"
                  disabled={isSubmitting}
                  maxLength={80}
                />

                <TextInput
                  id="join-display-name"
                  label="你的显示名称"
                  leftSection={<Users size={17} />}
                  value={joinDisplayName}
                  onChange={(event) => setJoinDisplayName(event.currentTarget.value)}
                  placeholder="请输入你的显示名称"
                  disabled={isSubmitting}
                  maxLength={40}
                />

                <Button
                  type="submit"
                  color="blue"
                  size="md"
                  leftSection={<LogIn size={17} />}
                  disabled={isSubmitting || !trimmedJoinDisplayName || !trimmedRoomId}
                  fullWidth
                >
                  {isSubmitting ? '正在加入...' : '加入会议室'}
                </Button>

                <Alert className="entry-join-help" color="blue" variant="light" radius="sm" icon={<Eye size={18} />}>
                  如需向会议室发起申请或获取口令，请联系会议创建者或管理员。
                </Alert>
              </Stack>
            </Paper>
          </div>

          <Paper className="entry-existing-panel" withBorder radius="md" shadow="xs">
            <Group className="entry-existing-title" justify="space-between" align="center">
              <Group gap="xs">
                <Database size={18} />
                <Title order={2}>已有房间</Title>
              </Group>
              <Badge variant="light" color="teal">{recentRooms.length}</Badge>
            </Group>

            <ExistingRoomsTable
              isLoading={recentRoomsState.isLoading}
              errorMessage={recentRoomsState.errorMessage}
              onCopyRoomId={handleCopyRoomId}
              onJoinRoom={handleJoinExistingRoom}
              rooms={recentRooms}
              canJoin={Boolean(trimmedJoinDisplayName)}
            />
          </Paper>
        </section>
      </div>
    </main>
  )
}

function ExistingRoomsTable({ canJoin, errorMessage, isLoading, onCopyRoomId, onJoinRoom, rooms }) {
  if (isLoading) {
    return <Text className="entry-muted-state">正在加载已有房间...</Text>
  }

  if (errorMessage) {
    return <Text className="entry-muted-state">{errorMessage}</Text>
  }

  if (rooms.length === 0) {
    return <Text className="entry-muted-state">暂无正在进行的会议室。</Text>
  }

  return (
    <div className="entry-room-table" role="table">
      <div className="entry-room-table-head" role="row">
        <span role="columnheader">会议名称</span>
        <span role="columnheader">房间 ID</span>
        <span role="columnheader">创建者</span>
        <span role="columnheader">模式</span>
        <span role="columnheader">Agent 数</span>
        <span role="columnheader">状态</span>
        <span role="columnheader">操作</span>
      </div>
      {rooms.slice(0, 3).map((roomItem) => (
        <div className="entry-room-table-row" role="row" key={roomItem.id}>
          <span role="cell">{roomItem.name || '未命名会议'}</span>
          <button type="button" className="entry-room-id-button" onClick={() => onCopyRoomId(roomItem)}>
            {roomItem.id}
            <Copy size={13} />
          </button>
          <span role="cell">{roomItem.hasPasscode ? '需要口令' : '公开加入'}</span>
          <span role="cell">
            <Badge size="sm" variant="light" color={roomItem.dialoguePolicy?.mode === 'guided_dialogue' ? 'blue' : 'teal'}>
              {roomItem.dialoguePolicy?.mode === 'guided_dialogue' ? '单 Agent 对话' : '多 Agent 协作'}
            </Badge>
          </span>
          <span role="cell">{roomItem.agentCount ?? roomItem.agents?.length ?? 0}</span>
          <span role="cell">
            <span className="entry-room-status">
              <Check size={12} />
              进行中
            </span>
          </span>
          <span role="cell">
            <Button type="button" size="xs" variant="outline" color="teal" onClick={() => onJoinRoom(roomItem)} disabled={!canJoin}>
              加入
            </Button>
          </span>
        </div>
      ))}
    </div>
  )
}

function matchesTemplate(agent, templateID) {
  const matchers = TEMPLATE_AGENT_MATCHERS[templateID] ?? [templateID]
  return matchers.some((matcher) => agent.id === matcher || agent.name === matcher || agent.role === matcher)
}

export default JoinScreen
