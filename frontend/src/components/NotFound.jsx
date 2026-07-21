import { Button, Paper, Text, Title } from '@mantine/core'

function NotFound({ onBackHome }) {
  return (
    <main className="workbench workbench--center">
      <Paper component="section" className="panel direct-entry-panel" withBorder radius="md" shadow="xs">
        <div className="panel-header panel-header--horizontal">
          <div>
            <Text className="eyebrow">404</Text>
            <Title order={1}>这个页面不存在</Title>
            <Text className="section-copy">请检查链接是否完整，或返回会议入口重新创建、加入房间。</Text>
          </div>
          <Button color="teal" type="button" onClick={onBackHome}>
            返回入口
          </Button>
        </div>
      </Paper>
    </main>
  )
}

export default NotFound
