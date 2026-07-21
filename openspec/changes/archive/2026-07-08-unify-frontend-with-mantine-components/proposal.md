## Why

当前前端页面由多套自定义按钮、表单、面板和列表样式混合组成，入口页、会议室页、管理后台和只读页面之间的视觉语言不一致，继续扩展会增加维护成本并放大交互细节差异。

项目已经引入 Mantine 8.3.14，适合用现成组件统一页面结构、表单控件、按钮、徽标、弹窗和数据展示，让用户在所有页面中获得一致的工作台体验。

## What Changes

- 将前端主要页面统一到 Mantine 组件体系，优先使用 `Paper`、`Group`、`Stack`、`Button`、`Badge`、`Avatar`、`Text`、`Title`、`TextInput`、`Textarea`、`Select`、`Table`、`Modal`、`Tabs`、`ScrollArea` 等现有组件。
- 统一入口页、会议室页、管理后台、直接加入页、只读页、404 页及会议详情/纪要历史弹层的视觉风格、控件尺寸、按钮状态和信息层级。
- 保留业务必要的自定义 CSS，例如三栏工作台布局、可拖拽宽度、消息气泡、自动补全定位和响应式约束；移除或缩减不再使用的通用自定义控件样式。
- 不改变后端 API、WebSocket 协议、路由语义、房间生命周期、管理员鉴权、Agent 触发逻辑或数据模型。
- 不新增 UI 组件库；继续使用现有 `@mantine/core` 与 `@mantine/hooks` 依赖。

## Capabilities

### New Capabilities

- `frontend-style-consistency`: 约束前端页面应使用 Mantine 组件库统一布局、表单、按钮、徽标、列表、弹层和数据展示，并保持页面间风格一致。

### Modified Capabilities

无。

## Impact

- 主要影响 `frontend/src/components/` 下的页面和面板组件，包括 `AdminConsole`、`AdminGate`、`MeetingAdmin`、`AgentAdmin`、`ChatRoom` 相关子面板、`RoomEntry`、`RoomGateway`、`RoomReadOnly`、`NotFound`、`MeetingRoomDetail` 和 `MinutesHistory`。
- 影响 `frontend/src/styles.css` 与 `frontend/src/chat-room.css` 中通用控件、面板、表单、徽标和页面布局相关样式。
- 需要补充或更新前端源代码级测试，验证关键页面继续使用 Mantine 组件并保留原业务入口。
- 验证重点为现有 Node 测试与 `npm --prefix frontend run build`。
