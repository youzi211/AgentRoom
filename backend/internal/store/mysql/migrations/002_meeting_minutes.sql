-- AgentRoom meeting minutes (versioned, persisted)
-- Reference only: runtime schema is applied by GORM AutoMigrate (see store.go Migrate()).
-- The rooms.status and rooms.archived_at columns already exist in 001_initial_schema.sql
-- and are reused for room archive/close; no new room columns are required here.

CREATE TABLE meeting_minutes (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  version INT NOT NULL,
  content MEDIUMTEXT NOT NULL,
  source VARCHAR(16) NOT NULL,            -- 'ai' | 'manual'
  created_by VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  KEY idx_minutes_room (room_id, version),
  CONSTRAINT fk_minutes_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
