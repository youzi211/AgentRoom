## Context

入口页 `JoinScreen` 的统计卡片目前是硬编码展示值，和后端真实状态无关。用户已经在浏览器中指出这些数字并不是真实数据，因此入口页需要从后端获取统一统计摘要。

现有系统已经有：
- `GET /api/recent-rooms` 用于普通入口页读取最近活跃会议。
- `GET /api/agents` 用于读取 Agent 列表。
- 后端 store 中已有 rooms、agents、knowledge documents 等数据来源。

本次变更横跨后端统计查询、API contract、前端入口页渲染和测试，因此需要设计文档固定统计口径。

## Goals / Non-Goals

**Goals:**
- 用后端真实统计替换入口页硬编码数字。
- 提供一个普通入口页可访问的摘要接口，避免前端串联多个接口或访问 admin API。
- 明确 4 个指标口径：进行中、今日会议、知识来源、Agent 角色。
- 在加载失败时避免显示假数据。
- 增加测试覆盖，防止硬编码数字回归。

**Non-Goals:**
- 不做复杂 analytics、趋势图或历史统计。
- 不引入缓存层、后台任务或新外部依赖。
- 不改变 admin 后台统计、房间权限或知识库上传流程。
- 不把统计拆分成 room-level / agent-level 的高级维度；第一版只做入口页总览。

## Decisions

### 新增入口摘要接口

新增 `GET /api/entry-summary`，返回入口页所需的 4 个统计值：
- `activeRooms`: 当前 active rooms 数量。
- `todayRooms`: 服务器本地日期当天创建的 rooms 数量。
- `knowledgeDocuments`: 知识库文档总数。
- `enabledAgents`: enabled agents 数量。

选择单一摘要接口，而不是前端组合多个 API。原因是统计口径属于后端业务语义，前端只负责展示；同时减少入口页请求数量，避免误用 admin-only API。

### 统计口径由后端统一计算

后端 service/store 层提供摘要查询。MySQL 实现可用聚合查询；测试 store 用内存集合计算同等口径。

今日会议使用服务器本地日期边界。该选择与“今日会议”中文 UI 语义一致，也避免前端和后端时区不一致导致显示不同数字。后续若产品需要跨时区用户视角，可再扩展请求参数或用户设置。

### 前端展示真实值或明确不可用

`JoinScreen` 加载摘要接口后渲染真实数字。加载中显示占位值（例如 `--`），失败时显示 `0` 或错误提示，但不得保留硬编码业务数字。

### 不复用 recent rooms 响应推导统计

`recent-rooms` 是列表投影，受 limit 限制，不适合推导总数。入口摘要必须从后端统计查询返回总量。

## Risks / Trade-offs

- [Risk] 统计接口每次入口页加载都查询数据库 → Mitigation: 只做 4 个轻量聚合，第一版不加缓存；如后续数据量上升再引入缓存。
- [Risk] “今日会议”时区口径可能与用户所在地不同 → Mitigation: 第一版明确使用服务器本地日期，测试固定时间边界；后续再做用户时区。
- [Risk] 知识来源总数可能包含 room 和 agent 两类文档，用户不清楚细分 → Mitigation: 第一版定义为知识文档总数，UI 文案保持“知识来源”；后续可扩展分项。
- [Risk] 前端加载失败显示 0 会被误解为真实 0 → Mitigation: 优先使用占位或错误态；如果必须显示数字，也要有加载失败提示。

## Migration Plan

1. 增加后端 entry summary contract、service/store 查询和 API route。
2. 增加后端测试覆盖真实统计口径和非 admin 访问。
3. 更新前端 `roomClient.js` 和 `JoinScreen.jsx`，删除硬编码统计值。
4. 增加前端测试，断言入口统计来自 `listEntrySummary`，不再保留 `6/12/24/18` 这组假数据。
5. 运行 Go 测试、前端相关测试、前端构建和 OpenSpec 校验。
