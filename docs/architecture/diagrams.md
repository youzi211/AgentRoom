# AgentRoom 架构图

> 本文档使用 Mermaid 图表描述 AgentRoom 当前架构。所有图表反映 ADR-001 统一模型执行边界实施后的状态。

## 1. 系统总览

```mermaid
graph TB
    subgraph Browser["浏览器"]
        FE["React 18 + Mantine SPA<br/>REST + WebSocket"]
    end

    subgraph GoControl["Go 控制面 (backend)"]
        API["Gin HTTP + Gorilla WS<br/>api.Server"]
        SVC["service.RoomService<br/>用例协调"]
        ROOM["room.Manager<br/>内存状态 + Hub"]
        AGENT["agent.Runner<br/>单 Agent 编排"]
        COLLAB["collaboration.Coordinator<br/>多 Agent 协作治理"]
        STORE["store.Store<br/>MySQL/GORM"]
        LLM["llm.Client<br/>OpenAI-compatible"]
        MODELPROF["Model Profile Service<br/>加密凭据 + 选择策略"]
    end

    subgraph PythonExec["Python 执行面 (agent-runtime)"]
        ARS["AgentRuntimeService<br/>单 Agent gRPC"]
        CRS["CollaborationRuntimeService<br/>协作 gRPC"]
        REG["ExecutorRegistry"]
        LLMEX["LLMExecutor"]
        DAEX["DeepAgentExecutor"]
        NATIVE["NativeCollaborationEngine"]
        AUTOGEN["AutoGenCollaborationEngine"]
        MES["ModelExecutionService<br/>统一模型入口"]
        MCR["ModelConfigResolver<br/>+ CredentialResolver"]
    end

    subgraph Data["数据层"]
        MYSQL[("MySQL 8<br/>房间/消息/审计")]
        OPENAI["OpenAI-compatible API<br/>LLM Provider"]
        TAVILY["Tavily Search API"]
    end

    FE <-->|"REST /api<br/>WebSocket"| API
    API --> SVC
    SVC --> ROOM
    SVC --> AGENT
    SVC --> COLLAB
    SVC --> STORE
    AGENT --> LLM
    SVC --> MODELPROF

    SVC -->|"gRPC ExecuteAgent"| ARS
    COLLAB -->|"gRPC ExecuteConversation"| CRS

    ARS --> REG
    REG --> LLMEX
    REG --> DAEX
    CRS --> NATIVE
    CRS --> AUTOGEN

    NATIVE -->|"AgentTurn + ModelSelection"| RRAE["RuntimeRegistryAgentExecutor"]
    AUTOGEN -->|"AgentTurn + ModelSelection"| RRAE
    RRAE --> MCR
    RRAE --> REG
    AUTOGEN -->|"selector purpose"| MES
    MES --> MCR

    LLMEX --> OPENAI
    DAEX --> TAVILY
    DAEX --> OPENAI
    MES --> OPENAI

    STORE <--> MYSQL

    style GoControl fill:#e8f5e9,stroke:#2e7d32
    style PythonExec fill:#fff3e0,stroke:#e65100
    style Data fill:#e3f2fd,stroke:#1565c0
```

## 2. 统一模型执行边界（ADR-001 核心数据流）

```mermaid
graph LR
    subgraph Go["Go Control Plane"]
        GP["房间/Agent 策略<br/>Model Profile"]
        MS["ModelSelection<br/>profile_id + credential_ref<br/>不含明文凭据"]
    end

    subgraph Py["Python Execution Plane"]
        CE["Collaboration Engine<br/>选角 + 轮次策略"]
        RRAE["RuntimeRegistryAgentExecutor<br/>准备阶段边界"]
        MCR["ModelConfigResolver"]
        CR["CredentialResolver"]
        MC["ModelConfig<br/>短生命周期<br/>含 api_key"]
        EX["LLMExecutor /<br/>DeepAgentExecutor"]
        MES["ModelExecutionService<br/>AutoGen Selector 入口"]
    end

    subgraph Ext["外部"]
        ENV["环境变量<br/>LLM_API_KEY / MODEL_API_KEY"]
        SP["Secret Provider<br/>(生产 Adapter, 待实施)"]
        PROVIDER["OpenAI-compatible API"]
    end

    GP --> MS
    MS -->|"gRPC 快照<br/>不含 API Key"| CE
    CE -->|"AgentTurn +<br/>ModelSelection"| RRAE
    RRAE -->|"resolve()"| MCR
    MCR --> CR
    CR -->|"environment:go"| ENV
    CR -->|"profile:id"| SP
    CR -.->|"❌ 无 Adapter<br/>preparation 失败<br/>不回退环境"| SP
    MCR -->|"完整 ModelConfig"| MC
    MC --> EX
    EX --> PROVIDER

    CE -->|"selector purpose"| MES
    MES --> MCR
    MES -->|"ModelClient"| PROVIDER

    style Go fill:#e8f5e9,stroke:#2e7d32
    style Py fill:#fff3e0,stroke:#e65100
    style Ext fill:#e3f2fd,stroke:#1565c0
```

