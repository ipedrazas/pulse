-- Make agent name lookups case-insensitive via a unique lower index.
-- This prevents duplicates like "Pulse" and "pulse".
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_name_lower ON agents (LOWER(name));
