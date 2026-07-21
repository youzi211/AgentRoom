## 1. 基线与测试护栏

- [x] 1.1 盘点 `frontend/src/components/` 中仍使用原生 `button`、`input`、`textarea`、`select`、自定义面板和自定义徽标的页面，确认迁移范围不包含后端/API 变更。
- [x] 1.2 新增或更新前端源代码级测试，覆盖关键页面继续导入并使用 Mantine 组件，且主要自定义控件类不再作为新增通用控件入口。

## 2. 管理后台统一

- [x] 2.1 将 `AdminGate` 的页面外壳、登录表单、错误提示和操作按钮迁移到 Mantine 容器、表单和按钮组件。
- [x] 2.2 将 `AdminConsole` 的顶部导航和页面布局迁移到 Mantine 组件，保持会议管理与 Agent 管理切换行为不变。
- [x] 2.3 将 `MeetingAdmin` 的筛选、列表/表格、状态徽标、批量/单项操作和会议详情入口迁移到 Mantine 组件。
- [x] 2.4 将 `AgentAdmin` 的 Agent 列表、模板选择、启停状态、编辑表单、删除确认和知识库区域迁移到 Mantine 组件。

## 3. 会议室工作台统一

- [x] 3.1 补齐 `ChatRoom` 左侧会议上下文、房主控制、顶部提示和工具栏中的 Mantine 控件替换，保留三栏拖拽布局。
- [x] 3.2 将 `KnowledgePanel`、`MeetingMinutesPanel`、`AgentActivityPanel`、`FocusTimeline`、`ParticipantList` 等侧栏面板迁移到 Mantine 容器、徽标、列表、按钮和状态文本。
- [x] 3.3 将 `MessageComposer` 的文本域、发送按钮和提及自动补全弹层的通用控件迁移到 Mantine，保留现有键盘交互和提及插入逻辑。
- [x] 3.4 将 `MessageList` 的空状态、头像、徽标、报告下载按钮和知识来源标记迁移到 Mantine 组件，保留消息角色区分和思考态展示。

## 4. 直接访问、只读与弹层统一

- [x] 4.1 将 `RoomEntry`、`RoomGateway`、`RoomReadOnly` 和 `NotFound` 的页面容器、表单、按钮、状态提示和只读信息面板迁移到 Mantine。
- [x] 4.2 将 `MeetingRoomDetail` 的会议详情、历史消息加载、纪要预览和操作按钮迁移到 Mantine 组件。
- [x] 4.3 将 `MinutesHistory` 的弹层、版本列表、编辑文本域、生成/导出/保存操作和状态反馈迁移到 Mantine `Modal`、列表、文本域和按钮组件。

## 5. 样式收敛与验证

- [x] 5.1 收窄或移除 `frontend/src/styles.css` 与 `frontend/src/chat-room.css` 中被 Mantine 替代的通用按钮、表单、面板、徽标和弹窗样式，保留业务布局、响应式和特殊交互样式。
- [x] 5.2 运行相关前端 Node 测试，至少覆盖新增风格一致性测试以及现有会议室、纪要、消息来源、角色集、房间访问相关测试。
- [x] 5.3 运行 `npm --prefix frontend run build`，确认生产构建通过。
- [x] 5.4 运行 `openspec validate unify-frontend-with-mantine-components`，确认变更规格有效。
