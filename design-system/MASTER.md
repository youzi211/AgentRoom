# AgentRoom Design System — MASTER

> 本文件是 AgentRoom 前端 Design System 的最高级规范。所有 UI 决策必须遵守本文件定义的原则。
> 后续 Codex 重构时应以本文件为唯一设计依据，结合 pages/ 下的页面规范进行修改。

---

## 1. Product UI Principles

### 1.1 核心原则

| # | 原则 | 说明 |
|---|------|------|
| 1 | **内容优先** | UI 的存在是为了展示内容，而不是装饰内容。去掉所有不服务于信息传达的装饰。 |
| 2 | **克制** | 每个视觉元素必须有存在的理由。不要因为"可以加"就加。 |
| 3 | **一致性** | 相同功能使用相同视觉模式。不创造新的变体。 |
| 4 | **可预测性** | 用户能预判交互结果。相同操作触发相同反馈。 |
| 5 | **包容性** | 无障碍不是附加功能，是基本要求。 |

### 1.2 视觉方向

**Minimal & Direct + Technical**

| 关键词 | 说明 |
|--------|------|
| clean | 干净，多余装饰为零 |
| professional | 专业，值得信赖 |
| information-dense but breathable | 信息密度高但不拥挤 |
| restrained | 克制，不滥用视觉强调 |
| technical | 技术感，适合开发者/AI 产品 |
| trustworthy | 可信赖，适合会议协作场景 |

### 1.3 避免

- 过度 Card（每个区域都套 Paper）
- 过度圆角（全部 sm 起步，不滥用 lg）
- 大面积渐变（背景最多微渐变）
- 过重阴影（shadow 层级不超过 3 层）
- 无意义装饰（装饰性图标、不必要的渐变）
- Dashboard template 感（避免标准卡片式管理后台模板）
- AI-generated UI 的廉价感（均匀分布、无重点、无层次）

---

## 2. Color System

### 2.1 核心色板

```css
/* Brand */
--brand: #0D9488;          /* teal-600 */
--brand-hover: #0F766E;    /* teal-700 */
--brand-soft: #CCFBF1;     /* teal-100 */

/* Background */
--bg: #F8FAFC;             /* slate-50 */
--bg-gradient: linear-gradient(180deg, #FFFFFF 0%, #F8FAFC 100%);

/* Surface */
--surface: #FFFFFF;
--surface-muted: #F1F5F9;  /* slate-100 */
--surface-hover: #F8FAFC;  /* slate-50 */

/* Text */
--text: #0F172A;           /* slate-900 */
--text-secondary: #475569; /* slate-600 */
--text-muted: #94A3B8;     /* slate-400 */
--text-inverse: #FFFFFF;

/* Border */
--border: #E2E8F0;         /* slate-200 */
--border-strong: #CBD5E1;  /* slate-300 */

/* Semantic */
--success: #22C55E;        /* green-500 */
--success-soft: #F0FDF4;   /* green-50 */
--warning: #F59E0B;        /* amber-500 */
--warning-soft: #FFFBEB;   /* amber-50 */
--danger: #EF4444;         /* red-500 */
--danger-soft: #FEF2F2;    /* red-50 */
--info: #3B82F6;           /* blue-500 */
--info-soft: #EFF6FF;      /* blue-50 */
```

### 2.2 Mantine Theme 映射

```js
const theme = createTheme({
  primaryColor: "teal",
  primaryShade: 6,
  colors: {
    teal: [
      "#F0FDFA", // 0 — bg brand soft
      "#CCFBF1", // 1 — brand-soft
      "#99F6E4", // 2
      "#5EEAD4", // 3
      "#2DD4BF", // 4
      "#14B8A6", // 5 — secondary
      "#0D9488", // 6 — PRIMARY (brand)
      "#0F766E", // 7 — brand-hover
      "#115E59", // 8
      "#134E4A", // 9
    ],
  },
})
```

### 2.3 色值对照

| 角色 | 当前项目 | 新规范 | 说明 |
|------|---------|--------|------|
| Brand | `#2f6657` (CSS `--accent`) | `#0D9488` | 统一到 Mantine teal-600 |
| Background | `#f6f7f4` (CSS `--bg`) | `#F8FAFC` | 向 slate-50 靠拢 |
| Text | `#1f2933` (CSS `--text`) | `#0F172A` | 提高对比度 |
| Muted | `#657268` (CSS `--muted`) | `#64748B` | 使用标准 slate-500 |
| Success | `#2f7d4f` (CSS `--success`) | `#22C55E` | 使用标准 green-500 |
| Danger | `#b42318` (CSS `--danger`) | `#EF4444` | 使用标准 red-500 |
| Warning | `#b7791f` (CSS `--warning`) | `#F59E0B` | 使用标准 amber-500 |

