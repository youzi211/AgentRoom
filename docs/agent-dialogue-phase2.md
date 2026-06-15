# Agent-to-Agent Dialogue Phase 2

## Goal

Add a controlled multi-agent dialogue mode on top of the migrated `langchaingo` backend without rewriting the room transport, persistence flow, or agent identity model.

This phase exists because the current AgentRoom behavior is still human-triggered:
- a human message mentions one or more agents;
- the runner fans out one response per mentioned agent;
- agents do not currently reply to each other.

After the LLM migration, the backend now supports richer message-role handling and structured prompt assembly, which makes a second-phase dialogue orchestrator practical without another model-layer rewrite.

## Non-Goals

- Do not introduce fully autonomous framework agents that bypass room policy.
- Do not let agents run indefinitely without turn limits.
- Do not replace room history with framework memory buffers.
- Do not change the WebSocket event contract unless a concrete product need appears.

## Proposed Runtime Model

### 1. Trigger modes

Support two explicit room-level modes:

- `mention_fanout`
  - current behavior
  - only directly mentioned agents respond

- `guided_dialogue`
  - a human message can start a bounded multi-turn exchange
  - one agent responds first, then eligible agents may continue according to policy

The default remains `mention_fanout`.

### 2. Conversation policy

Store a room-scoped dialogue policy object:

```json
{
  "mode": "mention_fanout",
  "max_autonomous_turns": 3,
  "max_turns_per_agent": 1,
  "allow_self_followup": false,
  "allow_agent_to_agent_mentions": true,
  "response_strategy": "mentioned_first",
  "cooldown_ms": 0
}
```

This policy should live in local orchestration code, not inside `langchaingo`.

### 3. Turn scheduler

Split the runner into four conceptual stages:

1. `SelectInitialResponders`
2. `GenerateAgentMessage`
3. `PersistAndBroadcastGeneratedMessage`
4. `SelectNextSpeakerOrStop`

Today, stages 2 and 3 mostly exist already. Phase 2 mainly formalizes stages 1 and 4.

### 4. Next-speaker selection

Start with simple deterministic rules:

- directly mentioned agents get first priority
- an agent cannot speak twice in a row unless policy allows it
- an agent cannot exceed `max_turns_per_agent`
- stop if no eligible next speaker remains

Later, a model-assisted speaker selector can be added, but only as an advisor to the policy layer.

## Safety and Loop Guards

### Hard stop conditions

Stop the dialogue when any of these becomes true:

- total autonomous turns reached `max_autonomous_turns`
- the same agent would repeat without permission
- no new eligible agent exists
- generated content is empty or repeated
- moderation or provider error occurs

### Duplicate suppression

Before scheduling the next turn, compare normalized generated text against:

- the previous assistant turn
- the current trigger chain's recent generated turns

If the content is identical or near-identical, stop the loop.

### Optional cooldown

If room traffic is high, insert a small delay between autonomous turns to preserve message ordering and UI readability.

## Prompt Model

Each generated turn should include:

- room conversation history
- trigger chain metadata
- current speaker identity
- eligible peers
- policy constraints
- stop instruction: only produce one visible room message

The current migrated adapter already supports:

- system messages
- human messages
- assistant-role history
- JSON-mode structured calls for auxiliary tasks

That is enough for phase 2 without changing the adapter contract again.

## Persistence and Observability

Add a dialogue-run record distinct from single agent runs:

- `dialogue_run_id`
- `room_id`
- `trigger_message_id`
- `mode`
- `turn_count`
- `status`
- `started_at`
- `finished_at`

Each generated agent message should optionally point back to:

- `dialogue_run_id`
- `turn_index`
- `parent_message_id`

This makes debugging and replay much easier.

## Recommended Rollout

### Phase 2A

- keep default room mode as `mention_fanout`
- add scheduler abstraction only
- add bounded `guided_dialogue` behind config or admin toggle

### Phase 2B

- allow agent-to-agent mentions
- add duplicate suppression and turn counters
- add room-level dialogue policy persistence

### Phase 2C

- experiment with model-assisted next-speaker recommendation
- keep local policy as the final authority

## Verification Plan

Before rollout, add tests for:

- max-turn enforcement
- no self-loop when disabled
- no duplicate consecutive generated content
- mentioned agents still reply first
- message order remains human message first, generated turns after
- dialogue stops cleanly when provider errors occur

## Decision

Phase 2 should build on the migrated local orchestration model, not replace it with framework-native autonomous agents.

`langchaingo` should remain the model, prompt, and structured-output layer. Room policy, turn scheduling, and stop rules should remain application code.
