-- 0002_sessions_last_seen_index.sql — index for ordering/paginating sessions by
-- most recent activity (last_seen_at), used by the dashboard's recent-sessions
-- panel and the Sessions list view.

CREATE INDEX idx_sessions_last_seen ON sessions(last_seen_at DESC);
