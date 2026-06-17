# AgentRoom Trusted Meeting Workbench Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the current AgentRoom implementation into a more trustworthy meeting workbench by first repairing user-visible text quality, then adding knowledge-source provenance, and finally shaping Agent setup around predefined meeting roles.

**Architecture:** Keep the existing Go/React/MySQL monolith and reuse current service boundaries. Extend `KnowledgeService` and existing API/frontend components instead of introducing a vector database, background indexer, state machine framework, UI library, or new routing layer. Each chunk should preserve existing REST/WebSocket payloads unless the chunk explicitly adds a backward-compatible field.

**Tech Stack:** Go 1.23, Gin, gorilla/websocket, GORM/MySQL, existing `llm.Client`, React 18, Vite, Node built-in test runner, current CSS modules/files.

---

## Direction

This plan deliberately avoids a big rewrite. The current system already has the important bones:

- `backend/internal/service/knowledge_service.go` parses Markdown and returns chunks.
- `backend/internal/agent/prompt_context.go` and `prompt_composer.go` already shape Agent prompts.
- `backend/internal/service/agent_service.go` owns global Agent configuration.
- `backend/internal/api/contracts/*` already isolates transport payloads.
- `frontend/src/components/KnowledgePanel.jsx`, `AgentAdmin.jsx`, `JoinScreen.jsx`, `ChatRoom.jsx`, and `RoomReadOnly.jsx` already provide the user surfaces.

The next implementation should make those surfaces reliable and more product-specific, not replace them.

## Non-Goals

- Do not add a vector database in this pass.
- Do not add formal login, organizations, or tenant management.
- Do not replace React/Vite, Gin, GORM, or the current local CSS approach.
- Do not invent a new knowledge ingestion pipeline while Markdown upload already exists.
- Do not rewrite the backend into CQRS/DDD packages.
- Do not change existing public APIs except for additive response fields.

## Target Outcomes

- All user-facing Chinese text in source files is readable UTF-8 and protected by tests.
- Knowledge snippets shown to Agents have source metadata that can be surfaced in the UI.
- Agent responses can optionally expose which knowledge documents informed the reply.
- Agent creation is guided by predefined role templates rather than blank arbitrary forms.
- Room creation can recommend a role set for common meeting scenarios without removing manual selection.

## Chunk 1: Repair Visible Text Quality

### Task 1: Lock readable Chinese copy with stronger regression tests

**Files:**
- Modify: `frontend/src/components/copyRegression.test.mjs`
- Modify: `frontend/src/components/joinScreenCopy.test.mjs`
- Test: `frontend/src/components/copyRegression.test.mjs`

- [ ] **Step 1: Strengthen mojibake detection**

Expand the current test to scan all user-facing frontend files, including:

```text
frontend/src/App.jsx
frontend/src/components/*.jsx
frontend/src/api/*.js
frontend/src/routing.js
```

Detect common mojibake tokens and broken JSX text such as:

```js
const MOJIBAKE_MARKERS = [
  '\u951b',
  '\u7ba0',
  '\u93b4',
  '\u6d7c',
  '\u9428',
  '\u9359',
  '\u9225',
  '\u20ac',
  '\ufffd',
]
```

The test should fail with file names and the first few suspicious snippets.

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
node --test frontend/src/components/copyRegression.test.mjs
```

Expected: FAIL while files still contain mojibake.

- [ ] **Step 3: Repair source copy in focused files**

Repair readable Chinese text in these files first:

```text
frontend/src/App.jsx
frontend/src/components/JoinScreen.jsx
frontend/src/components/RoomGateway.jsx
frontend/src/components/ChatRoom.jsx
frontend/src/components/AdminConsole.jsx
```

Keep component structure unchanged. This task is only copy repair, not UI redesign.

- [ ] **Step 4: Run focused frontend checks**

Run:

```powershell
node --test frontend/src/components/copyRegression.test.mjs
node --test frontend/src/components/joinScreenCopy.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

Use a Lore-style commit message:

```text
Make the meeting interface readable before the next product layer

Constraint: Existing UI structure and tests should remain stable.
Rejected: Redesign the frontend while fixing copy | copy repair should stay low risk.
Confidence: high
Scope-risk: narrow
Tested: node --test frontend/src/components/copyRegression.test.mjs; node --test frontend/src/components/joinScreenCopy.test.mjs; npm --prefix frontend run build
```

### Task 2: Repair docs encoding without rewriting product intent

**Files:**
- Modify: `docs/agentroom-requirements.md`
- Modify: `docs/data-persistence-design.md`
- Test: `docs/agentroom-requirements.md`
- Test: `docs/data-persistence-design.md`

