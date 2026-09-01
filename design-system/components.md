# Components — AgentRoom Design System

> 本文件定义 AgentRoom 前端核心组件的使用规范。
> 所有组件基于 Mantine UI v8，图标使用 Lucide React。

---

## 1. Button

### Variant 选择

| Variant | 使用场景 | 颜色 | Radius |
|---------|---------|------|--------|
| `filled` | 主要操作（创建、提交、保存） | `teal` | sm |
| `light` | 次要操作（@mention、编辑、删除） | `teal` / `gray` | sm |
| `subtle` | 第三级操作（导航、取消、返回） | `gray` | sm |
| `outline` | 边框按钮（低频使用） | `teal` | sm |

### 尺寸

| 尺寸 | 使用场景 | 图标 |
|------|---------|------|
| `xs` | 紧凑按钮、内联操作 | `size={14}` |
| `sm` | 默认操作 | `size={16}` |
| `md` | 主要操作 | `size={16}` |
| `lg` | Hero 操作 | `size={18}` |

### 状态

- hover: 颜色加深（filled 变 shade 7，light 变 shade 6）
- active: 轻微缩小 (`transform: scale(0.98)`)
- disabled: 透明度 0.5，禁止光标
- loading: 显示 Mantine Loader，禁用交互

### 规则

- 避免在同一区域使用超过 2 个 filled 按钮
- 主要/次要按钮使用 `leftSection` 加图标
- 导航按钮使用 `variant="subtle" color="gray"`
- 危险操作（删除）使用 `color="red" variant="light"`

---

## 2. Input / TextInput

### 规范

| 属性 | 值 |
|------|----|
| radius | sm (4px) |
| size | sm (默认) |
| variant | `default` |
| label | Label 层级 (0.85rem, 600) |
| description | Body Secondary 层级 |
| error | 红色文字 + red border |

### 状态

- focus: 品牌色 outline ring
- error: 红色边框 + 错误提示
- disabled: 透明度 0.5
- placeholder: `--text-muted` 颜色

---

## 3. Select

- 使用 Mantine `Select` 组件
- 与 Input 保持相同的 radius 和 size
- 搜索功能的 Select 使用 `searchable`
- 多选使用 `Select` 的 `multiple` 属性

---

## 4. Badge

### Variant

| Variant | 使用场景 |
|---------|---------|
| `light` | 默认（标签、状态、计数） |
| `filled` | 强调状态（极少使用） |
| `dot` | 状态指示器 |

### 颜色

- 计数 Badge: `teal`
- 角色 Badge (Agent): `teal`
- 角色 Badge (人类): `gray`
- 状态 Badge: 语义色（success/warning/danger）
- 来源 Badge (知识库): `gray`

### 尺寸

- 默认使用 Mantine 默认 size
- 计数 Badge 可稍小

---

## 5. Avatar

### 规范

| 属性 | 值 |
|------|----|
| radius | sm (4px) |
| 默认颜色 | `teal`（Agent）/ `gray`（人类） |
| 文字 | 首字母大写 |

### 尺寸

| 场景 | 尺寸 |
|------|------|
| 消息头像 | sm (24px) |
| 列表项头像 | sm (24px) |
| 顶栏品牌头像 | md (30px) |
| 大头像 | lg (40px) |

---

## 6. Card / Paper

### 核心原则

**Layout / Panel ≠ Card**

- **容器/面板**：使用普通 `<div>` 或 `<section>`，不加 `withBorder` 和 `shadow`
- **独立信息实体**：使用 `<Paper withBorder radius="md">`

### 使用场景

| 应该用 Paper | 不应该用 Paper |
|-------------|----------------|
| 消息卡片（MessageList） | 页面容器（workbench） |
| 直接入口面板（RoomEntry） | 顶栏（topbar） |
| 配置文件卡片 | 侧栏容器（sidebar-section） |
| 表单区域 | 列表容器 |
| Modal/Drawer 内容 | 区块背景 |

