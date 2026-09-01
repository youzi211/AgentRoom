# Typography — AgentRoom Design System

> 本文件定义 AgentRoom 的字体层级规范。基于 MASTER.md 的层级定义，提供详细用法说明。

---

## 1. Font Family

```css
font-family: "PingFang SC", "Microsoft YaHei", "Hiragino Sans GB",
  "Noto Sans SC", Inter, system-ui, -apple-system, BlinkMacSystemFont,
  "Segoe UI", sans-serif;
```

### 说明

- 中文字体优先（PingFang SC → Microsoft YaHei → Hiragino Sans GB → Noto Sans SC）
- 英文字体回退（Inter → system-ui → sans-serif）
- 保留当前字体栈，不做更换
- 不需要引入额外字体（无需 Google Fonts）

---

## 2. 层级定义

### Display（极少使用）

| 属性 | 值 |
|------|----|
| font-size | 1.75rem (28px) |
| font-weight | 800 (ExtraBold) |
| line-height | 1.25 |
| 使用场景 | 首页大标题、品牌展示 |

### H1

| 属性 | 值 |
|------|----|
| font-size | 1.5rem (24px) |
| font-weight | 800 (ExtraBold) |
| line-height | 1.3 |
| 使用场景 | 页面主标题 |
| 对应组件 | JoinScreen 标题、AdminConsole 页面标题 |

### H2

| 属性 | 值 |
|------|----|
| font-size | 1.25rem (20px) |
| font-weight | 700 (Bold) |
| line-height | 1.35 |
| 使用场景 | 区块标题 |
| 对应组件 | 侧栏标题、Section 标题 |

### H3

| 属性 | 值 |
|------|----|
| font-size | 1.1rem (17.6px) |
| font-weight | 600 (Semibold) |
| line-height | 1.4 |
| 使用场景 | 卡片标题、列表项标题 |
| 对应组件 | Agent 名称、消息标题 |

### Body（默认正文）

| 属性 | 值 |
|------|----|
| font-size | 0.9rem (14.4px) |
| font-weight | 400 |
| line-height | 1.55 |
| 使用场景 | 正文、消息内容、描述文字 |

### Body Secondary

| 属性 | 值 |
|------|----|
| font-size | 0.85rem (13.6px) |
| font-weight | 400 |
| line-height | 1.5 |
| 使用场景 | 副文本、Agent 角色描述、辅助说明 |

### Caption

| 属性 | 值 |
|------|----|
| font-size | 0.8rem (12.8px) |
| font-weight | 400 |
| line-height | 1.4 |
| 使用场景 | 时间戳、辅助信息、页脚文字 |

### Label

| 属性 | 值 |
|------|----|
| font-size | 0.85rem (13.6px) |
| font-weight | 600 (Semibold) |
| line-height | 1.4 |
| 使用场景 | 表单标签、Badge 文字、按钮文字 |

### Eyebrow

| 属性 | 值 |
|------|----|
| font-size | 0.75rem (12px) |
| font-weight | 600 (Semibold) |
| line-height | 1.3 |
| text-transform | uppercase |
| letter-spacing | 0.5px |
| 使用场景 | 区块前置标签、分类标签 |

### Code

| 属性 | 值 |
|------|----|
| font-size | 0.85rem (13.6px) |
| font-weight | 400 |
| font-family | `"SF Mono", "Fira Code", "Cascadia Code", monospace` |
| 使用场景 | 代码片段、mention 文本 |

---

## 3. Mantine 映射

```js
const theme = createTheme({
  fontFamily: appFontFamily,
  fontSizes: {
    xs: "0.75rem",   // 12px — Eyebrow, Caption
    sm: "0.85rem",   // 13.6px — Label, Body Secondary, Code
    md: "0.9rem",    // 14.4px — Body default
    lg: "1.1rem",    // 17.6px — H3
    xl: "1.25rem",   // 20px — H2
  },
  headings: {
    fontFamily: appFontFamily,
    sizes: {
      h1: { fontSize: "1.5rem", fontWeight: "800", lineHeight: "1.3" },
      h2: { fontSize: "1.25rem", fontWeight: "700", lineHeight: "1.35" },
      h3: { fontSize: "1.1rem", fontWeight: "600", lineHeight: "1.4" },
    },
  },
})
```

---

## 4. 使用规则

### 4.1 层级选择

- 一个页面有且只有一个 H1
- 段落标题使用 H2，不超过 3 级嵌套
- 卡片标题使用 H3 或 Body + 600 weight
- 不要在正文中使用 Caption 或 Eyebrow 替代 Body

### 4.2 颜色搭配

| 层级 | 颜色 |
|------|------|
| Display, H1, H2, H3 | `--text` / `#0F172A` |
| Body | `--text` / `#0F172A` |
| Body Secondary | `--text-secondary` / `#475569` |
| Caption, Eyebrow | `--text-muted` / `#94A3B8` |
| Label | `--text` / `#0F172A` |

### 4.3 中文排版注意事项

- 中文不需要 italic 样式，避免使用斜体
- 中文标题使用 ExtraBold (800) 比 Bold (700) 视觉效果更好
- 中文正文 line-height 推荐 1.5-1.6
- 中文中不要使用 `text-transform: uppercase`（Eyebrow 仅用于英文标签）
- 避免在中文段落中使用 `letter-spacing` 增加字间距

---

## 5. 当前项目问题

| 问题 | 位置 | 建议 |
|------|------|------|
| `.chat-topbar-title` 使用 `font-size: 1rem` 但需要更突出 | ChatRoom 顶栏 | 改为 H3 或 1.1rem + 700 |
| 消息内容使用默认 Body 但缺少明确层级定义 | MessageList | 统一使用 Body 层级 |
| 侧栏标题使用 `h2` 但样式不统一 | AgentRoster, 侧栏 | 统一使用 H3 层级 |
| 部分 Caption 文字颜色过浅 | 各处 | 确保使用 `--text-muted` |