> **迁移策略**：CSS 变量 `--accent` 保留为 `#0D9488` 别名，旧色值逐步替换。不一次性删除旧变量。

---

## 3. Typography

### 3.1 Font Family

保留当前字体栈，不做更换：

```css
font-family: "PingFang SC", "Microsoft YaHei", "Hiragino Sans GB",
  "Noto Sans SC", Inter, system-ui, -apple-system, BlinkMacSystemFont,
  "Segoe UI", sans-serif;
```

### 3.2 层级定义

| 层级 | size | weight | line-height | 使用场景 |
|------|------|--------|-------------|---------|
| H1 | 1.5rem (24px) | 800 (ExtraBold) | 1.3 | 页面主标题 |
| H2 | 1.25rem (20px) | 700 (Bold) | 1.35 | 区块标题 |
| H3 | 1.1rem (17.6px) | 600 (Semibold) | 1.4 | 卡片标题、侧栏标题 |
| Body | 0.9rem (14.4px) | 400 | 1.55 | 正文 |
| Body Secondary | 0.85rem (13.6px) | 400 | 1.5 | 副文本、描述 |
| Caption | 0.8rem (12.8px) | 400 | 1.4 | 辅助信息、时间戳 |
| Label | 0.85rem (13.6px) | 600 | 1.4 | 表单标签、Badge 文字 |
| Eyebrow | 0.75rem (12px) | 600 | 1.3 | 区块前置标签，uppercase |

> 中文 UI 中 line-height 应比英文稍大（1.5-1.6），以容纳中文字符的阅读舒适度。

### 3.3 Mantine 映射

