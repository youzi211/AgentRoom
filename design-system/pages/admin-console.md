# Page: AdminConsole — AgentRoom Design System

> 本文件定义 AdminConsole（管理后台外壳）的 UI 规范。

---

## 1. 页面目标

- 提供管理后台的统一导航和布局外壳
- 切换管理功能（会议管理 / Agent 管理 / 模型配置）
- 返回首页和退出登录

---

## 2. 信息层级

```
1. TopBar: 品牌 Logo + 导航标签
2. 内容区: 根据当前标签切换子页面
```

---

## 3. Layout

```
+--------------------------------------------------+
|  .workbench--admin (max-width: 1240px)             |
|  padding: 24px                                    |
|                                                    |
|  ┌──────────────────────────────────────────────┐ |
|  │  Admin Header                                │ |
|  │  ├── Logo + "AgentRoom"                      │ |
|  │  ├── [会议管理] [模型配置] [Agent 管理]       │ |
|  │  └── [会议入口] [退出后台]                     │ |
|  └──────────────────────────────────────────────┘ |
|                                                    |
|  ┌──────────────────────────────────────────────┐ |
|  │  Content Area                                │ |
|  │  ├── MeetingAdmin / AgentAdmin /             │ |
|  │  │   ModelProfileAdmin                       │ |
|  └──────────────────────────────────────────────┘ |
+--------------------------------------------------+
```

---

## 4. 核心组件

| 组件 | 规范 |
|------|------|
| 品牌 Logo | Avatar + Sparkles 图标 + "AgentRoom" |
| 导航按钮 | Mantine Button, variant="light" (active) / "subtle" (inactive), color="teal" (active) / "gray" (inactive) |
| 退出按钮 | variant="subtle" color="gray" |
| 内容容器 | 普通 div，不套 Paper |

---

## 5. Spacing

| 区域 | 间距 |
|------|------|
| 整体容器 padding | 24px |
| 导航按钮间距 | 8px |
| 内容区与顶栏间距 | 16px |
| 内容区内边距 | 0（由子页面控制） |

---

## 6. 状态

- 导航按钮有激活状态指示
- 无 loading 状态（页面切换即时）
- 无需 empty 状态
- 错误由子页面处理

---

## 7. 响应式

| 断点 | 调整 |
|------|------|
| mobile | 导航按钮折叠到汉堡菜单 |
| tablet | 导航按钮保持但不换行，内容区正常 |
