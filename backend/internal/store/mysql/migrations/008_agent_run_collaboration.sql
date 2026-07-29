ALTER TABLE agent_runs
  ADD COLUMN collaboration_run_id VARCHAR(64) NULL AFTER trigger_message_id,
  ADD COLUMN turn_index INT NULL AFTER collaboration_run_id,
  ADD COLUMN parent_message_id VARCHAR(64) NULL AFTER turn_index,
  ADD KEY idx_agent_runs_collaboration (collaboration_run_id),
  ADD KEY idx_agent_runs_parent_message (parent_message_id),
  ADD CONSTRAINT fk_agent_runs_collaboration
    FOREIGN KEY (collaboration_run_id) REFERENCES collaboration_runs(id),
  ADD CONSTRAINT fk_agent_runs_parent_message
    FOREIGN KEY (parent_message_id) REFERENCES messages(id);
