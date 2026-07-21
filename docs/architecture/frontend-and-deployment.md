# 前端与部署架构

## 四服务部署更新

Compose 的当前拓扑是 `frontend -> backend -> agent-runtime`，backend 同时连接 MySQL。`agent-runtime` 只通过 Compose 内部网络暴露 50051，不发布宿主机端口；标准 gRPC Health 控制 backend 启动依赖。`/api/health` 只表示 Go 存活，`/api/ready` 分别报告 MySQL 和 Agent Runtime。开发 Compose 显式使用 plaintext，非本机生产部署必须配置 TLS、服务名校验和可选客户端证书。

[返回架构索引](README.md)

本文描述 React SPA、浏览器协议边界、Docker 拓扑和部署安全约束。完整启动命令和环境变量表仍以根 [`README.md`](../../README.md) 与 [`.env.example`](../../.env.example) 为准。

## 1. 前端技术栈

```text
React 18
+ Vite 5
+ Mantine 8
+ lucide-react
+ custom History API routing
+ native fetch / WebSocket
```

前端没有 React Router，也没有集中式状态管理库。顶层路由状态由 `App.jsx` 和 `routing.js` 管理；房间实时状态主要位于 `ChatRoom` 组件树。

入口文件：

- `frontend/src/main.jsx`
- `frontend/src/App.jsx`
- `frontend/src/routing.js`
- `frontend/src/api/roomClient.js`

## 2. 路由和页面层次

`routing.js` 封装 `window.history.pushState`、`replaceState`、`popstate` 和应用自定义导航事件。

```text
App
├── /                         JoinScreen
│   ├── create room
│   ├── join room
│   ├── select Agents / role sets
│   └── recent rooms / entry summary
│
├── /rooms/:roomID           RoomGateway
│   ├── loading / error
│   ├── RoomEntry            name required; passcode required only when configured
│   ├── ChatRoom             active live session
│   ├── RoomReadOnly         closed room
│   └── archived denied
│
├── /admin                   AdminGate -> AdminConsole(meetings)
├── /agents                  AdminGate -> AdminConsole(agents)
├── /models                  AdminGate -> AdminConsole(models)
├── /admin/agents            compatible route
├── /admin/models            compatible route
├── /admin/<other...>        current fallback to meetings
└── other                    NotFound
```

规范管理入口是 `/admin`、`/agents` 和 `/models`。后两者仍进入统一 `AdminConsole`，不是独立应用。

关键组件：

```text
components/
├── JoinScreen.jsx
├── RoomGateway.jsx
├── RoomEntry.jsx
├── ChatRoom.jsx
├── MeetingRoomExperience.jsx
├── RoomReadOnly.jsx
├── AdminGate.jsx
├── AdminConsole.jsx
├── MeetingAdmin.jsx
├── AgentAdmin.jsx
└── ModelProfileAdmin.jsx
```

## 3. 房间页面状态

`RoomGateway` 根据 URL、sessionStorage 和后端房间状态决定：

- 是否需要输入参与者名称或房间口令；
- 是否建立实时 WebSocket 会话；
- 是否显示 closed 房间只读页面；
- 是否拒绝 archived 或不存在房间。

当前 live 页面由 `ChatRoom` 建立 Socket 和维护数据，再渲染 `MeetingRoomExperience`。主要区域包括：

```text
+--------------------------- top bar ---------------------------+
| room identity | connection state | room ID | leave           |
+----------------+-----------------------------+----------------+
| room info      | real-time discussion        | Agent roster   |
| knowledge      | message filters and list    | Focus timeline |
| minutes        | composer                    | Agent activity |
+----------------+-----------------------------+----------------+
```

开发时需要注意：`ChatRoom.jsx` 在当前实际 `return <MeetingRoomExperience ... />` 之后仍保留一套不可达旧布局。旧布局中的部分房主控制不等于当前 UI 一定可见；真实渲染应从实际 return 路径判断。

