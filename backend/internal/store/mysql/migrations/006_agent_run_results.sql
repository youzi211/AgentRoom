-- Link one durable final Agent message to one Agent Run. Existing messages
-- remain compatible because the new reference is nullable.
ALTER TABLE messages
  ADD COLUMN agent_run_id VARCHAR(64) NULL AFTER id,
  ADD UNIQUE KEY uk_messages_agent_run_id (agent_run_id),
  ADD CONSTRAINT fk_messages_agent_run
    FOREIGN KEY (agent_run_id) REFERENCES agent_runs(id);
