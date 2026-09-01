# Interaction — AgentRoom Design System

> 本文件定义 AgentRoom 的交互模式和动画规范。

---

## 1. 过渡动画

### 统一配置

```css
--transition-fast: 150ms ease;
--transition-normal: 200ms ease;
```

所有交互状态变化使用 `transition` 属性：

```css
.element {
  transition: background-color var(--transition-fast),
              color var(--transition-fast),
              box-shadow var(--transition-fast),
              transform var(--transition-fast),
              opacity var(--transition-fast);
}
```

---

## 2. 状态反馈

### Hover

| 元素 | 反馈 | 时间 |
|------|------|------|
| 按钮 (filled) | 背景色加深 shade 6→7 | 150ms |
| 按钮 (light) | 背景色加深 shade 0→1 | 150ms |
| 按钮 (subtle) | 背景色变为 surface-muted | 150ms |
| 列表项 | 背景色变为 surface-hover | 150ms |
| 消息卡片 | 轻微阴影变化 | 150ms |
| 链接 | 颜色变为 brand-hover | 150ms |
| 可点击卡片 | 轻微上移 + 阴影加深 | 200ms |

### Active (Press)

```css
.button:active {
  transform: scale(0.98);
}
```

### Focus

```css
/* Mantine 默认 focus ring */
*:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 2px;
}
```

### Disabled

```css
*:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}
```

---

## 3. Loading 状态

### 按钮 Loading

```jsx
<Button loading>加载中...</Button>
```

### 页面 Loading

- 主内容区使用 Mantine `Skeleton` 组件
- 列表使用 `Skeleton` 行
- 全页 loading 使用居中 Spinner 或 Paper

### 消息 Loading (Thinking)

Agent 正在思考时，显示 Thinking 指示器：

```html
<div class="thinking-indicator">
  <span class="typing-dot" />
  <span class="typing-dot" />
  <span class="typing-dot" />
  正在思考...
</div>
```

---

## 4. Empty 状态

### 规范

所有列表/表格必须有 Empty State，包含：

1. 提示文字（说明当前为什么空）
2. 操作建议（指导用户下一步做什么）

```html
<div class="empty-state">
  <Text class="eyebrow">标题</Text>
  <Title order={2}>空状态说明</Title>
  <Text class="muted-text">操作建议文字</Text>
</div>
```

### 当前 Empty State 位置

| 组件 | 当前状态 | 建议 |
|------|---------|------|
| MessageList | ✅ 有空状态 | 保留 |
| AgentRoster | ✅ 有文本 | 可增加引导操作 |
| MeetingAdmin (空列表) | 无专门空状态 | 需要增加 |
| ActivityPanel | ✅ 有 | 保留 |
| KnowledgePanel | 无 | 需要增加 |

---

## 5. Error 状态

### 规范

- 使用 Mantine `Alert` 组件
- 错误提示使用 `color="red" variant="light"`
- 包含错误消息和重试操作
- 使用 `role="alert"` 确保无障碍

```jsx
<Alert icon={<AlertCircle size={16} />} color="red" variant="light">
  {errorMessage}
</Alert>
```

### 全局错误处理

- API 请求失败时显示 Alert
- 网络断开时在顶栏显示连接状态指示
- WebSocket 断开时在顶栏显示状态变化

---

## 6. 状态指示器

### 连接状态点

```css
.status-dot { width: 9px; height: 9px; border-radius: 999px; }
.status-dot--connected  { background: var(--success); }
.status-dot--connecting { background: var(--warning); }
.status-dot--disconnected { background: var(--danger); }
.status-dot--offline { background: var(--text-muted); }
```

### 房间状态指示

| 状态 | 指示方式 |
|------|---------|
| active | 绿色 dot + "进行中" |
| closed | 灰色 dot + "已关闭" |
| archived | 灰色 dot + "已归档" |

---

## 7. 拖拽交互

### ResizeHandle

- 用于 ChatRoom 左右面板宽度调整
- hover 时显示拖拽指示器
- 拖拽区域宽度: 4px
- 使用 `cursor: col-resize`

---

## 8. 复制与粘贴

### 复制房间 ID

```jsx
<Button onClick={() => navigator.clipboard.writeText(roomId)}>
  <Copy size={16} />
</Button>
```

- 复制成功后显示短暂反馈（"已复制" 文字或图标变化）
- 反馈时长: 1.5s 后恢复

---

## 9. 禁止的交互模式

- 不使用自动播放动画
- 不使用页面过渡动画（页面切换应即时）
- 不使用拖拽排序（除非明确需求）
- 不使用无限滚动（使用分页或"加载更多"）
- 不使用自动聚焦弹窗