## 3. 模型配置解析与凭据引用映射

```mermaid
graph TD
    subgraph Selection["ModelSelection (Go 产生)"]
        S1["profile_id"]
        S2["source: environment / database"]
        S3["protocol: openai_chat_completions"]
        S4["model_name"]
        S5["credential_ref"]
        S6["runtime_scope: go / deepagent"]
        S7["purpose: agent_turn / selector"]
    end

    subgraph Resolver["ModelConfigResolver"]
        R1["验证必填字段"]
        R2["调用 CredentialResolver"]
        R3["合并元数据 + 凭据"]
        R4["最终验证"]
    end

    subgraph CredRef["credential_ref 解析路径"]
        C1["environment:go<br/>→ LLM_BASE_URL<br/>→ LLM_API_KEY<br/>→ LLM_MODEL"]
        C2["environment:deepagent<br/>→ MODEL_BASE_URL<br/>→ MODEL_API_KEY<br/>→ MODEL_NAME"]
        C3["profile:&lt;id&gt;<br/>→ 生产 Secret Provider Adapter<br/>无 Adapter → 失败"]
    end

    subgraph Config["ModelConfig (短生命周期)"]
        M1["base_url"]
        M2["api_key"]
        M3["model_name"]
        M4["protocol"]
        M5["profile_id"]
        M6["runtime_scope"]
    end

    S5 --> R2
    R1 --> R2
    R2 --> C1
    R2 --> C2
    R2 --> C3
    C1 --> R3
    C2 --> R3
    C3 -.->|"❌ CredentialNotFoundError"| FAIL["ModelConfigPreparationError<br/>→ FAILED 事件"]
    C3 --> R3
    R3 --> R4
    R4 --> M1
    R4 --> M2
    R4 --> M3
    R4 --> M4
    R4 --> M5
    R4 --> M6

    M2 -.->|"❌ 不进入<br/>事件/checkpoint/日志"| LEAK["密钥泄漏防护"]

    style Selection fill:#e3f2fd,stroke:#1565c0
    style Resolver fill:#fff3e0,stroke:#e65100
    style CredRef fill:#fce4ec,stroke:#c62828
    style Config fill:#e8f5e9,stroke:#2e7d32
    style FAIL fill:#ffebee,stroke:#c62828,stroke-width:2px
    style LEAK fill:#fff9c4,stroke:#f57f17,stroke-dasharray: 5 5
```

## 4. 准备阶段事件顺序（时序图）

