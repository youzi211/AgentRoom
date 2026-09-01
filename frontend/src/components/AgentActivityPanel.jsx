import { useState } from 'react'
import { Alert, Badge, Button, Group, Modal, Paper, Stack, Text, Title } from '@mantine/core'
import { descriptionForActivity, labelForActivityStatus } from './agentActivity'

function AgentActivityPanel({ activities = [], errorMessage = '', isLoading = false }) {
  const [isHistoryOpen, setIsHistoryOpen] = useState(false)
  const currentActivities = activities.filter((activity) => activity.status === 'running')
  const visibleActivities = currentActivities.slice(0, 4)

  return (
    <>
      <section className="sidebar-section agent-activity-panel">
        <div className="sidebar-header">
          <div>
            <Title order={3}>协作活动</Title>
            <Text className="sidebar-note">当前运行、发言与交接</Text>
          </div>
          <Badge className="sidebar-count" color="teal" variant="light">{currentActivities.length}</Badge>
        </div>
        <Group className="agent-activity-toolbar" gap="xs">
          <Button variant="light" color="teal" size="xs" type="button" onClick={() => setIsHistoryOpen(true)}>
            查看历史
          </Button>
          <Text component="span">{activities.length} 条记录</Text>
        </Group>
        {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}
        {isLoading ? (
          <div className="sidebar-loading">
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--medium" />
            <div className="sidebar-loading-skeleton sidebar-loading-skeleton--short" />
            <div className="sidebar-loading-skeleton" />
          </div>
        ) : null}
        {!isLoading && visibleActivities.length === 0 ? <Text className="agent-activity-notice">暂无正在进行的协作</Text> : null}
        {visibleActivities.length > 0 ? <ActivityList activities={visibleActivities} /> : null}
      </section>

      <Modal
        opened={isHistoryOpen}
        onClose={() => setIsHistoryOpen(false)}
        title="协作活动历史"
        size="lg"
        centered
      >
        {activities.length === 0 ? (
          <Text className="agent-activity-notice">暂无协作活动历史。</Text>
        ) : (
          <ActivityList activities={activities} history />
        )}
      </Modal>
    </>
  )
}

function ActivityList({ activities, history = false }) {
  return (
    <Stack className={`agent-activity-list${history ? ' agent-activity-list--history' : ''}`} gap="xs">
      {activities.map((activity) => (
        <Paper
          component="article"
          className={`agent-activity-item${activity.status === 'running' ? ' agent-activity-item--running' : ''}`}
          key={`${activity.kind}:${activity.id}`}
          withBorder
          radius="md"
          shadow="none"
        >
          <div className="agent-activity-main">
            <strong>{titleForActivity(activity)}</strong>
            <Text component="span">{descriptionForActivity(activity)}</Text>
          </div>
          <Badge className="agent-activity-status" color={activity.status === 'running' ? 'teal' : 'gray'} variant="light">
            {labelForActivityStatus(activity.status)}
          </Badge>
        </Paper>
      ))}
    </Stack>
  )
}

function titleForActivity(activity) {
  if (activity.kind === 'collaboration_run') {
    return '协作运行'
  }
  if (activity.kind === 'dialogue_run') {
    return '对话链路'
  }
  return activity.agentName || activity.agentID || 'Agent'
}

export default AgentActivityPanel
