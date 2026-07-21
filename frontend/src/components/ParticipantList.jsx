import { Avatar, Badge, Paper, Stack, Text, Title } from '@mantine/core'

function ParticipantList({ participants }) {
  return (
    <Paper component="section" className="sidebar-section" withBorder radius="md" shadow="none">
      <div className="sidebar-header">
        <Title order={2}>在线成员</Title>
        <Badge className="sidebar-count" color="teal" variant="light">{participants.length}</Badge>
      </div>

      {participants.length === 0 ? (
        <Text className="sidebar-empty">暂无成员在线</Text>
      ) : (
        <Stack component="ul" className="sidebar-list" gap="xs">
          {participants.map((participant) => (
            <Paper component="li" className="sidebar-list-item" key={participant.id} withBorder radius="md" shadow="none">
              <div className="participant-identity">
                <Avatar className="sidebar-avatar participant-avatar" radius="sm" color="teal">{participant.name.charAt(0).toUpperCase()}</Avatar>
                <div className="sidebar-copy">
                  <Text className="sidebar-primary">{participant.name}</Text>
                  <Text className="sidebar-secondary">{formatJoinedAt(participant.joinedAt)} 加入</Text>
                </div>
              </div>
            </Paper>
          ))}
        </Stack>
      )}
    </Paper>
  )
}

function formatJoinedAt(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '刚刚'
  }

  return new Intl.DateTimeFormat('zh-CN', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export default ParticipantList
