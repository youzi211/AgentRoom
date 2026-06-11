-- AgentRoom initial schema

CREATE TABLE agents (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  mention VARCHAR(128) NOT NULL,
  role VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  system_prompt TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_agents_mention (mention)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rooms (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  archived_at DATETIME(6) NULL,
  KEY idx_rooms_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE room_agents (
  room_id VARCHAR(64) NOT NULL,
  agent_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  mention VARCHAR(128) NOT NULL,
  role VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  system_prompt TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (room_id, agent_id),
  KEY idx_room_agents_mention (room_id, mention),
  CONSTRAINT fk_room_agents_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE participants (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  guest_key VARCHAR(128) NULL,
  joined_at DATETIME(6) NOT NULL,
  last_seen_at DATETIME(6) NOT NULL,
  left_at DATETIME(6) NULL,
  KEY idx_participants_room_active (room_id, left_at),
  KEY idx_participants_guest (guest_key),
  CONSTRAINT fk_participants_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE messages (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  sender_id VARCHAR(64) NOT NULL,
  sender_name VARCHAR(128) NOT NULL,
  sender_type VARCHAR(32) NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_messages_room_created (room_id, created_at, id),
  CONSTRAINT fk_messages_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE agent_runs (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  agent_id VARCHAR(64) NOT NULL,
  trigger_message_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  error TEXT NULL,
  started_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_agent_runs_room (room_id, started_at),
  KEY idx_agent_runs_trigger (trigger_message_id),
  CONSTRAINT fk_agent_runs_room FOREIGN KEY (room_id) REFERENCES rooms(id),
  CONSTRAINT fk_agent_runs_trigger FOREIGN KEY (trigger_message_id) REFERENCES messages(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