```js
const theme = createTheme({
  fontFamily: appFontFamily,
  fontSizes: {
    xs: "0.75rem",   // 12px — Eyebrow, Caption
    sm: "0.85rem",   // 13.6px — Label, Body Secondary
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

## 4. Spacing

### 4.1 基础网格

采用 **4px 网格**，只使用以下值：

```
4, 8, 12, 16, 20, 24, 32, 40, 48
```

禁止随意使用 9px, 10px, 14px, 18px, 22px 等值。

### 4.2 使用规范

| 场景 | 值 | 说明 |
|------|----|------|
| 页面内边距 | 24px | `.workbench` 容器 |
| Section 间距 | 24px | 同一页面内区块间 |
| Panel 内边距 | 20px | 面板内部 padding |
| 组件间距 | 16px | 同一区域内组件间 |
| 密集排列 | 8px | 标签组、按钮组 |
| 紧凑模式 | 4px | 极小间距（Badge 组） |
| 元素内边距 | 12px | 列表项、卡片内部 |

### 4.3 Mantine 映射

```js
const theme = createTheme({
  spacing: {
    xs: "4px",
    sm: "8px",
    md: "16px",
    lg: "24px",
    xl: "32px",
  },
})
```

---

## 5. Radius

| 层级 | 值 | 使用场景 |
|------|----|---------|
| none | 0 | 容器、Panel |
| sm | 4px | 按钮、输入框、Badge |
| md | 8px | 卡片、Paper、Modal |
| lg | 12px | 大卡片、大图 |
| pill | 999px | 标签、头像圆形 |

### 5.1 原则

- 按钮默认使用 `sm`（4px），不推荐使用 `pill` 按钮
- 容器（Panel、Section）使用 `none` 或 `sm`
- 独立信息卡片使用 `md`（8px）
- 不在非必要场景使用 `lg` 圆角

```js
const theme = createTheme({
  defaultRadius: "sm", // 4px
  radius: {
    xs: "2px",
    sm: "4px",
    md: "8px",
    lg: "12px",
    xl: "16px",
  },
})
```

---

## 6. Shadow / Elevation

保持 Minimal 风格，只使用 3 层阴影：

| 层级 | 值 | 使用场景 |
|------|----|---------|
| sm | `0 1px 3px rgba(0,0,0,0.06)` | 微隆起、卡片默认 |
| md | `0 4px 12px rgba(0,0,0,0.08)` | 下拉菜单、Popover |
| lg | `0 8px 24px rgba(0,0,0,0.10)` | Modal、Drawer |

**禁止使用：** `0 20px 50px` 等大范围重阴影。

### Elevation 规则

| 层 | 元素 | shadow |
|----|------|--------|
| 0 | 背景、容器 | none |
| 1 | 普通卡片、列表项 | sm |
| 2 | 浮动顶栏、下拉菜单 | md |
| 3 | Modal、Drawer、Toast | lg |

---

## 7. Layout

### 7.1 App Shell

```
+--------------------------------------------------+
|  Header / TopBar (可选)                          |
+--------------------------------------------------+
|  Sidebar  |  Main Content  |  Sidebar (可选)    |
|  (270px)  |  flex: 1       |  (320px)           |
+--------------------------------------------------+
```

### 7.2 容器宽度

| 页面 | 最大宽度 | 说明 |
|------|---------|------|
| JoinScreen | 1240px | `.workbench` 容器 |
| AdminConsole | 1240px | `.workbench--admin` |
| ChatRoom | 1450px | `.chat-workbench` 三栏布局 |
| ReadOnly | 1240px | 只读页面 |

### 7.3 页面布局模式

- **Entry 模式**：顶部导航 + 居中内容（JoinScreen, AdminConsole）
- **Chat 模式**：全屏三栏布局（ChatRoom）
- **Center 模式**：居中单卡片（RoomGateway loading/error）

---

## 8. Iconography

### 8.1 图标集

使用 **Lucide React**，保持统一。

### 8.2 使用规则

- 所有图标使用 `size={16}` 或 `size={18}`（内联图标）
- 按钮图标使用 `size={16}`，`leftSection` 图标
- 装饰性图标（非交互）加 `aria-hidden="true"`
- 不使用 emoji 作为 UI 图标
- 不使用 Lucide 以外的图标库

---

## 9. Component Principles

### 9.1 通用规则

- 所有可交互元素必须设置 `cursor: pointer`
- 所有 hover 状态必须有视觉反馈（颜色/阴影/边框变化）
- 过渡动画统一使用 `150-200ms ease`
- 禁用状态必须有视觉指示

### 9.2 Card/Paper 使用规则

**核心原则：Layout / Panel ≠ Card**

| 应该使用 Card | 不应该使用 Card |
|-------------|----------------|
| 独立信息实体（消息、Agent 条目） | 容器/面板（侧栏、顶栏） |
| 可点击的卡片式选择 | 区块背景（section 背景） |
| Modal/Drawer 内容 | 页面布局容器 |
| 列表项（需要视觉分割时） | 列表容器本身 |

**具体规则：**

1. 页面容器（workbench、sidebar-section）使用普通 `<div>`，不套 Paper
2. 顶栏（topbar）使用普通 `<header>`，不套 Paper
3. 侧栏区块使用 `section/sidebar-section` 类，不加 `withBorder`
4. 消息列表中的消息使用 Paper，但去掉 `withBorder`，用背景色区分
5. Agent 列表中的每个 Agent 条目**不使用**独立 Paper，使用普通列表项 + 顶部边框或 hover 背景

### 9.3 Message 显示规则

- 自己的消息：右对齐，`--accent-soft` 背景，无边框
- 他人消息：左对齐，`--surface-muted` 背景，无边框
- Agent 消息：左对齐，`--surface` 背景，左侧品牌色边条
- 系统消息：居中显示，`--text-muted` 颜色，小字号

### 9.4 Agent Item 显示规则

- 使用普通列表项（`<li>`），不套 Paper
- 左侧 Avatar + 名称/角色
- 右侧 `@mention` 按钮
- hover 时背景色变为 `--surface-hover`
- 选中时背景色变为 `--brand-soft`

---

## 10. Interaction Principles

### 10.1 过渡动画

```css
--transition-fast: 150ms ease;
--transition-normal: 200ms ease;
```

### 10.2 状态反馈

| 状态 | 反馈 |
|------|------|
| hover | 颜色/背景色变化，`150ms ease` |
| active (press) | 轻微缩小或颜色加深 |
| focus | 蓝色 outline 或品牌色 ring |
| disabled | 降低不透明度 (0.5) + 禁止光标 |
| loading | 禁用交互 + spinner 或 skeleton |

---

## 11. Responsive Strategy

### 11.1 断点

| 断点 | 宽度 | 说明 |
|------|------|------|
| mobile | < 768px | 单栏布局 |
| tablet | 768-1024px | 两栏布局 |
| desktop | 1024-1440px | 三栏布局 |
| wide | > 1440px | 保持 max-width |

### 11.2 ChatRoom 响应式策略

- mobile: 只显示中央消息区，顶栏简化，侧栏通过抽屉访问
- tablet: 显示中央消息区 + 右侧栏（折叠左侧栏）
- desktop: 完整三栏

---

## 12. Do / Don't

### Do

- 使用 4px 网格
- 使用 Mantine theme 定义的值
- 保持一致的间距和颜色
- 优先使用背景色而非边框分割内容
- 使用文字权重建立层级
- 为所有交互元素添加 hover 反馈
- 为所有图标按钮添加 aria-label

### Don't

- 不使用随意间距值（9px, 10px, 14px, 18px, 22px）
- 不滥用 Paper/Card 包裹容器
- 不使用大面积阴影
- 不使用 emoji 作为 UI 图标
- 不创建新的颜色变体
- 不覆盖 Mantine 组件默认样式（除非必要）
- 不在 CSS 中重复 Mantine 已提供的样式
