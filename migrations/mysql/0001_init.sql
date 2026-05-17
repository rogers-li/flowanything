-- Initial registry schema draft. SQLite uses the same logical model in code for local development.

CREATE TABLE IF NOT EXISTS agents (
  tenant_id VARCHAR(128) NOT NULL,
  id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  skill_ids_json JSON NOT NULL,
  default_lang VARCHAR(32) NOT NULL,
  version VARCHAR(64) NOT NULL,
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS skills (
  tenant_id VARCHAR(128) NOT NULL,
  id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  tool_ids_json JSON NOT NULL,
  knowledge_ids_json JSON NOT NULL,
  system_prompt TEXT NOT NULL,
  policy_version VARCHAR(64) NOT NULL,
  version VARCHAR(64) NOT NULL,
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS tools (
  tenant_id VARCHAR(128) NOT NULL,
  id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  implementation VARCHAR(64) NOT NULL,
  binding_json JSON NOT NULL,
  input_schema_json JSON NOT NULL,
  output_schema_json JSON NOT NULL,
  side_effect VARCHAR(64) NOT NULL,
  risk_level VARCHAR(64) NOT NULL,
  requires_confirmation BOOLEAN NOT NULL,
  timeout_ms INT NOT NULL,
  version VARCHAR(64) NOT NULL,
  PRIMARY KEY (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS connector_operations (
  tenant_id VARCHAR(128) NOT NULL,
  id VARCHAR(128) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  type VARCHAR(64) NOT NULL,
  base_url TEXT NOT NULL,
  method VARCHAR(32) NOT NULL,
  path TEXT NOT NULL,
  headers_json JSON NOT NULL,
  input_schema_json JSON NOT NULL,
  timeout_ms INT NOT NULL,
  version VARCHAR(64) NOT NULL,
  PRIMARY KEY (tenant_id, id)
);
