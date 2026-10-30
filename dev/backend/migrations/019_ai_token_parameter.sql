ALTER TABLE ai_provider_config ADD COLUMN token_parameter text NOT NULL DEFAULT 'max_tokens' CHECK(token_parameter IN('max_tokens','max_completion_tokens'));
INSERT INTO schema_migration(version) VALUES(19);