```mermaid
sequenceDiagram
    participant CE as Collaboration Engine
    participant RRAE as RuntimeRegistryAgentExecutor
    participant MCR as ModelConfigResolver
    participant EX as Executor (LLM/DeepAgent)
    participant COORD as Go Coordinator

    CE->>RRAE: AgentTurnRequest + ModelSelection
    RRAE->>RRAE: agent_turn_started

    rect rgb(255, 243, 224)
        Note over RRAE,MCR: 准备阶段 (preparation)
        RRAE->>MCR: resolve(ModelSelection)
        alt 解析成功
            MCR-->>RRAE: ModelConfig (含 api_key)
            RRAE->>EX: ExecuteAgentRequest (含 ModelConfig)
        else CredentialNotFoundError
            MCR-->>RRAE: ModelConfigPreparationError
            RRAE->>CE: FAILED(code=model_not_configured, retryable=false)
            Note over RRAE: 不产生 model_started
        else CredentialAccessDeniedError
            MCR-->>RRAE: CredentialAccessDeniedError
            RRAE->>CE: FAILED(code=model_authentication_failed, retryable=false)
        else CredentialProviderUnavailableError
            MCR-->>RRAE: CredentialProviderUnavailableError
            RRAE->>CE: FAILED(code=engine_unavailable, retryable=true)
        end
    end

    rect rgb(232, 245, 233)
        Note over EX: 模型执行阶段
        EX->>CE: model_started
        EX->>CE: output_delta* (流式)
        EX->>CE: model_completed
        EX->>CE: completed (content + artifacts + usage)
    end

    CE->>COORD: Event Stream
    COORD->>COORD: 验证 + 事务提交 + 审计
    COORD->>COORD: WebSocket 广播
```

## 5. 协作运行时 gRPC 流程

```mermaid
sequenceDiagram
    participant H as Human (浏览器)
    participant API as Go API Server
    participant SVC as RoomService
    participant CC as CollaborationCoordinator
    participant CR as CollaborationRuntime (Python)
    participant ENG as Engine (Native/AutoGen)
    participant DB as MySQL

    H->>API: 发送消息 (WebSocket)
    API->>SVC: 持久化人类消息
    SVC->>DB: INSERT message
    SVC->>API: 广播人类消息
    API->>H: 实时推送

    SVC->>CC: 创建 collaboration run
    CC->>CC: 容量检查 + 房间串行锁

    CC->>CR: gRPC ExecuteConversation(snapshot)
    Note over CC,CR: ModelSelection 含 credential_ref<br/>不含明文 API Key

    CR->>ENG: execute(request, cancel_event)
    loop 轮次循环
        ENG->>ENG: speaker_selected
        ENG->>ENG: agent_turn_started
        ENG->>CR: ExecutorEvent (model_started / output_delta / model_completed)
        ENG->>CR: ExecutorEvent (completed)
        ENG->>CR: agent_message_completed
    end

    alt 正常完成
        ENG-->>CR: COMPLETED
    else 准备阶段失败
        ENG-->>CR: FAILED(model_not_configured)
        Note over ENG: 无 HTTP 请求<br/>不回退环境凭据
    else 取消
        ENG-->>CR: CANCELLED
    end

    CR-->>CC: Event Stream (gRPC)
    loop 事件验证
        CC->>CC: 验证事件顺序
        CC->>DB: 幂等事务提交 Agent 消息
        CC->>API: 广播事件
        API->>H: WebSocket 推送
    end
    CC->>DB: 更新 collaboration_runs 终态
```

## 6. 四层职责边界（ADR-001）

```mermaid
graph TD
    subgraph L1["① Go Control Plane"]
        L1A["决定使用哪个模型"]
        L1B["产生 ModelSelection<br/>profile_id + credential_ref"]
        L1C["房间/Agent/Profile 数据权威"]
        L1D["MySQL 持久化 + WebSocket 广播"]
    end

    subgraph L2["② Collaboration Engine"]
        L2A["选择下一位 Agent"]
        L2B["轮次/handoff/cooldown 策略"]
        L2C["映射执行结果为协作事件"]
        L2D["❌ 不知 CredentialResolver 存在"]
    end

    subgraph L3["③ Agent Execution"]
        L3A["RuntimeRegistryAgentExecutor"]
        L3B["准备阶段：resolve ModelConfig"]
        L3C["失败映射为稳定 FAILED 事件"]
        L3D["选择具体 Executor"]
    end

    subgraph L4["④ Model Execution"]
        L4A["ModelConfigResolver (唯一入口)"]
        L4B["CredentialResolver"]
        L4C["ModelConfig (短生命周期)"]
        L4D["ModelExecutionService<br/>AutoGen Selector 入口"]
        L4E["ModelClient → Provider HTTP"]
    end

    L1B -->|"gRPC 快照"| L2A
    L2A -->|"AgentTurn +<br/>ModelSelection"| L3A
    L3B --> L4A
    L4A --> L4B
    L4B --> L4C
    L4C --> L3D
    L2A -->|"selector purpose"| L4D
    L4D --> L4A
    L4D --> L4E

    L1C -.->|"❌ Python 不访问<br/>AgentRoom MySQL"| L4B
    L2D -.->|"❌ Engine 不 import<br/>model_config"| L4A
    L4C -.->|"❌ 不序列化/持久化/<br/>进入事件/checkpoint"| L1D

    style L1 fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px
    style L2 fill:#fff3e0,stroke:#e65100,stroke-width:2px
    style L3 fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px
    style L4 fill:#e3f2fd,stroke:#1565c0,stroke-width:2px
```

