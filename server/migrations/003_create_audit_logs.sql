-- Schema for audit logging
-- Stores audit logs for compliance and security monitoring

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    audit_id VARCHAR(64) UNIQUE NOT NULL,
    reference_hash VARCHAR(64),
    operation VARCHAR(50) NOT NULL, -- "tokenize", "detokenize", etc.
    requesting_service VARCHAR(255) NOT NULL,
    requesting_user VARCHAR(255),
    purpose VARCHAR(255),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    client_ip INET,
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Indexes for performance
    CONSTRAINT valid_operation CHECK (operation IN ('tokenize', 'detokenize', 'access', 'admin'))
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_audit_id ON audit_logs(audit_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_reference_hash ON audit_logs(reference_hash);
CREATE INDEX IF NOT EXISTS idx_audit_logs_operation ON audit_logs(operation);
CREATE INDEX IF NOT EXISTS idx_audit_logs_requesting_service ON audit_logs(requesting_service);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_metadata ON audit_logs USING gin(metadata);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_logs_ref_op ON audit_logs(reference_hash, operation);
CREATE INDEX IF NOT EXISTS idx_audit_logs_service_time ON audit_logs(requesting_service, timestamp DESC);

-- Automatic cleanup of old audit logs (keep last 7 years for compliance)
-- Note: Adjust retention period based on your compliance requirements
CREATE OR REPLACE FUNCTION cleanup_old_audit_logs() RETURNS void AS $$
BEGIN
    DELETE FROM audit_logs WHERE timestamp < NOW() - INTERVAL '7 years';
END;
$$ LANGUAGE plpgsql;

-- Optional: Create a scheduled job to run cleanup monthly (requires pg_cron extension)
-- SELECT cron.schedule('cleanup-old-audit-logs', '0 2 1 * *', 'SELECT cleanup_old_audit_logs()');

COMMENT ON TABLE audit_logs IS 'Audit logs for PII tokenization operations and access tracking';
COMMENT ON COLUMN audit_logs.audit_id IS 'Unique identifier for each audit event';
COMMENT ON COLUMN audit_logs.reference_hash IS 'Reference to the PII token (if applicable)';
COMMENT ON COLUMN audit_logs.operation IS 'Type of operation performed';
COMMENT ON COLUMN audit_logs.metadata IS 'Additional context in JSON format';