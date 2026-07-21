CREATE TABLE model_profiles (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  runtime_scope VARCHAR(32) NOT NULL,
  protocol VARCHAR(64) NOT NULL,
  base_url VARCHAR(1024) NOT NULL,
  model_name VARCHAR(255) NOT NULL,
  api_key_ciphertext TEXT NOT NULL,
  api_key_hint VARCHAR(32) NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  default_slot VARCHAR(32) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_model_profiles_default_slot (default_slot)
);

ALTER TABLE agents ADD COLUMN model_profile_id VARCHAR(64) NULL;
ALTER TABLE room_agents ADD COLUMN model_profile_id VARCHAR(64) NULL;
ALTER TABLE agent_runs ADD COLUMN model_profile_id VARCHAR(64) NULL;
ALTER TABLE agent_runs ADD COLUMN model_source VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN model_name VARCHAR(255) NOT NULL DEFAULT '';