- [ ] **Step 1: Add a lightweight docs check script or test**

Prefer extending the existing Node test style rather than adding dependencies. Create or extend a root-level test only if needed:

```text
deploymentArtifacts.test.mjs
```

Assert important docs do not contain common mojibake markers.

- [ ] **Step 2: Rewrite garbled docs from current README and code reality**

Use `README.md`, `docs/ARCHITECTURE.md`, and the current implementation as source of truth. Do not invent new requirements in this task.

Required sections:

- 产品定位
- 当前角色
- 当前功能边界
- 会议生命周期
- 知识库能力
- 已知限制
- 下一步建议

- [ ] **Step 3: Run docs and existing deployment tests**

Run:

```powershell
node --test deploymentArtifacts.test.mjs
```

Expected: PASS.

- [ ] **Step 4: Commit**

```text
Restore readable product docs for implementation handoff

Constraint: Documentation must reflect the current repository, not a future wish list.
Rejected: Replace docs with a broad new PRD | this chunk only repairs current-state docs.
Confidence: high
Scope-risk: narrow
Tested: node --test deploymentArtifacts.test.mjs
```

## Chunk 2: Knowledge Source Provenance

### Task 3: Preserve document names on selected knowledge chunks

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/knowledge_repo.go`
- Modify: `backend/internal/tests/teststore/store.go`
- Modify: `backend/internal/tests/service/knowledge_service_test.go`
- Test: `backend/internal/tests/service/knowledge_service_test.go`

- [ ] **Step 1: Write failing service test**

Add a test proving `SearchForAgent` returns chunks with document source metadata:

```go
func TestKnowledgeServiceSearchReturnsChunkSources(t *testing.T) {
	// Upload room and agent Markdown documents.
	// Search as an agent.
	// Assert returned chunks include DocumentID and DocumentName/FileName.
}
```

- [ ] **Step 2: Extend domain model additively**

Add optional fields to `model.KnowledgeChunk`:

```go
type KnowledgeChunk struct {
	ID           string    `json:"id"`
	DocumentID   string    `json:"documentId"`
	DocumentName string    `json:"documentName,omitempty"`
	Scope        string    `json:"scope"`
	ScopeID      string    `json:"scopeId"`
	ChunkIndex   int       `json:"chunkIndex"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"createdAt"`
}
```

Do not change the database schema for this. Join `knowledge_documents` during search and hydrate the field.

- [ ] **Step 3: Update MySQL search implementation**

In `backend/internal/store/mysql/knowledge_repo.go`, update `SearchKnowledgeChunks` to join document metadata:

```sql
SELECT c.*, d.file_name
FROM knowledge_chunks c
JOIN knowledge_documents d ON d.id = c.document_id
WHERE c.scope = ? AND c.scope_id = ?
...
```

Keep existing text matching and ranking. Do not introduce embeddings.

- [ ] **Step 4: Update test store**

Make `backend/internal/tests/teststore/store.go` hydrate `DocumentName` from stored documents.

- [ ] **Step 5: Run targeted tests**

Run:

```powershell
go -C backend test ./internal/tests/service -run KnowledgeService
go -C backend test ./internal/tests/agent -run Knowledge
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
Attach document provenance to knowledge chunks

