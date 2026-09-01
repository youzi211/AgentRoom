# Spacing — AgentRoom Design System

> 本文件定义 AgentRoom 的间距系统，基于 4px 基础网格。

---

## 1. 基础网格

采用 **4px 网格**，所有间距值必须是 4 的倍数：

```
4   8   12   16   20   24   32   40   48
```

---

## 2. 间距层级

| 层级 | 值 | 别名 | 使用场景 |
|------|----|------|---------|
| xs | 4px | 紧凑 | Badge 组、内联标签、极小区块 |
| sm | 8px | 密集 | 按钮组、标签组、密集排列 |
| md | 16px | 标准 | 组件间距、表单元素间距 |
| lg | 24px | 宽松 | 页面内边距、Section 间距 |
| xl | 32px | 大 | 大区块间距、页面顶部 |
| 2xl | 40px | 超大 | 首页 Hero 区域 |
| 3xl | 48px | 特大 | 页面底部留白 |

---

## 3. 页面间距

| 页面 | 容器 | 内边距 |
|------|------|--------|
| JoinScreen | `.workbench` | 24px |
| AdminConsole | `.workbench--admin` | 24px |
| ChatRoom | `.chat-workbench` | 12px（全屏布局） |
| RoomGateway | `.workbench--center` | 24px |

---

## 4. 组件间距

### 4.1 Section 间距

```
Section 1
  └── gap: 24px ──→ Section 2
                      └── gap: 24px ──→ Section 3
```

### 4.2 Panel 内边距

```
┌─────────────────────────────┐
│  padding: 20px              │
│  ┌─── 16px ───┐            │
│  │ title      │ 12px       │
│  │ content    │            │
│  └────────────┘            │
└─────────────────────────────┘
```

### 4.3 列表项间距

```
┌──────────────────────────┐
│  List Item 1             │
│  └── padding: 12px 16px  │
├──  gap: 4px              │
│  List Item 2             │
│  └── padding: 12px 16px  │
└──────────────────────────┘
```

---

## 5. ChatRoom 布局间距

```
┌────────────────────────────────────────────────┐
│  TopBar (padding: 12px, margin-bottom: 10px)    │
├────────┬───────────────────────┬────────────────┤
│  Left  │   Main Content        │  Right         │
│  Panel │   (flex: 1)           │  Panel         │
│  270px │   padding: 0 12px     │  320px         │
│        │                       │                │
│  gap:  │   gap: 12px           │  gap:          │
│  12px  │   (between sections)  │  12px          │
└────────┴───────────────────────┴────────────────┘
```

---

## 6. 当前项目问题

| 问题 | 位置 | 建议 |
|------|------|------|
| `gap: 22px` | `.workbench--entry` | 改为 24px |
| `gap: 18px` | `.workbench--admin` | 改为 16px 或 24px |
| `gap: 14px` | `.chat-topbar` | 改为 16px |
| `gap: 9px` | `.chat-topbar-subtitle` | 改为 8px |
| `gap: 8px` | `.chat-topbar-actions` | 保留 8px ✅ |
| `margin-bottom: 10px` | `.chat-topbar` | 改为 12px 或 8px |
| `padding: 12px` | `.chat-workbench` | 保留 12px（全屏布局） |
| 多个 `padding: 24px` | 页面容器 | 保留 24px ✅ |
