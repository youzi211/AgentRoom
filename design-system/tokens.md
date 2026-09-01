# Design Tokens — AgentRoom

> 本文件汇总 AgentRoom 前端的所有 Design Token，作为 MASTER.md 的详细参考。
> 所有 Token 必须与 Mantine Theme 和 CSS Variables 保持一致。

---

## 1. Color Tokens

### Brand

| Token | Mantine | CSS Variable | Value |
|-------|---------|-------------|-------|
| brand | teal.6 | `--brand` | `#0D9488` |
| brand-hover | teal.7 | `--brand-hover` | `#0F766E` |
| brand-soft | teal.1 | `--brand-soft` | `#CCFBF1` |
| brand-light | teal.0 | — | `#F0FDFA` |
| secondary | teal.5 | — | `#14B8A6` |

### Neutral

| Token | CSS Variable | Value |
|-------|-------------|-------|
| bg | `--bg` | `#F8FAFC` |
| surface | `--surface` | `#FFFFFF` |
| surface-muted | `--surface-muted` | `#F1F5F9` |
| surface-hover | `--surface-hover` | `#F8FAFC` |
| text | `--text` | `#0F172A` |
| text-secondary | `--text-secondary` | `#475569` |
| text-muted | `--text-muted` | `#94A3B8` |
| border | `--border` | `#E2E8F0` |
| border-strong | `--border-strong` | `#CBD5E1` |

### Semantic

| Token | CSS Variable | Value |
|-------|-------------|-------|
| success | `--success` | `#22C55E` |
| success-soft | `--success-soft` | `#F0FDF4` |
| warning | `--warning` | `#F59E0B` |
| warning-soft | `--warning-soft` | `#FFFBEB` |
| danger | `--danger` | `#EF4444` |
| danger-soft | `--danger-soft` | `#FEF2F2` |
| info | `--info` | `#3B82F6` |
| info-soft | `--info-soft` | `#EFF6FF` |

---

## 2. Spacing Tokens

| Token | Mantine | Value | Usage |
|-------|---------|-------|-------|
| space-xs | xs | 4px | 紧凑模式 |
| space-sm | sm | 8px | 密集排列 |
| space-md | md | 16px | 组件间距 |
| space-lg | lg | 24px | 页面/区块间距 |
| space-xl | xl | 32px | 大区块间距 |
| space-2xl | — | 40px | 页面顶部/底部 |
| space-3xl | — | 48px | 超大间距 |

---

## 3. Radius Tokens

| Token | Mantine | Value | Usage |
|-------|---------|-------|-------|
| radius-none | — | 0 | 容器、Panel |
| radius-sm | sm | 4px | 按钮、输入框 |
| radius-md | md | 8px | 卡片、Paper |
| radius-lg | lg | 12px | 大卡片 |
| radius-pill | pill | 999px | 标签、圆形头像 |

---

## 4. Shadow Tokens

| Token | Value | Usage |
|-------|-------|-------|
| shadow-sm | `0 1px 3px rgba(0,0,0,0.06)` | 卡片默认 |
| shadow-md | `0 4px 12px rgba(0,0,0,0.08)` | 浮动元素 |
| shadow-lg | `0 8px 24px rgba(0,0,0,0.10)` | Modal/Drawer |

---

## 5. Typography Tokens

| Token | Mantine | Value | Weight |
|-------|---------|-------|--------|
| font-xs | xs | 0.75rem | 400 |
| font-sm | sm | 0.85rem | 400/600 |
| font-md | md | 0.9rem | 400 |
| font-lg | lg | 1.1rem | 600 |
| font-xl | xl | 1.25rem | 700 |
| font-display | — | 1.5rem | 800 |
| font-code | — | 0.85rem | 400 |

---

## 6. Transition Tokens

| Token | Value |
|-------|-------|
| transition-fast | 150ms ease |
| transition-normal | 200ms ease |

---

## 7. Current CSS Variable 映射

以下 CSS 变量需要根据新 Token 更新：

| 当前变量 | 当前值 | 新值 | 状态 |
|---------|-------|------|------|
| `--bg` | `#f6f7f4` | `#F8FAFC` | 待更新 |
| `--surface` | `#ffffff` | `#FFFFFF` | 保留 |
| `--text` | `#1f2933` | `#0F172A` | 待更新 |
| `--muted` | `#657268` | `#94A3B8` | 待更新 |
| `--accent` | `#2f6657` | `#0D9488` | 待更新 |
| `--accent-soft` | `#e7f1ec` | `#CCFBF1` | 待更新 |
| `--accent-strong` | `#214c42` | `#0F766E` | 待更新 |
| `--border` | `#dde5dc` | `#E2E8F0` | 待更新 |
| `--border-strong` | `#bdcbbb` | `#CBD5E1` | 待更新 |
| `--shadow-sm` | `0 10px 28px rgba(...)` | `0 1px 3px rgba(...)` | 待更新 |
| `--shadow-md` | `0 18px 44px rgba(...)` | `0 4px 12px rgba(...)` | 待更新 |
| `--shadow-lg` | `0 20px 50px rgba(...)` | `0 8px 24px rgba(...)` | 待更新 |
| `--radius-lg` | 10px | 12px | 待更新 |
| `--radius-md` | 8px | 8px | 保留 |
| `--radius-sm` | 6px | 4px | 待更新 |
