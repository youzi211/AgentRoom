# Page: ChatRoom — AgentRoom Design System

> 本文件是 ChatRoom 页面的最详细规范。
> ChatRoom 是 AgentRoom 的核心页面，必须重点关注。

---

## 1. 页面目标

- 提供实时多人 + 多 Agent 协作会议体验
- 人类通过输入消息与 Agent 交互
- 查看 Agent 活动、会议纪要、关注事项
- 管理会议生命周期（关闭/转移）

---

## 2. 信息层级

```
1. TopBar: 房间状态、参与者、操作按钮
2. 左侧栏: Agent 列表、参与者列表
3. 中央区: 消息历史、输入框
4. 右侧栏: Agent 活动、会议纪要、关注事项、知识库
```

### 视觉权重

| 区域 | 权重 | 说明 |
|------|------|------|
| 消息列表 | 最高 | 核心内容区域 |
| Composer | 高 | 主要交互入口 |
| Agent 列表 | 中 | 辅助信息 |
| 活动/纪要 | 中 | 辅助信息 |
| TopBar | 低 | 不干扰主要内容 |

---

## 3. Layout

```
+----------------------------------------------------------+
|  TopBar                                                   |
|  ┌──────────────────────────────────────────────────────┐ |
|  │  [status] 房间名  │  参与者  │  操作按钮             │ |
|  └──────────────────────────────────────────────────────┘ |
|                                                            |
|  ┌───────────┬─────────────────────────┬─────────────────┐ |
|  │  Left     │  Main Content           │  Right Panel    │ |
|  │  Panel    │                         │  320px          │ |
|  │  270px    │  ┌───────────────────┐  │                 │ |
|  │           │  │  MessageList      │  │  ├── Activity   │ |
|  │  ├──      │  │  (flex: 1,       │  │  ├── Minutes    │ |
|  │  Agent    │  │   overflow-y:    │  │  ├── Focus      │ |
|  │  Roster   │  │   auto)          │  │  └── Knowledge  │ |
|  │           │  └───────────────────┘  │                 │ |
|  │  ├──      │  ┌───────────────────┐  │                 │ |
|  │  Partici- │  │  Composer         │  │                 │ |
|  │  pant     │  │  (flex: none)     │  │                 │ |
|  │  List     │  └───────────────────┘  │                 │ |
|  └───────────┴─────────────────────────┴─────────────────┘ |
+----------------------------------------------------------+
```

---

## 4. TopBar 规范

### 结构

```html
<header class="chat-topbar">
  <!-- 左：房间信息 -->
  <div class="chat-room-meta">
    <span class="status-dot status-dot--connected" />
    <h2 class="chat-topbar-title">房间名称</h2>
    <span class="chat-identity">@用户名</span>
  </div>

  <!-- 中：参与者 -->
  <div class="chat-topbar-subtitle">
    <Badge>参与者数</Badge>
    <Badge>Agent 数</Badge>
  </div>

  <!-- 右：操作 -->
  <div class="chat-topbar-actions">
    <Select>转移 Owner</Select>
    <Button>关闭会议</Button>
    <Button>离开</Button>
  </div>
</header>
```

### 样式

```css
.chat-topbar {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px;
  margin-bottom: 12px;
  border-radius: var(--radius-md);
  background: var(--surface);
  border: 1px solid var(--border);
}

.chat-topbar-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-strong);
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 42vw;
}
```

---

## 5. 左侧栏规范

### 结构

```html
<aside class="chat-left-panel">
  <!-- Agent 列表 -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>可用 Agent</h3>
      <Badge variant="light" color="teal">{count}</Badge>
    </div>
    <ul class="sidebar-list">
      <li class="agent-item">
        <!-- 不使用 Paper -->
        <div class="agent-item-main">
          <Avatar />
          <div class="agent-item-info">
            <Text class="agent-item-name">名称</Text>
            <Text class="agent-item-role">角色</Text>
          </div>
          <Button variant="light" color="teal" size="xs">@mention</Button>
        </div>
        <Text class="agent-item-description">描述</Text>
      </li>
    </ul>
  </section>

  <!-- 参与者列表 -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>参与者</h3>
    </div>
    <ul class="sidebar-list">
      <li class="participant-item">
        <Avatar />
        <span class="participant-name">名称</span>
        <Badge>Owner</Badge>
      </li>
    </ul>
  </section>
</aside>
```

### 列表项样式