## 4. API Client 边界

`frontend/src/api/roomClient.js` 是统一浏览器协议适配层：

- 所有 REST 使用相对路径 `/api`；
- 使用原生 `fetch`；
- 解析后端 JSON error；
- 管理 `X-Admin-Key`；
- 管理 `X-Room-Passcode`；
- 构建 same-origin WebSocket URL；
- 提供 artifact/minutes 下载；
- 封装房间、Agent、知识、会议管理和模型 Profile API。

使用相对路径意味着：

- 本地开发由 Vite 把 `/api` 代理到 `127.0.0.1:8080`；
- Docker 部署由 nginx 把 `/api` 代理到 `backend:8080`；
- 浏览器不直接知道 Docker backend service name。

### 4.1 房间凭据

普通房间请求可携带：

```text
X-Room-Passcode: <room secret>
```

WebSocket 当前把 passcode 放在 query 参数。路由和 sessionStorage 用于刷新后恢复房间入口状态，但这不是正式用户 session。

### 4.2 管理凭据

管理员请求使用：

```text
X-Admin-Key: <shared admin secret>
```

浏览器输入的 key 保存到 localStorage。该机制是共享 bearer secret，不具有用户身份、角色、过期和审计语义。

## 5. WebSocket 状态合并

浏览器 Socket 地址从当前页面 scheme 和 host 派生：

```text
http  -> ws
https -> wss

/api/rooms/<roomID>/ws?name=<name>&passcode=<optional>
```

`ChatRoom` 处理：

- 初始 `room_snapshot`；
- 新 message；
- participant join/leave；
- Focus update；
- Agent activity；
- room close/archive；
- error 和连接状态。

浏览器内状态不是持久化事实。刷新或漏掉事件后，应通过 REST 房间/消息历史和新的 snapshot 重建，而不是假设 WebSocket 可靠重放。

当前客户端/服务端没有完整 heartbeat、断线指数退避和 resume cursor 协议。如果增加自动重连，需要同时设计去重、历史补偿和 participant session 语义。

## 6. 管理控制台

`AdminGate` 在渲染 `AdminConsole` 前调用 `/api/admin/verify`。真正授权必须由后端 `requireAdmin` 执行，前端 Gate 只是 UX。

`AdminConsole` 包含三个 section：

| Section | 组件 | 能力 |
| --- | --- | --- |
| Meetings | `MeetingAdmin` | 查看房间、历史、纪要、archive/reopen/restore |
| Agents | `AgentAdmin` | Agent CRUD、Runtime、Profile 绑定、知识 |
| Models | `ModelProfileAdmin` | Go/DeepAgent Profile CRUD、默认、密钥和连接测试 |

模型 API Key 只在创建/替换或草稿测试时从表单提交；列表/详情响应只包含是否配置及 hint。

## 7. 前端测试和构建

前端测试使用 Node 内置 test runner，文件通常与模块相邻：

```powershell
node --test frontend/src/api/roomClient.test.mjs
node --test frontend/src/components/<feature>.test.mjs
npm --prefix frontend run build
```

没有统一 `npm test` script。修改路由、Client payload 或组件 helper 时，应运行对应 `*.test.mjs` 以及生产构建。

## 8. Docker Compose 拓扑

```text
Host
├── ${FRONTEND_HOST_PORT:-5173} -> frontend:80
├── ${BACKEND_HOST_PORT:-8080}  -> backend:8080
└── ${MYSQL_HOST_PORT:-3306}     -> mysql:3306

Docker network
frontend nginx --/api, WS--> backend --GORM--> mysql
                                  |
                                  `--exec--> Python DeepAgent child
