ALTER TABLE rooms
  ADD COLUMN collaboration_engine VARCHAR(32) NOT NULL DEFAULT 'native' AFTER cooldown_ms,
  ADD COLUMN collaboration_trigger_mode VARCHAR(32) NOT NULL DEFAULT 'mention_only' AFTER collaboration_engine,
  ADD COLUMN collaboration_max_turns INT NOT NULL DEFAULT 3 AFTER collaboration_trigger_mode,
  ADD COLUMN collaboration_max_turns_per_agent INT NOT NULL DEFAULT 1 AFTER collaboration_max_turns,
  ADD COLUMN collaboration_allow_agent_handoff BOOLEAN NOT NULL DEFAULT TRUE AFTER collaboration_max_turns_per_agent,
  ADD COLUMN collaboration_allow_self_followup BOOLEAN NOT NULL DEFAULT FALSE AFTER collaboration_allow_agent_handoff,
  ADD COLUMN collaboration_cooldown_ms INT NOT NULL DEFAULT 0 AFTER collaboration_allow_self_followup;
