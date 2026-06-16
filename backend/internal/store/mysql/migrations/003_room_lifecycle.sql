-- AgentRoom meeting lifecycle reference schema.
-- Runtime schema changes are still applied through GORM AutoMigrate; this file
-- documents the intended room columns and indexes for review / manual rollout.

ALTER TABLE rooms
  ADD COLUMN owner_participant_id VARCHAR(64) NULL AFTER status,
  ADD COLUMN closed_at DATETIME(6) NULL AFTER cooldown_ms,
  ADD COLUMN closed_reason VARCHAR(32) NOT NULL DEFAULT '' AFTER closed_at,
  ADD COLUMN auto_close_deadline_at DATETIME(6) NULL AFTER closed_reason;

CREATE INDEX idx_rooms_status_created ON rooms (status, created_at, id);