```

服务职责：

### 8.1 `mysql`

- MySQL 8.4；
- 初始化 application database/user；
- `mysql-data` volume；
- utf8mb4；
- healthcheck。

### 8.2 `backend`

- 运行 Go API/WebSocket；
- 通过容器网络 DSN 访问 MySQL；
- 自动迁移默认启用；
- 内置 Python 3.12、固定 `uv`、DeepAgent locked dependencies；
- `DEEPAGENT_WORKDIR=/app/deepagent`；
- `deepagent-runs` volume；
- backend healthcheck。

### 8.3 `frontend`

- multi-stage 构建 Vite bundle；
- nginx 托管静态文件；
- `/api` 和 WebSocket 反向代理；
- SPA fallback 到 `index.html`；
- 依赖 backend healthcheck。

关键文件：

- `docker-compose.yml`
- `backend/Dockerfile`
- `frontend/Dockerfile`
- `frontend/nginx.conf`

## 9. DeepAgent 打包和运行

Backend 镜像大致分为：

```text
Go builder stage
  -> build /out/agentroom

Python runtime stage
  -> install CA certificates and wget
  -> copy pinned uv
  -> uv sync from deepagent/uv.lock
  -> copy DeepAgent source/config/registry
  -> install non-editable Python project
  -> copy Go binary
  -> run as non-root UID
```

DeepAgent 不监听端口。Go 使用 `exec.CommandContext` 启动 `uv run deepagent-research`，输出到 `/app/deepagent/runs`。

联网研究除了可解析的 `deepagent` Model Profile 外，还需要独立的 `TAVILY_API_KEY`。

## 10. 配置所有权

完整变量、默认值和启动方式由以下文件维护：

- `.env.example`
- 根 `README.md`
- `scripts/docker-up.ps1`
- `scripts/docker-up.sh`

组件配置类别：

| 组件 | 配置类别 |
| --- | --- |
| backend | HTTP port、MySQL DSN、迁移、日志、安全、模型主密钥 |
| Go model fallback | `LLM_*`，仅数据库无 Go 默认 Profile 时迁移兜底 |
| DeepAgent fallback | `MODEL_*`，仅数据库无 DeepAgent 默认 Profile 时迁移兜底 |
| DeepAgent runtime | command、workdir、config、registry、timeout、concurrency、Tavily |
| frontend build | API proxy 开发目标、当前兼容用 build-time admin key |
| Compose | host ports、MySQL bootstrap、`ALLOWED_ORIGINS` 传递 |
| Docker 启动脚本 | `PUBLIC_ORIGIN` 输入及其到 `ALLOWED_ORIGINS` 的合并 |

启动脚本会创建 `.env`、随机化 placeholder secrets、生成 Model Profile 主密钥、对齐管理员 key、把 `PUBLIC_ORIGIN` 合并到 allowlist、验证 Compose 并等待健康检查。Backend 和 Compose 本身不读取 `PUBLIC_ORIGIN`；直接执行 `docker compose up` 时必须显式设置 `ALLOWED_ORIGINS`。生产部署仍应把秘密迁移到正式 Secret Store，而不是依赖仓库根 `.env`。

## 11. 安全与信任边界

```text
Browser: untrusted public client
   |
   | TLS required for admin key / passcode / draft provider key
   v
Go backend: privileged authority
   |-- MySQL credentials
   |-- admin key
   |-- profile encryption master key
   |-- decrypted provider credentials during calls
   |-- Tavily / legacy env fallbacks
   v
DeepAgent child: currently same high-trust domain
   |
   +--> model provider / Tavily: external data processors