Constraint: Reuse the existing Markdown chunk store before adding retrieval infrastructure.
Rejected: Add a vector database | source visibility does not require semantic search yet.
Confidence: high
Scope-risk: narrow
Tested: go -C backend test ./internal/tests/service -run KnowledgeService; go -C backend test ./internal/tests/agent -run Knowledge
```

### Task 4: Put knowledge provenance into Agent prompt context

**Files:**
- Modify: `backend/internal/agent/prompt_context.go`
- Modify: `backend/internal/agent/prompt_composer.go`
- Modify: `backend/internal/tests/agent/prompt_context_test.go`
- Test: `backend/internal/tests/agent/prompt_context_test.go`

- [ ] **Step 1: Add failing prompt test**

Add a test proving prompt context includes source labels:

```go
func TestRunnerPromptLabelsKnowledgeSources(t *testing.T) {
	// Build context with two chunks from different documents.
	// Compose prompt.
	// Assert the prompt contains document names near the chunk content.
}
```

- [ ] **Step 2: Extend existing prompt formatting**

Reuse the current knowledge section. Change only the chunk display format, for example:

```text
[room: roadmap.md #1]
...

[agent: qa-playbook.md #2]
...
```

Do not add a separate prompt engine.

- [ ] **Step 3: Run agent prompt tests**

Run:

```powershell
go -C backend test ./internal/tests/agent -run Prompt
```

Expected: PASS.

- [ ] **Step 4: Commit**

```text
Show source labels inside Agent knowledge context

Constraint: Agent prompting already has a structured context composer.
Rejected: Build a new citation renderer in the runner | prompt context is the existing source of truth.
Confidence: medium
Scope-risk: narrow
Tested: go -C backend test ./internal/tests/agent -run Prompt
```

## Chunk 3: Response-Level Source Visibility

### Task 5: Store response citations as additive message metadata

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/models.go`
- Modify: `backend/internal/store/mysql/messages_repo.go`
- Create: `backend/internal/store/mysql/migrations/004_message_sources.sql`
- Modify: `backend/internal/tests/teststore/store.go`
- Modify: `backend/internal/tests/agent/runner_knowledge_test.go`
- Test: `backend/internal/tests/agent/runner_knowledge_test.go`

- [ ] **Step 1: Write failing runner test**

Add a test proving an Agent message created after knowledge search carries source summaries:

```go
func TestRunnerPersistsKnowledgeSourcesOnAgentMessage(t *testing.T) {}
```

- [ ] **Step 2: Add a small metadata type**

In `model/types.go`:

```go
type MessageKnowledgeSource struct {
	DocumentID   string `json:"documentId"`
	DocumentName string `json:"documentName"`
	Scope        string `json:"scope"`
}

type Message struct {
	...
	KnowledgeSources []MessageKnowledgeSource `json:"knowledgeSources,omitempty"`
}
```

- [ ] **Step 3: Persist as JSON in MySQL**

Add a nullable `knowledge_sources_json` column to `messages`.

Reference migration:

```sql
ALTER TABLE messages
  ADD COLUMN knowledge_sources_json JSON NULL AFTER parent_message_id;
```

If MySQL JSON support is inconvenient with current GORM model style, use `TEXT` containing JSON. Keep the domain API the same.

- [ ] **Step 4: Fill sources in Agent runner**

In `backend/internal/agent/runner.go`, after `searchKnowledge`, derive unique document sources from chunks and attach them to generated Agent messages.

Do not ask the LLM to produce citations in this step. This is deterministic provenance from retrieval.

- [ ] **Step 5: Update API responses through existing message model**

Because API already returns `model.Message`, this should be additive and frontend-compatible.

- [ ] **Step 6: Run focused backend tests**

Run:

```powershell
go -C backend test ./internal/tests/agent -run Knowledge
go -C backend test ./internal/tests/api -run Messages
```

Expected: PASS.

- [ ] **Step 7: Commit**

```text
Persist deterministic knowledge sources on Agent messages

Constraint: Source visibility should come from retrieval, not model self-reporting.
Rejected: Ask the model to cite documents | generated citations can drift from actual retrieval.
Confidence: medium
Scope-risk: moderate
Tested: go -C backend test ./internal/tests/agent -run Knowledge; go -C backend test ./internal/tests/api -run Messages
```

### Task 6: Display sources below Agent messages

**Files:**
- Modify: `frontend/src/components/MessageList.jsx`
- Modify: `frontend/src/components/copyRegression.test.mjs`
- Create: `frontend/src/components/messageSources.test.mjs`
- Modify: `frontend/src/chat-room.css`
- Test: `frontend/src/components/messageSources.test.mjs`

- [ ] **Step 1: Add source rendering test**

Create a source-level test that verifies `MessageList.jsx` has a source display path for `knowledgeSources`.

Keep this consistent with existing frontend tests that inspect source/pure helpers rather than requiring a DOM test library.

- [ ] **Step 2: Render compact source chips**

For Agent messages with `knowledgeSources`, render a small row:

```text
参考：roadmap.md · qa-playbook.md
```

Keep it passive. Do not make source chips open documents yet.

- [ ] **Step 3: Style source chips**

Add compact CSS in `frontend/src/chat-room.css`.

- [ ] **Step 4: Run frontend checks**

Run:

```powershell
node --test frontend/src/components/messageSources.test.mjs
node --test frontend/src/components/copyRegression.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```text
Surface retrieved knowledge sources under Agent replies

Constraint: Keep the meeting timeline dense and readable.
Rejected: Add a document preview drawer | provenance chips are enough for this pass.
Confidence: high
Scope-risk: narrow
Tested: node --test frontend/src/components/messageSources.test.mjs; node --test frontend/src/components/copyRegression.test.mjs; npm --prefix frontend run build
```

## Chunk 4: Predefined Role Templates

### Task 7: Add reusable Agent role templates in the backend

**Files:**
- Create: `backend/internal/agent/templates.go`
- Modify: `backend/internal/agent/registry.go`
- Modify: `backend/internal/api/contracts/agents.go`
- Modify: `backend/internal/api/rest_handlers.go`
- Create: `backend/internal/tests/agent/templates_test.go`
- Create: `backend/internal/tests/api/agent_templates_test.go`
- Test: `backend/internal/tests/agent/templates_test.go`

- [ ] **Step 1: Add failing template tests**

Cover:

- templates have stable IDs
- each template has name, role, description, system prompt
- default predefined agents can be derived from templates
- API returns templates without exposing unrelated internal state

- [ ] **Step 2: Define templates once**

Create `backend/internal/agent/templates.go`:

```go
type RoleTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
}

func RoleTemplates() []RoleTemplate
func AgentFromTemplate(template RoleTemplate) model.Agent
```

Initial templates:

- product_manager
- architect
- qa_reviewer
- risk_reviewer
- meeting_scribe

Reuse existing predefined agent content where possible.

- [ ] **Step 3: Add read-only API endpoint**

Add:

```text
GET /api/agent-templates
```

Response:

```json
{ "templates": [...] }
```

This is read-only and should not require admin auth.

- [ ] **Step 4: Keep seeding behavior stable**

Update `PredefinedAgents()` to derive from templates while preserving stable names/mentions where tests depend on them.

- [ ] **Step 5: Run backend tests**

Run:

```powershell
go -C backend test ./internal/tests/agent ./internal/tests/api -run "Template|Agent"
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
Ground Agent setup in reusable meeting role templates

Constraint: Existing Agent configuration and room snapshots must remain compatible.
Rejected: Store templates in a new database table | templates are product defaults, not user data yet.
Confidence: high
Scope-risk: narrow
Tested: go -C backend test ./internal/tests/agent ./internal/tests/api -run "Template|Agent"
```

### Task 8: Use templates in Agent admin without removing manual control

**Files:**
- Modify: `frontend/src/api/roomClient.js`
- Modify: `frontend/src/api/roomClient.test.mjs`
- Modify: `frontend/src/components/AgentAdmin.jsx`
- Create: `frontend/src/components/agentTemplates.test.mjs`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/components/agentTemplates.test.mjs`

- [ ] **Step 1: Add client helper test**

Test:

```js
getAgentTemplates()
```

calls `/api/agent-templates` and parses `{ templates }`.

- [ ] **Step 2: Add a template picker to Agent admin**

In `AgentAdmin.jsx`, add a small template selector above the create form:

- choose template
- prefill name, role, description, systemPrompt
- allow user edits before save

Do not remove the existing manual form.

- [ ] **Step 3: Add source-level frontend test**

Assert `AgentAdmin.jsx` imports/uses `getAgentTemplates` and keeps create/update paths intact.

- [ ] **Step 4: Run frontend checks**

Run:

```powershell
node --test frontend/src/api/roomClient.test.mjs
node --test frontend/src/components/agentTemplates.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```text
Let admins create Agents from role templates

Constraint: Templates should speed setup without removing editable Agent configuration.
Rejected: Force every Agent to match a template | teams still need manual role edits.
Confidence: high
Scope-risk: narrow
Tested: node --test frontend/src/api/roomClient.test.mjs; node --test frontend/src/components/agentTemplates.test.mjs; npm --prefix frontend run build
```

## Chunk 5: Meeting Role Sets

### Task 9: Recommend role sets during room creation

**Files:**
- Modify: `backend/internal/agent/templates.go`
- Modify: `backend/internal/api/contracts/agents.go`
- Modify: `backend/internal/api/rest_handlers.go`
- Create: `backend/internal/tests/api/role_sets_test.go`
- Modify: `frontend/src/api/roomClient.js`
- Modify: `frontend/src/components/JoinScreen.jsx`
- Create: `frontend/src/components/roleSets.test.mjs`
- Modify: `frontend/src/styles.css`
- Test: `frontend/src/components/roleSets.test.mjs`

- [ ] **Step 1: Add backend role-set endpoint**

Add simple static role sets:

```text
GET /api/agent-role-sets
```

Suggested response:

```json
{
  "roleSets": [
    {
      "id": "product_review",
      "name": "产品评审",
      "description": "适合需求、方案和风险评审",
      "templateIDs": ["product_manager", "architect", "qa_reviewer", "risk_reviewer"]
    }
  ]
}
```

Do not persist role sets yet.

- [ ] **Step 2: Add frontend helper**

Add:

```js
export async function getAgentRoleSets()
```

- [ ] **Step 3: Use role sets as selection shortcuts**

In `JoinScreen.jsx`, show role-set buttons near the Agent picker. Clicking a role set should select currently available agents whose names/template-derived roles match the set.

Because existing persisted Agents do not yet store `templateID`, match by stable default template names in this pass. If this feels brittle during implementation, add optional `templateID` to newly created agents in a later chunk, not here.

- [ ] **Step 4: Keep manual selection**

Role sets are shortcuts. The user can still select/deselect individual Agents.

- [ ] **Step 5: Run tests**

Run:

```powershell
go -C backend test ./internal/tests/api -run RoleSet
node --test frontend/src/components/roleSets.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```text
Offer meeting role sets as room-creation shortcuts

Constraint: Room creation already supports selecting explicit Agent IDs.
Rejected: Create a separate meeting-template subsystem | role sets only need selection shortcuts now.
Confidence: medium
Scope-risk: narrow
Tested: go -C backend test ./internal/tests/api -run RoleSet; node --test frontend/src/components/roleSets.test.mjs; npm --prefix frontend run build
```

## Chunk 6: Verification And Documentation

### Task 10: Update docs for the new direction

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/agentroom-requirements.md`

- [ ] **Step 1: Update README**

Add concise notes for:

- readable UTF-8 UI copy expectation
- knowledge source chips under Agent replies
- role templates
- role-set shortcuts

- [ ] **Step 2: Update architecture docs**

Document:

- source metadata stays in `KnowledgeChunk`
- deterministic message provenance is stored on `Message`
- role templates are static product defaults in `agent/templates.go`
- role sets are selection shortcuts, not persisted meeting templates

- [ ] **Step 3: Run full backend verification**

Run:

```powershell
go -C backend test ./...
go -C backend vet ./...
go -C backend build ./cmd/server
```

Expected: PASS.

- [ ] **Step 4: Run full frontend verification**

Run:

```powershell
$tests = Get-ChildItem -Path frontend\src -Recurse -Filter *.test.mjs | ForEach-Object { $_.FullName }
node --test $tests
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Run deployment artifact tests**

Run:

```powershell
node --test deploymentArtifacts.test.mjs
```

Expected: PASS.

- [ ] **Step 6: Manual smoke test**

Run the app and verify:

- home page copy is readable
- create a room with a role-set shortcut
- upload room Markdown and Agent Markdown
- mention an Agent and see a source chip under its reply
- create an Agent from a template, then edit it before saving
- closed room read-only surface still works
- admin console still loads

- [ ] **Step 7: Commit**

```text
Document the trusted meeting workbench direction

Constraint: Docs should describe the shipped behavior from this iteration.
Rejected: Promise vector search and account permissions | those remain future design work.
Confidence: high
Scope-risk: narrow
Tested: go -C backend test ./...; go -C backend vet ./...; go -C backend build ./cmd/server; node --test frontend tests; npm --prefix frontend run build; node --test deploymentArtifacts.test.mjs
```

## Suggested Execution Order

1. Chunk 1 first. A product with unreadable text cannot be evaluated.
2. Chunk 2 next. Provenance starts in the retrieval layer and does not affect UI yet.
3. Chunk 3 after provenance is stable. This adds user-visible trust without changing retrieval.
4. Chunk 4 after sources are visible. Templates clarify what Agents are supposed to be.
5. Chunk 5 last. Role sets are useful only once templates are clear.

## Architecture Guardrails

- Reuse `KnowledgeService.SearchForAgent`; do not create a parallel retrieval service.
- Reuse `model.Message`; add optional metadata rather than creating a separate timeline entity.
- Reuse `AgentService`; templates should feed Agent creation, not bypass global Agent management.
- Reuse `JoinScreen` selection logic; role sets should only manipulate `selectedAgentIds`.
- Keep static role templates in code for now. A database-backed template library is premature.
- Keep source chips deterministic from retrieved chunks. Do not rely on model-generated citations.
- Keep tests near the existing test organization: backend behavior under `backend/internal/tests/**`, frontend source/pure tests near `frontend/src/components/**`.

## Future Work Not In This Plan

- Embedding search or hybrid BM25/vector retrieval.
- PDF, Word, image, or code-repository ingestion.
- Formal user accounts, organizations, and long-lived permissions.
- Document preview drawer from source chips.
- Meeting agenda templates and action-item workflow.
- Multi-instance realtime scaling with Redis or a queue.