```css
.agent-item,
.participant-item {
  padding: 12px 16px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background-color var(--transition-fast);
}

.agent-item:hover,
.participant-item:hover {
  background-color: var(--surface-hover);
}

.agent-item-main {
  display: flex;
  align-items: center;
  gap: 12px;
}
```

---

## 6. 中央区规范

### MessageList

```html
<section class="message-panel">
  <ul class="message-list">
    <li class="message-row message-row--own">
      <!-- 自己消息：右对齐 -->
      <Paper radius="md" class="message-card message-card--own">
        <div class="message-meta">
          <span class="message-author">我</span>
          <Badge variant="light" color="gray">我</Badge>
          <time>14:30</time>
        </div>
        <Text class="message-content">消息内容</Text>
      </Paper>
    </li>

    <li class="message-row message-row--agent">
      <!-- Agent 消息：左对齐 + 品牌色边条 -->
      <Avatar />
      <Paper radius="md" class="message-card message-card--agent">
        <div class="message-meta">
          <span class="message-author">Agent 名</span>
          <Badge variant="light" color="teal">Agent</Badge>
          <time>14:31</time>
        </div>
        <Text class="message-content">消息内容</Text>
        <div class="message-sources">
          <Badge variant="light" color="gray">来源</Badge>
        </div>
      </Paper>
    </li>
  </ul>
</section>
```

### 消息样式

```css
.message-card--own {
  background: var(--brand-soft);
  border: none;
}

.message-card--agent {
  background: var(--surface);
  border-left: 3px solid var(--brand);
}

.message-card--other {
  background: var(--surface-muted);
  border: none;
}

.message-row--own {
  justify-content: flex-end;
}

.message-row--agent,
.message-row--other {
  justify-content: flex-start;
}
```

### Composer

```html
<div class="composer">
  <TextInput
    placeholder="输入消息，@AgentName 提及 Agent..."
    radius="sm"
    onKeyDown={handleKeyDown}
  />
  <Button variant="light" color="teal">
    <Send size={16} />
  </Button>
</div>
```

---

## 7. 右侧栏规范

### 结构

```html
<aside class="chat-right-panel">
  <!-- Agent Activity -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>Agent 活动</h3>
    </div>
    <div class="activity-list">
      <!-- 活动条目 -->
    </div>
  </section>

  <!-- 会议纪要 -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>会议纪要</h3>
      <Button variant="light" color="teal" size="xs">生成</Button>
    </div>
    <div class="minutes-content">
      <!-- 纪要内容 -->
    </div>
  </section>

  <!-- 关注事项 -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>关注事项</h3>
    </div>
    <div class="focus-list">
      <!-- 关注事项条目 -->
    </div>
  </section>

  <!-- 知识库 -->
  <section class="sidebar-section">
    <div class="sidebar-header">
      <h3>知识库</h3>
    </div>
    <div class="knowledge-list">
      <!-- 文档条目 -->
    </div>
  </section>
</aside>
```

---

## 8. 状态

### Loading

| 场景 | 展示 |
|------|------|
| 页面加载 | Skeleton 占位 |
| 消息加载 | 历史的 skeleton 行 |
| Agent 思考 | 思考指示器动画 |

### Empty

| 场景 | 展示 |
|------|------|
| 无消息 | "开始一次协作会议，@AgentName 提及 Agent" |
| 无 Agent | "暂无可用 Agent" |
| 无活动 | "暂无活动记录" |
| 无纪要 | "尚未生成会议纪要" |

### Error

| 场景 | 展示 |
|------|------|
| 消息发送失败 | Composer 上方显示错误提示 |
| 连接断开 | TopBar 状态 dot 变红 + 提示文字 |
| 纪要生成失败 | Alert 提示 |

---

## 9. 响应式

| 断点 | 调整 |
|------|------|
| mobile | 仅显示消息列表 + Composer，侧栏通过 Drawer 访问 |
| tablet | 显示消息列表 + 右侧栏，左侧栏通过 Drawer 访问 |
| desktop | 完整三栏布局 |

---

## 10. Do / Don't

### Do

- 消息列表使用背景色区分角色，而不是边框
- 侧栏内容使用列表项（非卡片）
- 保持 Composer 固定在底部
- 新消息自动滚动到底部

### Don't

- 不要在每个 Agent 条目上使用 Paper
- 不要在消息卡片上使用 `withBorder`
- 不要在顶栏上使用 `backdrop-filter`
- 不要使用大面积渐变背景
- 不要让消息卡片使用阴影