```

### 11.1 Build-time admin key 不是秘密

`VITE_ADMIN_API_KEY` 会被 Vite 编译进浏览器 bundle。任何能下载静态 JS 的人都可以提取或在 DevTools 中观察它。因此：

```text
VITE_ADMIN_API_KEY != server-side authentication secret
```

当前脚本/Compose 可能把它与 `ADMIN_API_KEY` 对齐，这只适合作为本地或完全封闭可信环境的便利机制，不能用于公网生产授权。

此外，`roomClient` 会优先使用 localStorage key，否则使用 build-time key；部分普通房间读取也会附带管理员 key。后端把有效管理员 key 视为可绕过房间口令，因此 build-time key 会扩大权限面。

面向生产的正确方向是服务端身份/session，例如 HttpOnly cookie、组织 SSO、短期 token 和 RBAC，而不是把长期管理员秘密下发到浏览器。

### 11.2 AdminGate 不是授权边界

`AdminGate` 可以被客户端绕过或修改。所有管理操作必须由 backend middleware 验证。当前 backend 在 `ADMIN_API_KEY` 为空时放行管理路由；公网部署不得留空。

### 11.3 TLS

Compose 内置 nginx 只提供 HTTP。若跨主机或公网访问，必须在外部 reverse proxy/load balancer 终止 TLS，否则以下内容会明文传输：

- 管理员 key；
- 房间口令；
- 模型 Profile 表单中的 Provider key；
- 会议内容。

### 11.4 端口暴露

Compose 默认把 frontend、backend 和 MySQL 都发布到 host，并未限制为 `127.0.0.1`。生产环境必须用 firewall/security group 控制 backend 和 MySQL；不能假设所有请求都只经过 frontend nginx。

### 11.5 DeepAgent 环境继承

当前 Python 子进程基于 `os.Environ()` 启动，因此实际继承 backend 全部环境变量，而不仅是本次 `MODEL_*`。DeepAgent 及其第三方依赖必须视为 backend 同等级可信代码。详见 [Agent Runtime 与模型](agent-runtime-and-models.md)。

### 11.6 Profile Base URL

管理员可配置后端主动访问的 HTTP/HTTPS URL。连接测试和模型调用可能携带保存的 Provider key，所以该能力同时是受信任的服务端出站请求能力。生产应考虑 HTTPS-only、私网地址限制和 egress policy。

## 12. 构建上下文卫生

Docker 使用仓库根作为 build context。当前 `.dockerignore` 已排除根 `.env`、前端依赖/构建产物和部分 Go 缓存，但**尚未排除** `deepagent/.venv/`、`deepagent/runs/` 及全部测试缓存。

DeepAgent 自己的 `.gitignore` 不会自动成为 Docker ignore 规则。由于 backend Dockerfile 会复制 `deepagent/`，当前本地虚拟环境或历史报告可能进入 build context，甚至进入镜像层。这是现存的构建卫生和潜在数据泄露风险；上线或共享镜像前应补齐 ignore 规则并检查构建上下文。

## 13. 部署范围

当前部署最适合：

- 单 backend 实例；
- 可信内网/单组织；
- 外部 TLS 代理；
- backend/MySQL 端口受网络限制；
- DeepAgent 被视为高权限可信执行器。

公网、多租户或多实例之前至少需要：

- 正式用户身份、组织隔离和 RBAC；
- 模型/Tavily 调用配额与速率限制；
- Redis/事件总线和 WebSocket sticky routing；
- durable Agent task queue；
- DeepAgent 独立低权限 worker；
- secret redaction 和审计；
- artifact retention；
- Provider egress policy。

## 14. 修改检查表

- 新路由是否能被 `routing.js`、nginx SPA fallback 和刷新访问正确处理；
- API Client 是否只使用 `/api/*`；
- 房间 passcode/admin key 是否只发给必要请求；
- WebSocket 新事件是否有 snapshot/历史补偿策略；
- `VITE_*` 中是否误放秘密；
- Compose host port、Origin 和 nginx proxy 是否同步；
- DeepAgent 镜像是否仍包含 locked runtime；
- `.dockerignore` 是否覆盖新增本地产物；
- 前端相关 Node 测试和生产 build 是否通过；
- 部署行为变化是否同步根 README 和 `.env.example`。

## 15. 相关架构

- [后端架构](backend.md)
- [Agent Runtime 与模型](agent-runtime-and-models.md)
- [数据、实时与会议生命周期](data-realtime-lifecycle.md)
