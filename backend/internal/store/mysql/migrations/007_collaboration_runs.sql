CREATE TABLE collaboration_runs (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  root_message_id VARCHAR(64) NOT NULL,
  engine VARCHAR(32) NOT NULL,
  engine_version VARCHAR(64) NOT NULL DEFAULT '',
  policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
  status VARCHAR(32) NOT NULL,
  stop_reason VARCHAR(64) NOT NULL DEFAULT '',
  turn_count INT NOT NULL DEFAULT 0,
  error TEXT NULL,
  created_at DATETIME(6) NOT NULL,
  started_at DATETIME(6) NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_collaboration_runs_room (room_id, created_at),
  KEY idx_collaboration_runs_root_message (root_message_id),
  CONSTRAINT fk_collaboration_runs_room FOREIGN KEY (room_id) REFERENCES rooms(id),
  CONSTRAINT fk_collaboration_runs_root_message FOREIGN KEY (root_message_id) REFERENCES messages(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
