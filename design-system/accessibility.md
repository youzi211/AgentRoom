# Accessibility — AgentRoom Design System

> 本文件定义 AgentRoom 前端的无障碍规范。

---

## 1. 色彩对比度

### 最低要求

| 文字类型 | 最小对比度 | 标准 |
|---------|-----------|------|
| 正文 (>= 18px) | 3:1 | WCAG AA |
| 正文 (< 18px) | 4.5:1 | WCAG AA |
| 大文字 (>= 24px) | 3:1 | WCAG AA |
| 辅助文字 | 3:1 | WCAG AA |

### 当前色值对比度检查

| 色值 | 背景 | 对比度 | 状态 |
|------|------|--------|------|
| `#0F172A` (text) | `#FFFFFF` (白色) | 12.6:1 | ✅ AAA |
| `#475569` (text-secondary) | `#FFFFFF` (白色) | 5.8:1 | ✅ AA |
| `#94A3B8` (text-muted) | `#FFFFFF` (白色) | 3.2:1 | ⚠️ AA (仅限辅助) |
| `#0D9488` (brand) | `#FFFFFF` (白色) | 3.8:1 | ⚠️ 不适合小字 |
| `#0D9488` (brand) | `#CCFBF1` (brand-soft) | 2.1:1 | ❌ 仅用于装饰 |
| `#FFFFFF` (白色) | `#0D9488` (brand) | 3.8:1 | ⚠️ 按钮文字需加粗 |

---

## 2. 焦点指示

### 键盘导航

```css
/* 全局 focus 样式 */
*:focus-visible {
  outline: 2px solid var(--brand);
  outline-offset: 2px;
  border-radius: var(--radius-sm);
}

/* 移除鼠标 focus（仅保留键盘 focus） */
*:focus:not(:focus-visible) {
  outline: none;
}
```

### 焦点顺序

- Tab 键导航顺序必须与视觉顺序一致
- 表单使用 `tabIndex` 控制顺序
- Modal 打开时焦点锁定在 Modal 内
- 侧栏抽屉打开时焦点锁定在抽屉内

---

## 3. ARIA 属性

### 必须添加的场景

| 场景 | ARIA 属性 | 示例 |
|------|----------|------|
| 图标按钮 | `aria-label` | `aria-label="关闭"` |
| 导航区域 | `aria-label` | `aria-label="管理导航"` |
| 消息列表 | `aria-label` | `aria-label="消息列表"` |
| 错误提示 | `role="alert"` | `role="alert"` |
| 动态内容 | `aria-live` | `aria-live="polite"` |
| 加载状态 | `aria-busy` | `aria-busy="true"` |
| 当前页面 | `aria-current` | `aria-current="page"` |
| 装饰性图标 | `aria-hidden` | `aria-hidden="true"` |

### 当前项目检查

| 位置 | 当前状态 | 需要改进 |
|------|---------|---------|
| 消息列表 | 已有 `aria-label` | ✅ 保留 |
| 管理导航 | 已有 `aria-label` | ✅ 保留 |
| 装饰性图标 | 部分已加 `aria-hidden` | 需要检查所有图标 |
| 连接状态 | 颜色仅指示 | 需要增加文字说明 |
| 错误提示 | 无 `role="alert"` | 需要增加 |
| 动态消息更新 | 无 `aria-live` | 需要增加 |

---

## 4. 语义化 HTML

### 使用规范

| 元素 | 语义 | 使用场景 |
|------|------|---------|
| `<main>` | 主内容 | 每个页面唯一 |
| `<nav>` | 导航 | 顶栏导航、侧栏 |
| `<section>` | 区块 | 侧栏区块、内容区块 |
| `<article>` | 独立内容 | 消息卡片 |
| `<header>` | 头部 | 顶栏、区块头部 |
| `<footer>` | 底部 | 页面底部 |
| `<ul>` / `<ol>` | 列表 | Agent 列表、消息列表 |
| `<li>` | 列表项 | 每个列表项 |
| `<time>` | 时间 | 消息时间戳 |

---

## 5. 状态指示器

### 颜色不是唯一指示方式

当前问题：`status-dot` 仅通过颜色区分连接状态。

改进方案：

```html
<!-- 错误：仅颜色指示 -->
<span class="status-dot status-dot--connected"></span>

<!-- 正确：颜色 + 文字 -->
<span class="status-dot status-dot--connected" aria-hidden="true"></span>
<span class="sr-only">已连接</span>
```

### 需要增加文字说明的状态

| 状态 | 当前 | 改进 |
|------|------|------|
| 连接状态 | 仅颜色 dot | 增加 "已连接/连接中/断开" 文字 |
| 房间状态 | 仅 Badge 颜色 | Badge 文字已包含状态名 ✅ |
| Agent 思考 | 动画 + 文字 | 已有 "正在思考..." ✅ |

---

## 6. 减少动画

### prefers-reduced-motion

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

### 需要此处理的动画

- 消息列表新消息进入动画
- 思考指示器动画
- 按钮 hover 过渡
- 面板展开/折叠

---

## 7. 表单无障碍

### 要求

- 所有表单输入必须有 `<label>`
- 错误提示关联输入框 (`aria-describedby`)
- 必填字段标记 (`required` 属性或 `aria-required="true"`)
- 表单提交反馈

### 当前检查

| 表单 | 状态 |
|------|------|
| 创建房间表单 | 使用 Mantine 组件，自带 label ✅ |
| 加入房间表单 | 使用 Mantine 组件 ✅ |
| Agent 编辑表单 | 使用 Mantine 组件 ✅ |
| 模型配置表单 | 使用 Mantine 组件 ✅ |
