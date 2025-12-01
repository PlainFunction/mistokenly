-- Add updated_at column to pii_tokens table
-- This column tracks when a token was last modified

ALTER TABLE pii_tokens 
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Create index for updated_at for efficient queries
CREATE INDEX IF NOT EXISTS idx_pii_tokens_updated_at ON pii_tokens(updated_at DESC);

-- Update existing rows to have updated_at = created_at
UPDATE pii_tokens SET updated_at = created_at WHERE updated_at IS NULL;

-- Make updated_at NOT NULL after setting values
ALTER TABLE pii_tokens ALTER COLUMN updated_at SET NOT NULL;

COMMENT ON COLUMN pii_tokens.updated_at IS 'Timestamp of last update to this token record';