### Paper 属性规范

| 场景 | withBorder | radius | shadow | 背景 |
|------|-----------|--------|--------|------|
| 消息卡片 | false | md | none | 见消息规则 |
| 入口面板 | true | md | sm | `--surface` |
| 表单区域 | false | sm | none | `--surface` |
| Modal 内容 | false | md | none | `--surface` |

---

## 7. Modal

- 使用 Mantine `Modal` 组件
- 标题使用 H3 层级
- 操作按钮使用 `Group justify="end"`
- 关闭按钮使用右上角 `x` 按钮
- 遮罩层点击不关闭（`closeOnClickOutside={false}`）

---

## 8. Table

- 使用 Mantine `Table` 组件
- 表头使用 Semibold (600) + Caption 层级
- 行 hover 时背景色变为 `--surface-hover`
- 避免过多边框，使用条纹或分割线
- 操作列右对齐

---

## 9. Message

### 消息角色区分

| 角色 | 对齐 | 背景 | 边框 | Avatar |
|------|------|------|------|--------|
| 自己 (own) | 右 | `--brand-soft` | 无 | "我" |
| Agent | 左 | `--surface` | 左侧品牌色边条 | 首字母 |
| 他人 (other) | 左 | `--surface-muted` | 无 | 首字母 |
| 系统 (system) | 居中 | 无 | 无 | 无 |

### 消息卡片属性

```jsx
// 自己消息
<Paper radius="md" shadow="none" style={{ background: "var(--brand-soft)" }}>
  // 右对齐
</Paper>

// Agent 消息
<Paper radius="md" shadow="none" style={{ background: "var(--surface)" }}>
  // 左侧 3px brand 色边条
  // 知识来源 chip
</Paper>

// 他人消息
<Paper radius="md" shadow="none" style={{ background: "var(--surface-muted)" }}>
  // 左对齐
</Paper>
```

---

## 10. Agent Item

### 规范

```html
<li class="agent-item">
  <div class="agent-item-main">
    <Avatar src={...} radius="sm" color="teal" />
    <div class="agent-item-info">
      <Text class="agent-item-name">名称</Text>
      <Text class="agent-item-role">角色</Text>
    </div>
    <Button variant="light" color="teal" size="xs">@mention</Button>
  </div>
  <Text class="agent-item-description">描述</Text>
</li>
```

### 属性

- **不使用** Paper 包裹每个 Agent 条目
- 使用普通 `<li>` 元素
- hover 时背景色变为 `--surface-hover`
- `border-radius: sm`（4px）
- `padding: 12px 16px`
- 描述文字使用 Body Secondary 层级

---

## 11. Participant Item

- 与 Agent Item 保持一致的视觉风格
- 不需要 @mention 按钮
- 当前用户标注"我"
- 房间 Owner 显示皇冠或 Owner 标签

---

## 12. Composer

### 规范

| 属性 | 值 |
|------|----|
| 组件 | Mantine TextInput 或自定义 div |
| radius | sm (4px) |
| 占位符 | "输入消息，@AgentName 提及 Agent..." |
| 发送按钮 | IconButton 或 Button, variant="light", color="teal" |
| 多行 | 支持 Shift+Enter 换行，Enter 发送 |

### Mention 支持

- 输入 `@` 时弹出 Agent 列表
- 选中后插入 `@AgentName` 文本
- 发送时保持 `@AgentName` 格式

---

## 13. Tabs / Navigation

### 顶部导航（AdminConsole）

- 使用 Mantine `Button` 组件模拟标签页
- 激活状态: `variant="light" color="teal"`
- 非激活: `variant="subtle" color="gray"`
- 间距: 8px 按钮间距

### 状态指示

- 连接状态使用 `status-dot` 类
- 绿色: 已连接
- 黄色: 连接中
- 红色: 断开
- 灰色: 离线
