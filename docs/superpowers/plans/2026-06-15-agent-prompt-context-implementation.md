# Agent Prompt Context Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify agent prompt construction so both dialogue modes receive the same structured room context, while keeping only the agent role template editable.

**Architecture:** Add a shared prompt-context builder plus a langchaingo-backed prompt composer under `backend/internal/agent/`, then route both `mention_fanout` and `guided_dialogue` through it. Keep retrieval, run tracking, and dialogue scheduling behavior unchanged while expanding room-awareness data and admin wording.

**Tech Stack:** Go, langchaingo `prompts`, existing `llm.ChatMessage` adapter, React, Node test runner

---

## Chunk 1: Shared Prompt Infrastructure

### Task 1: Lock prompt-context behavior with backend tests

**Files:**
- Create: `backend/internal/tests/agent/prompt_context_test.go`
- Modify: `backend/internal/tests/agent/runner_knowledge_test.go`
- Modify: `backend/internal/tests/agent/dialogue_phase2_test.go`

- [ ] **Step 1: Add failing mention-fanout tests**
  Cover online human participants, room agent roster, trigger sender, latest visible speaker, transcript ordering, and system-message filtering.
- [ ] **Step 2: Run targeted backend tests and verify RED**
  Run: `go test ./internal/tests/agent -run "TestPromptContext|TestRunnerIncludesRoomAndAgentKnowledgeInPrompt"`
  Expected: failures for missing prompt context/composer support.
- [ ] **Step 3: Add failing guided-dialogue tests**
  Cover immediate `trigger*`, stable `rootHumanTrigger*`, eligible peers, and guided policy metadata.
- [ ] **Step 4: Run targeted backend tests again and verify RED**
  Run: `go test ./internal/tests/agent -run "TestPromptContext|TestGuidedDialogue"`
  Expected: failures on guided prompt assertions before implementation.

### Task 2: Implement shared prompt context + composer

**Files:**
- Create: `backend/internal/agent/prompt_context.go`
- Create: `backend/internal/agent/prompt_composer.go`
- Modify: `backend/internal/agent/runner.go`
- Modify: `backend/internal/agent/dialogue.go`

- [ ] **Step 1: Extend `RuntimeRoom`**
  Add `Participants() []model.Participant` and update runtime test doubles.
- [ ] **Step 2: Implement deterministic `PromptContext` builders**
  Filter system messages from transcript, derive latest visible speaker, keep intentional trigger duplication, and capture guided-mode metadata.
- [ ] **Step 3: Implement langchaingo prompt composer**
  Render system contract + role template + meeting context + mode constraints + transcript/knowledge + output contract into `[]llm.ChatMessage`.
- [ ] **Step 4: Refactor runner + dialogue to use the shared composer**
  Preserve retrieval semantics: mention mode uses current human trigger content; guided mode uses the immediate parent trigger content.
- [ ] **Step 5: Run targeted backend tests and verify GREEN**
  Run: `go test ./internal/tests/agent -run "TestPromptContext|TestRunnerIncludesRoomAndAgentKnowledgeInPrompt|TestGuidedDialogue"`

## Chunk 2: Role Template Semantics And UI Copy

### Task 3: Align default agent templates with role-template ownership

**Files:**
- Modify: `backend/internal/agent/registry.go`

- [ ] **Step 1: Remove platform rules from predefined templates**
  Keep defaults focused on role voice, scope, and collaboration style only.
- [ ] **Step 2: Run targeted backend tests**
  Run: `go test ./internal/tests/agent/...`

### Task 4: Update admin wording to “role template”

**Files:**
- Modify: `frontend/src/components/AgentAdmin.jsx`
- Modify: `frontend/src/components/copyRegression.test.mjs`

- [ ] **Step 1: Update labels, helper copy, and collapse text**
  Replace generic “system prompt/behavior rules” wording with “role template” semantics.
- [ ] **Step 2: Update copy regression expectations**
  Keep assertions aligned with the new visible text in `AgentAdmin.jsx`.
- [ ] **Step 3: Run targeted frontend copy tests**
  Run: `node --test src/components/copyRegression.test.mjs`

## Chunk 3: Full Verification

### Task 5: Run repository verification

**Files:**
- Modify: `backend/internal/agent/runner.go`
- Modify: `backend/internal/agent/dialogue.go`
- Modify: `backend/internal/agent/registry.go`
- Modify: `frontend/src/components/AgentAdmin.jsx`
- Modify: `frontend/src/components/copyRegression.test.mjs`
- Create: `backend/internal/agent/prompt_context.go`
- Create: `backend/internal/agent/prompt_composer.go`
- Create: `backend/internal/tests/agent/prompt_context_test.go`

- [ ] **Step 1: Run backend tests**
  Run: `go test ./...`
- [ ] **Step 2: Run backend static verification**
  Run: `go vet ./...`
- [ ] **Step 3: Build backend server**
  Run: `go build ./cmd/server`
- [ ] **Step 4: Build frontend**
  Run: `npm run build`
- [ ] **Step 5: Summarize changed files, verification evidence, and residual risk**