## 7. 凭据安全边界

```mermaid
graph LR
    subgraph Allowed["✅ credential_ref 允许出现"]
        A1["ModelConfigResolver"]
        A2["CredentialResolver"]
        A3["ModelSelection (数据类)"]
        A4["Protobuf 合约"]
        A5["Mapping (Go→Python)"]
    end

    subgraph Forbidden["❌ credential_ref 禁止出现"]
        F1["Collaboration Engine"]
        F2["Executor (LLM/DeepAgent)"]
        F3["事件流 EngineEvent"]
        F4["Checkpoint"]
        F5["日志 / 审计对象"]
        F6["ModelClientFactory"]
    end

    subgraph Secret["❌ api_key 禁止出现"]
        S1["Collaboration gRPC 请求"]
        S2["事件 / Checkpoint"]
        S3["Protobuf state/events"]
        S4["日志"]
    end

    A1 -->|"验证"| Forbidden
    A1 -->|"验证"| Secret

    style Allowed fill:#e8f5e9,stroke:#2e7d32
    style Forbidden fill:#ffebee,stroke:#c62828
    style Secret fill:#fce4ec,stroke:#ad1457
```

## 8. Docker Compose 部署拓扑

```mermaid
graph TB
    subgraph Host["Docker Host"]
        subgraph Compose["Docker Compose"]
            FE_C["frontend (nginx)<br/>Vite 构建 + /api 代理<br/>:5173"]
            BE_C["backend (Go)<br/>Gin + WebSocket<br/>:8080"]
            AR_C["agent-runtime (Python)<br/>gRPC :50051<br/>单 Agent + 协作引擎"]
            DB_C["mysql (MySQL 8)<br/>:3306"]
        end
    end

    Browser["浏览器"] <-->|":5173"| FE_C
    FE_C <-->|"/api 反向代理"| BE_C
    BE_C <-->|"gRPC :50051"| AR_C
    BE_C <-->|"TCP :3306"| DB_C
    AR_C -.->|"❌ 不访问 MySQL"| DB_C
    AR_C -->|"出站 HTTPS"| ExternalLLM["OpenAI-compatible API"]
    AR_C -->|"出站 HTTPS"| ExternalTavily["Tavily API"]

    style Compose fill:#fafafa,stroke:#999
    style AR_C fill:#fff3e0,stroke:#e65100
    style DB_C fill:#e3f2fd,stroke:#1565c0
```

---

## 图表索引

| 编号 | 名称 | 用途 |
| --- | --- | --- |
| 1 | 系统总览 | 快速了解整体组件和数据流方向 |
| 2 | 统一模型执行边界 | ADR-001 核心数据流，理解四层边界 |
| 3 | 模型配置解析 | credential_ref 映射路径和 ModelConfig 生成 |
| 4 | 准备阶段事件顺序 | 时序图，理解 preparation 失败如何映射 |
| 5 | 协作运行时 gRPC 流程 | 完整协作 run 从人类消息到事件提交 |
| 6 | 四层职责边界 | ADR-001 的职责分层和禁止项 |
| 7 | 凭据安全边界 | credential_ref 和 api_key 的允许/禁止位置 |
| 8 | Docker Compose 拓扑 | 部署视图，理解服务间网络关系 |

## 相关文档

- [ADR-001](decisions/ADR-001-control-collaboration-agent-model-execution-boundaries.md) — 统一模型执行边界决策
- [Agent Runtime 与模型](agent-runtime-and-models.md) — §9A 统一模型执行边界
- [架构索引](README.md) — 完整文档地图
