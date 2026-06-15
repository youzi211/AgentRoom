import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const cases = [
  ['AgentAdmin.jsx', ['Agent 管理', '管理控制台', '角色模板', '确认删除 Agent']],
  ['AgentRoster.jsx', ['可用 Agent', '暂无可用 Agent', '正在思考']],
  ['ChatRoom.jsx', ['房间 ID：', '复制房间 ID', '会议上下文', '实时讨论']],
  ['FocusTimeline.jsx', ['会议焦点', '发送消息后，AI 会自动提取会议焦点。', '刚刚']],
  ['JoinScreen.jsx', ['协作会议', '创建会议室', '加入会议室', '房间口令']],
  ['KnowledgePanel.jsx', ['上传 .md', '当前只支持上传 .md 文件。', '加载知识文档失败。']],
  ['MeetingMinutesPanel.jsx', ['会议产物', '生成纪要', '导出 Markdown', '预览草稿']],
  ['meetingMinutes.js', ['会议纪要', '基本信息', '消息记录', '暂无消息。']],
  ['MessageComposer.jsx', ['输入消息，使用 @ 提及 Agent 参与讨论。', '发送', 'Enter 发送，Shift + Enter 换行']],
  ['MessageList.jsx', ['开始一次协作会议', '对话', '正在思考...']],
  ['NotFound.jsx', ['这个页面不存在', '返回入口']],
  ['ParticipantList.jsx', ['在线成员', '暂无成员在线', '刚刚']],
  ['RoomEntry.jsx', ['加入会议室', '输入昵称后进入房间', '显示名称', '进入会议室']],
  ['../App.jsx', ['创建房间失败，请稍后重试。', '加入房间失败，请稍后重试。', '会议室']],
  ['../api/roomClient.js', ['请求失败，请稍后重试。', '导出会议纪要失败，请稍后重试。']],
]
const forbiddenMarkers = ['\uFFFD', '锛', '銆', '鈥', '锟']

test('user-facing copy is stored as readable Chinese', () => {
  for (const [relativePath, expectedSnippets] of cases) {
    const source = readFileSync(new URL(relativePath, import.meta.url), 'utf8')
    expectedSnippets.forEach((snippet) => {
      assert.match(source, new RegExp(escapeRegExp(snippet)), `${relativePath} should contain "${snippet}"`)
    })
  }
})

test('user-facing source files do not contain common mojibake markers', () => {
  for (const [relativePath] of cases) {
    const source = readFileSync(new URL(relativePath, import.meta.url), 'utf8')
    forbiddenMarkers.forEach((marker) => {
      assert.doesNotMatch(source, new RegExp(escapeRegExp(marker)), `${relativePath} should not contain mojibake marker "${marker}"`)
    })
  }
})

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
