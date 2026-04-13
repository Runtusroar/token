-- Composite index for efficient per-user request log queries sorted by time.
CREATE INDEX IF NOT EXISTS idx_request_logs_user_created ON request_logs (user_id, created_at DESC);
