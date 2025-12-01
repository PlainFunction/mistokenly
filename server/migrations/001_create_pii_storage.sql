-- Schema for persistent PII token storage
-- This database is separate from the PGMQ database for network segmentation

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- PII tokens table with encrypted data
CREATE TABLE IF NOT EXISTS pii_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reference_hash VARCHAR(64) UNIQUE NOT NULL,
    encrypted_data BYTEA NOT NULL,
    iv BYTEA NOT NULL,  -- Initialization Vector
    data_type VARCHAR(50) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Indexes for common queries
    CONSTRAINT valid_data_type CHECK (data_type IN ('email', 'ssn', 'phone', 'credit_card', 'name', 'address'))
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_pii_tokens_reference_hash ON pii_tokens(reference_hash);
CREATE INDEX IF NOT EXISTS idx_pii_tokens_organization_id ON pii_tokens(organization_id);
CREATE INDEX IF NOT EXISTS idx_pii_tokens_client_id ON pii_tokens(client_id);
CREATE INDEX IF NOT EXISTS idx_pii_tokens_expires_at ON pii_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_pii_tokens_created_at ON pii_tokens(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pii_tokens_metadata ON pii_tokens USING gin(metadata);

-- Composite index for organization + reference lookup (most common query)
CREATE INDEX IF NOT EXISTS idx_pii_tokens_org_ref ON pii_tokens(organization_id, reference_hash);

-- Table for tracking TEKs (Tenant Encryption Keys) per organization
CREATE TABLE IF NOT EXISTS organization_teks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id VARCHAR(255) UNIQUE NOT NULL,
    encrypted_tek BYTEA NOT NULL,  -- TEK encrypted by KEK
    org_key_hash VARCHAR(64) NOT NULL,  -- SHA-256 hash of organization key
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMP WITH TIME ZONE,
    version INTEGER NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_organization_teks_org_id ON organization_teks(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_teks_active ON organization_teks(organization_id, is_active) WHERE is_active = true;

-- Automatic cleanup of expired tokens (run periodically via cron or pg_cron)
CREATE OR REPLACE FUNCTION cleanup_expired_tokens() RETURNS void AS $$
BEGIN
    DELETE FROM pii_tokens WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Optional: Create a scheduled job to run cleanup daily (requires pg_cron extension)
-- SELECT cron.schedule('cleanup-expired-pii-tokens', '0 2 * * *', 'SELECT cleanup_expired_tokens()');

-- Statistics view for monitoring
CREATE OR REPLACE VIEW pii_token_stats AS
SELECT
    COUNT(*) as total_tokens,
    COUNT(*) FILTER (WHERE expires_at > NOW()) as active_tokens,
    COUNT(*) FILTER (WHERE expires_at <= NOW()) as expired_tokens,
    COUNT(DISTINCT organization_id) as unique_organizations,
    COUNT(DISTINCT client_id) as unique_clients,
    data_type,
    COUNT(*) as count_by_type
FROM pii_tokens
GROUP BY data_type;

COMMENT ON TABLE pii_tokens IS 'Persistent storage for tokenized PII data with envelope encryption';
COMMENT ON TABLE organization_teks IS 'Tenant Encryption Keys per organization, encrypted by master KEK';
COMMENT ON COLUMN pii_tokens.encrypted_data IS 'PII data encrypted with organization-specific key derived via HKDF';
COMMENT ON COLUMN pii_tokens.iv IS 'AES-GCM initialization vector, stored separately from ciphertext';
COMMENT ON COLUMN pii_tokens.metadata IS 'Additional metadata in JSON format';
