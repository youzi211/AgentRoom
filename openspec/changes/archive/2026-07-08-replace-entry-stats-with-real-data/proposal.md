## Why

入口页左侧统计卡片当前展示的是前端硬编码数字，用户会误以为这些是系统实时状态。该卡片应该展示来自后端的真实统计，或者在加载失败时明确表现为不可用，而不是继续显示假数据。

## What Changes

- 新增入口页统计摘要能力，后端返回当前进行中会议数、今日会议数、知识来源数、Agent 角色数。
- 前端 `JoinScreen` 不再使用硬编码 `ENTRY_STATS` 数值，而是从后端摘要接口加载真实数据。
- 摘要接口应为普通入口页可访问，不依赖 admin key。
- 加载中和加载失败状态不得回退到假数据；可显示占位、0 或错误态。
- 增加后端和前端回归测试，防止入口页重新引入硬编码统计。

## Capabilities

### New Capabilities

### Modified Capabilities
- `agent-runtime-entry-reliability`: 增加入口页真实统计摘要要求，保证入口页展示的状态数据来自后端。

## Impact

- 后端 API、service/store 查询路径需要支持入口统计摘要。
- 前端 `frontend/src/components/JoinScreen.jsx` 和 `frontend/src/api/roomClient.js` 需要加载并渲染真实统计。
- 测试范围包括 Go API/service 测试、前端源码/行为测试、前端构建和 OpenSpec 校验。
