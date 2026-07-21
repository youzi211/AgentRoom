import { useState } from 'react'
import { Alert, Button, Group, Paper, PasswordInput, Stack, Text, TextInput, Title } from '@mantine/core'

function RoomEntry({ errorMessage, initialPasscode = '', isSubmitting, roomId, onBackHome, onJoinRoom }) {
  const [displayName, setDisplayName] = useState('')
  const [passcode, setPasscode] = useState(initialPasscode)
  const trimmedDisplayName = displayName.trim()
  const trimmedPasscode = passcode.trim()

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (!trimmedDisplayName) {
      return
    }

    await onJoinRoom({
      displayName: trimmedDisplayName,
      roomId,
      passcode: trimmedPasscode,
    })
  }

  return (
    <main className="workbench workbench--center">
      <Paper component="section" className="panel direct-entry-panel" withBorder radius="md" shadow="xs">
        <Stack gap="md">
          <div className="panel-header panel-header--horizontal">
            <div>
              <Text className="eyebrow">加入会议室</Text>
              <Title order={1}>输入昵称后进入房间</Title>
              <Text className="section-copy">
                这是一个可分享的会议链接。为了让成员和 Agent 正确识别你的发言，请先填写本次会议里的显示名称。
              </Text>
            </div>
            <Button variant="default" type="button" onClick={onBackHome}>
              返回入口
            </Button>
          </div>

          <form className="form-stack room-entry-form" onSubmit={handleSubmit}>
            <Stack gap="md">
              <TextInput id="direct-room-id" label="房间 ID" value={roomId} readOnly />

              <TextInput
                id="direct-display-name"
                label="显示名称"
                description="这个名称会显示在会议消息和成员列表中。"
                autoFocus
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="例如：小明"
                disabled={isSubmitting}
                maxLength={40}
              />

              <PasswordInput
                id="direct-passcode"
                label="房间口令"
                description="没有口令的房间可以留空。"
                value={passcode}
                onChange={(event) => setPasscode(event.target.value)}
                placeholder="如果房间设置了口令，请在这里输入"
                disabled={isSubmitting}
                maxLength={80}
              />

              <Group justify="space-between" className="button-row">
                <Text className="helper-text">加入后会加载房间已有消息。</Text>
                <Button color="teal" type="submit" disabled={isSubmitting || !trimmedDisplayName}>
                  {isSubmitting ? '正在加入...' : '进入会议室'}
                </Button>
              </Group>
            </Stack>
          </form>

          {errorMessage ? <Alert color="red" variant="light">{errorMessage}</Alert> : null}
        </Stack>
      </Paper>
    </main>
  )
}

export default RoomEntry
