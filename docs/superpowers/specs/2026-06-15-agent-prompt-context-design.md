# Agent Prompt Context And Management Design

Date: 2026-06-15
Status: Approved for implementation planning

## Background

AgentRoom now has two room dialogue modes: `mention_fanout` and `guided_dialogue`. The current prompt assembly is split across `backend/internal/agent/runner.go` and `backend/internal/agent/dialogue.go`, and standard agent replies only receive recent messages, trigger content, and optional knowledge snippets. This means an agent can respond to a meeting without clearly knowing who is currently in the room or which peer agents are available.

At the same time, prompt ownership is too loose. The editable `systemPrompt` field currently mixes role definition with platform rules, while fixed room-level rules are still embedded as hand-built strings in multiple places.

## Goals

- Every triggered agent should perceive the current meeting as a shared room, not an isolated direct message.
- Agents should consistently see the current online human participants and the full room agent roster.
- Prompt construction should be unified across `mention_fanout` and `guided_dialogue`.
- Only the agent role template should remain editable from management surfaces.
- The prompt stack should be deterministic, testable, and compatible with the current `langchaingo` integration.

## Non-Goals

- No room-level editable prompt layer.
- No database rename for the existing `systemPrompt` field in phase 1.
- No transcript summarization or long-term memory layer in this change.
- No replacement of local dialogue policy with model-native autonomous agent orchestration.

## Decision Summary

### 1. Prompt ownership model

Prompt content is split into two ownership classes:

- **Code-owned prompt layers**: system contract, room context, mode constraints, transcript, knowledge snippets, and output contract.
- **Admin-editable prompt layer**: the agent role template currently stored in `systemPrompt`.

The `systemPrompt` field remains in storage and APIs for backward compatibility, but its semantic meaning is narrowed to **agent role template**. It must no longer be treated as the place where platform-wide behavioral rules live.

### 2. Unified meeting context packet

All agent replies will be built from one structured `PromptContext` object. Both dialogue modes reuse the same base context.

The context packet includes:

- **Room**
  - `roomName`
  - `dialogueMode`
- **Online human participants**
  - current online humans only
  - display name only
- **Room agent roster**
  - `name`
  - `mention`
  - `role`
  - `description`
- **Turn context**
  - `triggerSender`
  - `triggerSenderType`
  - `triggerContent`
  - `latestVisibleSpeaker`
  - `latestVisibleSpeakerType`
- **Transcript**
  - recent visible room messages using the existing bounded history window
- **Knowledge snippets**
  - room-scoped and agent-scoped snippets exactly as today
- **Guided-dialogue additions**
  - current speaker
  - autonomous turn index
  - eligible peers
  - turn and mention policy constraints

This packet intentionally excludes internal IDs, passcode state, and other operational metadata that do not improve response quality.

### 3. Prompt layering

Every generated reply should be composed from the same six layers:

1. **System contract**
   - fixed code-owned guardrails
   - one visible room message only
   - no hidden reasoning output
   - do not impersonate other roles
   - stay within role boundaries
2. **Agent role template**
   - editable role-specific guidance from `systemPrompt`
3. **Meeting context**
   - room mode, online humans, room agents, trigger and recent speaker metadata
4. **Mode constraints**
   - `mention_fanout`: direct response to a human mention
   - `guided_dialogue`: speaker turn, eligible peers, limits, and stop conditions
5. **Transcript and knowledge**
   - bounded visible history plus retrieved snippets
6. **Output contract**
   - concise, room-visible, implementation-safe response instructions

Mode-specific behavior is an additive layer. This change does not create two separate prompt systems.

## Module Boundaries

### RuntimeRoom

`RuntimeRoom` must grow one new capability:

- `Participants() []model.Participant`

This allows the agent layer to build room awareness from runtime state without reaching through service or storage layers.

### New agent prompt modules

Introduce two prompt-focused modules under `backend/internal/agent/`:

- `prompt_context.go`
  - defines `PromptContext`
  - converts runtime room state, responder, trigger, policy, and knowledge into deterministic prompt input
- `prompt_composer.go`
  - renders `PromptContext` into `[]llm.ChatMessage`
  - owns shared templates and mode-specific prompt fragments

`runner.go` and `dialogue.go` should stop hand-building user prompt strings and instead call the shared composer.

### Template mechanism

Use `langchaingo/prompts` for the shared prompt composer, following the same style already used by `focus_service.go` and `minutes_service.go`. This keeps the LLM adapter contract unchanged while replacing ad hoc string construction with explicit prompt templates.

## Data And Ordering Rules

- Online participants are limited to current online human participants only.
- Participant ordering follows the existing room ordering by join time.
- Agent roster ordering follows the existing room agent order.
- Transcript ordering remains chronological.
- `latestVisibleSpeaker` is derived from the latest visible message in the transcript; if no prior visible message exists, it falls back to the trigger sender.

These ordering rules keep prompt rendering stable and make tests predictable.

## Management Rules

- The management API continues to read and write `systemPrompt`.
- Documentation and UI should describe that field as **agent role template** rather than generic system behavior.
- Room context, system contract, mode constraints, and output contract are not editable from the admin surface.

## Testing Strategy

Add tests under `backend/internal/tests/agent` for:

- `PromptContext` assembly in `mention_fanout`
  - includes online humans
  - includes room agent roster
  - includes trigger sender and latest visible speaker
- `PromptContext` assembly in `guided_dialogue`
  - includes the shared meeting context
  - includes eligible peers and guided policy metadata
- prompt composer layering
  - fixed system contract is always present
  - role template is injected in the intended layer
  - output contract remains present regardless of role template content
- runner regressions
  - standard replies now include meeting-member awareness
  - guided replies keep current peer-handoff behavior while gaining full room awareness

Existing mention, scheduling, duplicate-suppression, and provider-error tests must continue to pass.

## Rollout Plan

### Phase 1: Prompt infrastructure

- add `Participants()` to `RuntimeRoom`
- introduce `PromptContext`
- introduce `PromptComposer`
- keep generated behavior close to current output shape

### Phase 2: Standard mode migration

- switch `mention_fanout` to the shared composer
- inject online participants and full room agent roster

### Phase 3: Guided mode migration

- switch `guided_dialogue` to the same composer
- preserve existing turn scheduling and stop rules
- add guided constraints as a dedicated layer on top of the shared meeting context

### Phase 4: Documentation and management wording

- update architecture and contributor docs
- rename management-facing copy from generic "system prompt" language to "role template" where appropriate

## Acceptance Criteria

This design is ready for implementation planning when the following are true:

- a triggered agent can identify the current online human participants and room agents from its prompt
- both room dialogue modes use the same shared prompt context model
- only the role template remains editable outside code
- no new dependency is introduced
- no storage migration is required for renaming `systemPrompt`
- all prompt-related behavior is covered by deterministic tests
