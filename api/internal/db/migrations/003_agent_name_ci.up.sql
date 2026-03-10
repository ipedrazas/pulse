-- Remove case-duplicate agents, keeping the most recently seen one.
-- For each group of agents with the same LOWER(name), delete all but
-- the one with the latest last_seen timestamp.
DELETE FROM agents a
USING agents b
WHERE LOWER(a.name) = LOWER(b.name)
  AND a.name <> b.name
  AND (a.last_seen < b.last_seen OR (a.last_seen = b.last_seen AND a.name > b.name));

-- Make agent name lookups case-insensitive via a unique lower index.
-- This prevents duplicates like "Pulse" and "pulse".
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_name_lower ON agents (LOWER(name));
