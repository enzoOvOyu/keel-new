ALTER TABLE ai_provider_config DROP CONSTRAINT ai_provider_config_protocol_check;
ALTER TABLE ai_provider_config ADD CONSTRAINT ai_provider_config_protocol_check CHECK (protocol IN ('openai-chat','openai-responses','keel-mapping'));
