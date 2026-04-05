-- Backfill message_type in conversation metadata for existing assistant messages.
-- This ensures the frontend can strictly route by message_type without fallback guessing.
--
-- Rules:
--   - metadata has "status" field (clarify/confirmed) → message_type = "analysis"
--   - metadata has "tasks" array (plan output) → message_type = "plan"
--   - metadata has "workflow_id" (system msg after approve) → message_type = "system"

-- Analysis messages (status = clarify or confirmed)
UPDATE engine.conversations
SET metadata = metadata || '{"message_type": "analysis"}'::jsonb
WHERE role = 'assistant'
  AND metadata IS NOT NULL
  AND metadata ? 'status'
  AND NOT (metadata ? 'message_type');

-- Plan messages (has tasks array and risk_level)
UPDATE engine.conversations
SET metadata = metadata || '{"message_type": "plan"}'::jsonb
WHERE role = 'assistant'
  AND metadata IS NOT NULL
  AND metadata ? 'tasks'
  AND metadata ? 'risk_level'
  AND NOT (metadata ? 'message_type');

-- System messages (has workflow_id)
UPDATE engine.conversations
SET metadata = metadata || '{"message_type": "system"}'::jsonb
WHERE role = 'system'
  AND metadata IS NOT NULL
  AND metadata ? 'workflow_id'
  AND NOT (metadata ? 'message_type');
