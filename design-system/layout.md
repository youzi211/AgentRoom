# Layout — AgentRoom Design System

> 本文件定义 AgentRoom 的页面布局系统。

---

## 1. App Shell

```
+----------------------------------------------------------+
|  Header / TopBar (可选)                                   |
|  height: auto, padding: 12px                             |
+----------------------------------------------------------+
|  Sidebar (L)  |  Main Content Area       |  Sidebar (R)  |
|  width: 270px |  flex: 1                |  width: 320px |
|  max-height:  |  overflow-y: auto       |  max-height:  |
|  calc(100vh-) |                         |  calc(100vh-) |
+----------------------------------------------------------+
```

---

## 2. 布局模式

### Entry 模式

用于 JoinScreen、AdminConsole 等页面。

```
+--------------------------------------------------+
|  .workbench (max-width: 1240px, margin: 0 auto)   |
|  padding: 24px                                    |
|                                                    |
|  ┌──────────────────────────────────────────────┐ |
|  │  Header / App Bar                            │ |
|  │  ├── 品牌 Logo  │  导航项  │  操作按钮      │ |
|  └──────────────────────────────────────────────┘ |
|                                                    |
|  ┌──────────────────────────────────────────────┐ |
|  │  Content Area                                │ |
|  │  ├── Hero / 标题区                           │ |
|  │  ├── 表单 / 操作区                           │ |
|  │  └── 信息展示区                               │ |
|  └──────────────────────────────────────────────┘ |
+--------------------------------------------------+
```

### Chat 模式

用于 ChatRoom 页面。

```
+----------------------------------------------------------+
|  .chat-workbench (max-width: 1450px, height: 100vh)       |
|  padding: 12px                                            |
|                                                            |
|  ┌──────────────────────────────────────────────────────┐ |
|  │  TopBar                                             │ |
|  │  ├── 房间信息  │  状态  │  参与者  │  操作按钮       │ |
|  └──────────────────────────────────────────────────────┘ |
|                                                            |
|  ┌──────────┬────────────────────────┬──────────────────┐ |
|  │  Left    │  Main Content          │  Right Panel     │ |
|  │  Panel   │                        │                  │ |
|  │  270px   │  ┌──────────────────┐  │  320px           │ |
|  │          │  │  MessageList     │  │  ├── AgentRoster │ |
|  │  ├──     │  │  (flex: 1)       │  │  ├── Activity    │ |
|  │  Agent   │  └──────────────────┘  │  ├── Focus       │ |
|  │  Roster  │  ┌──────────────────┐  │  └── Knowledge   │ |
|  │          │  │  Composer        │  │                  │ |
|  │          │  └──────────────────┘  │                  │ |
|  └──────────┴────────────────────────┴──────────────────┘ |
+----------------------------------------------------------+
```

### Center 模式

用于 RoomGateway 的 loading/error 状态。

```
+--------------------------------------------------+
|  .workbench--center                               |
|  display: flex, justify-content: center           |
|  align-items: center                              |
|                                                    |
|  ┌──────────────────────────────────────────────┐ |
|  │  Single Panel (max-width: 520px)             │ |
|  │  ├── 标题                                    │ |
|  │  ├── 描述/状态                                │ |
|  │  └── 操作按钮                                 │ |
|  └──────────────────────────────────────────────┘ |
+--------------------------------------------------+
```

---

## 3. 容器定义

### 3.1 Workbench

```css
.workbench {
  width: min(1240px, 100%);
  min-height: 100vh;
  margin: 0 auto;
  padding: 24px;
}
```

### 3.2 Chat Workbench

```css
.chat-workbench {
  display: flex;
  width: min(1450px, 100%);
  height: 100vh;
  margin: 0 auto;
  flex-direction: column;
  padding: 12px;
  overflow: hidden;
}
```

### 3.3 Sidebar Section

```css
.sidebar-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
```

---

## 4. Panels

### 4.1 Panel 定义

Panel 是内容容器，**不是** Card。不要使用 `withBorder` 和 `shadow`。

```css
/* 正确 — 普通容器 */
.panel {
  background: var(--surface);
  border-radius: var(--radius-sm);
}

/* 错误 — 不要套 Paper+withBorder */
/* <Paper withBorder radius="md"> — 只在需要 Card 语义时使用 */
```

### 4.2 Panel Header

```css
.panel-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 20px;
}

.panel-header--horizontal {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}
```

---

## 5. 响应式布局

### 5.1 断点

| 断点 | 宽度 | 布局策略 |
|------|------|---------|
| mobile | < 768px | 单栏，侧栏折叠到抽屉 |
| tablet | 768-1024px | 两栏，隐藏左侧栏 |
| desktop | 1024-1440px | 完整三栏 |
| wide | > 1440px | 保持 max-width |

### 5.2 ChatRoom 响应式

```
mobile (< 768px):
┌──────────────────────┐
│  TopBar (简化)       │
├──────────────────────┤
│                      │
│  MessageList (全宽)  │
│                      │
├──────────────────────┤
│  Composer            │
└──────────────────────┘
  侧栏通过 Drawer 访问

tablet (768-1024px):
┌──────────────────────────────┐
│  TopBar                      │
├──────────────────┬───────────┤
│  MessageList     │  Right    │
│  (flex: 1)       │  Panel    │
│                  │  320px    │
├──────────────────┴───────────┤
│  Composer                    │
└──────────────────────────────┘
  左侧栏通过 Drawer 访问

desktop (1024-1440px):
┌────────┬─────────────┬──────────┐
│  Left  │  Message    │  Right   │
│  Panel │  List       │  Panel   │
│  270px │  + Composer │  320px   │
└────────┴─────────────┴──────────┘
  完整三栏
```
