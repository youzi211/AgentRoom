ALTER TABLE messages
  ADD COLUMN knowledge_sources_json TEXT NULL AFTER parent_message_id;
