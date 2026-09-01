import { useCallback, useEffect, useState } from 'react'
import { Alert, Badge, Button, Group, Paper, ScrollArea, Table, Text, Title } from '@mantine/core'
import { RefreshCw } from 'lucide-react'
import {
  archiveRoom,
  getCollaborationRuntimeCapabilities,
  listRooms,
  reopenRoom,
  restoreRoom,
} from '../api/roomClient'
import {
  labelForCollaborationEngine,
  labelForTriggerMode,
  normalizeCollaborationCapabilities,
} from '../collaborationRuntime'
import MeetingRoomDetail from './MeetingRoomDetail'
import {
  actionsForRoomStatus,
  labelForRoomAction,
  labelForRoomStatus,
  STATUS_FILTERS,
  toneForRoomStatus,
} from './meetingAdminModel'

function formatDateTime(value) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
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

function MeetingAdmin() {
  const [rooms, setRooms] = useState([])
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [busyRoomId, setBusyRoomId] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [selectedRoom, setSelectedRoom] = useState(null)
  const [runtimeState, setRuntimeState] = useState({
    capabilities: null,
    isLoading: true,
    errorMessage: '',
  })

  const loadRooms = useCallback(async (status) => {
    setIsLoading(true)
    setErrorMessage('')
    try {
      const response = await listRooms({ status })
      setRooms(response.rooms ?? [])
    } catch (error) {
      setErrorMessage(error.message || '加载会议列表失败，请稍后重试。')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadRooms(statusFilter)
  }, [loadRooms, statusFilter])

  const loadRuntimeCapabilities = useCallback(async () => {
    setRuntimeState((current) => ({ ...current, isLoading: true, errorMessage: '' }))
    try {
      const response = await getCollaborationRuntimeCapabilities()
      setRuntimeState({
        capabilities: normalizeCollaborationCapabilities(response),
        isLoading: false,
        errorMessage: '',
      })
    } catch (error) {
      setRuntimeState({
        capabilities: null,
        isLoading: false,
        errorMessage: error.message || '加载 Collaboration Runtime 状态失败。',
      })
    }
  }, [])

  useEffect(() => {
    void loadRuntimeCapabilities()
  }, [loadRuntimeCapabilities])

  const handleRoomAction = async (roomItem, action) => {
    if (!roomItem?.id) {
      return
    }
    if (action === 'detail') {
      setSelectedRoom(roomItem)
      return
    }

    setBusyRoomId(roomItem.id)
    setErrorMessage('')
    try {
      let response = null

      if (action === 'archive') {
        response = await archiveRoom(roomItem.id)
      } else if (action === 'reopen') {
        response = await reopenRoom(roomItem.id)
      } else if (action === 'restore') {
        response = await restoreRoom(roomItem.id)
      }

      if (response?.room) {
        setSelectedRoom((current) => (current?.id === roomItem.id ? response.room : current))
      }

      await loadRooms(statusFilter)
    } catch (error) {
      setErrorMessage(error.message || '更新会议状态失败，请稍后重试。')
    } finally {
      setBusyRoomId('')
    }
  }

  const handleCopyId = (roomId) => {
    try {
      void navigator.clipboard?.writeText(roomId)
    } catch {
      // Clipboard may be unavailable; ignore.
    }
  }

  return (
    <section className="admin-section">
      <section className="admin-hero">
        <div>
          <p className="eyebrow">会议记录</p>
          <Title order={1}>管理所有会议与纪要</Title>
          <Text className="section-copy">
            浏览进行中、已关闭和已归档的会议，查看全部消息历史，并按新的生命周期语义执行归档、恢复会议或取消归档。
          </Text>
        </div>
      </section>

      {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}

      <Paper className="panel collaboration-runtime-panel" withBorder radius="md" shadow="xs">
        <div className="panel-header panel-header--horizontal">
          <div className="panel-title-row">
            <Title order={2}>Collaboration Runtime</Title>
            {runtimeState.capabilities ? (
              <Badge color={runtimeState.capabilities.ready ? 'teal' : 'yellow'} variant="light">
                {runtimeState.capabilities.ready ? '已就绪' : '未就绪'}
              </Badge>
            ) : null}
          </div>
          <Button
            type="button"
            size="xs"
            variant="subtle"
            color="gray"
            leftSection={<RefreshCw size={15} />}
            onClick={() => void loadRuntimeCapabilities()}
            loading={runtimeState.isLoading}
          >
            刷新
          </Button>
        </div>

        {runtimeState.errorMessage ? (
          <Alert color="red" variant="light">{runtimeState.errorMessage}</Alert>
        ) : runtimeState.isLoading ? (
          <div className="sidebar-loading">
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--medium" />
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--short" />
          </div>
        ) : runtimeState.capabilities ? (
          <div className="collaboration-runtime-summary">
            <div>
              <Text component="span" size="sm" c="dimmed">部署模式</Text>
              <Text component="strong">{runtimeState.capabilities.mode}</Text>
            </div>
            <div>
              <Text component="span" size="sm" c="dimmed">协议版本</Text>
              <Group gap={6}>
                {runtimeState.capabilities.supportedProtocolVersions.length > 0
                  ? runtimeState.capabilities.supportedProtocolVersions.map((version) => <Badge key={version} color="gray" variant="light">{version}</Badge>)
                  : <Text size="sm">—</Text>}
              </Group>
            </div>
            <div>
              <Text component="span" size="sm" c="dimmed">触发方式</Text>
              <Group gap={6}>
                {runtimeState.capabilities.supportedTriggerModes.length > 0
                  ? runtimeState.capabilities.supportedTriggerModes.map((mode) => <Badge key={mode} color="blue" variant="light">{labelForTriggerMode(mode)}</Badge>)
                  : <Text size="sm">—</Text>}
              </Group>
            </div>
            <div className="collaboration-runtime-engines">
              <Text component="span" size="sm" c="dimmed">引擎</Text>
              <Group gap={8}>
                {runtimeState.capabilities.engines.length > 0
                  ? runtimeState.capabilities.engines.map((engine) => (
                    <Group className="collaboration-runtime-engine" gap={5} key={engine.engine}>
                      <Text component="strong" size="sm">
                        {labelForCollaborationEngine(engine.engine)}{engine.version ? ` · ${engine.version}` : ''}
                      </Text>
                      <Badge color={engine.enabled ? 'teal' : 'gray'} variant="light">
                        {engine.enabled ? '已启用' : '未启用'}
                      </Badge>
                      <Badge color={engine.ready ? 'blue' : 'yellow'} variant="light">
                        {engine.ready ? '就绪' : '未就绪'}
                      </Badge>
                    </Group>
                  ))
                  : <Text size="sm">—</Text>}
              </Group>
            </div>
          </div>
        ) : null}
      </Paper>

      <Paper className="panel" withBorder radius="md" shadow="xs">
        <div className="panel-header panel-header--horizontal">
          <div className="panel-title-row">
            <Title order={2}>会议列表</Title>
            <Badge color="gray" variant="light">{rooms.length}</Badge>
          </div>
          <Group className="agent-select-actions" gap="xs" role="tablist" aria-label="会议状态筛选">
            {STATUS_FILTERS.map((filter) => (
              <Button
                key={filter.value || 'all'}
                type="button"
                size="xs"
                variant={statusFilter === filter.value ? 'light' : 'subtle'}
                color={statusFilter === filter.value ? 'teal' : 'gray'}
                onClick={() => setStatusFilter(filter.value)}
              >
                {filter.label}
              </Button>
            ))}
          </Group>
        </div>

        {isLoading ? (
          <div className="sidebar-loading">
            <div className="sidebar-loading-skeleton" />
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--medium" />
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--short" />
            <div className="sidebar-loading-skeleton" />
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--medium" />
          </div>
        ) : rooms.length === 0 ? (
          <Text className="sidebar-empty">暂无会议记录。</Text>
        ) : (
          <ScrollArea>
            <Table className="meeting-table" verticalSpacing="sm" horizontalSpacing="md">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>会议名称</Table.Th>
                  <Table.Th>状态</Table.Th>
                  <Table.Th>消息数</Table.Th>
                  <Table.Th>创建时间</Table.Th>
                  <Table.Th>最近消息</Table.Th>
                  <Table.Th>操作</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
            {rooms.map((roomItem) => (
              <Table.Tr key={roomItem.id}>
                <Table.Td className="meeting-cell-name">
                  <strong>{roomItem.name}</strong>
                  <Button type="button" className="meeting-room-id" title="复制房间 ID" variant="subtle" color="gray" size="xs" onClick={() => handleCopyId(roomItem.id)}>
                    {roomItem.id}
                  </Button>
                </Table.Td>
                <Table.Td>
                  <Badge className={`agent-state agent-state--${toneForRoomStatus(roomItem.status)}`} color="teal" variant="light">
                    {labelForRoomStatus(roomItem.status)}
                  </Badge>
                </Table.Td>
                <Table.Td>{roomItem.messageCount ?? 0}</Table.Td>
                <Table.Td>{formatDateTime(roomItem.createdAt)}</Table.Td>
                <Table.Td>{formatDateTime(roomItem.lastMessageAt)}</Table.Td>
                <Table.Td className="meeting-cell-actions">
                  {actionsForRoomStatus(roomItem.status).map((action) => (
                    <Button
                      key={`${roomItem.id}:${action}`}
                      size="xs"
                      variant={action === 'detail' ? 'subtle' : 'light'}
                      color={action === 'detail' ? 'gray' : 'teal'}
                      type="button"
                      onClick={() => handleRoomAction(roomItem, action)}
                      disabled={action !== 'detail' && busyRoomId === roomItem.id}
                    >
                      {labelForRoomAction(action)}
                    </Button>
                  ))}
                </Table.Td>
              </Table.Tr>
            ))}
              </Table.Tbody>
            </Table>
          </ScrollArea>
        )}
      </Paper>

      {selectedRoom ? (
        <MeetingRoomDetail
          busyRoomId={busyRoomId}
          onClose={() => setSelectedRoom(null)}
          onRoomAction={handleRoomAction}
          room={selectedRoom}
        />
      ) : null}
    </section>
  )
}

export default MeetingAdmin
