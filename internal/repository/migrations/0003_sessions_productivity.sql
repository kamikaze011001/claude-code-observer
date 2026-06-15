-- 0003_sessions_productivity.sql — per-session productivity totals derived from
-- Claude Code metric datapoints (lines_of_code.count, commit.count,
-- pull_request.count, active_time.total[type=user], code_edit_tool.decision).
-- These counters arrive with DELTA temporality, so the metric rollup accumulates
-- them additively (col = col + excluded.col). See docs/CLAUDE-CODE-OTEL.md.

ALTER TABLE sessions ADD COLUMN lines_added     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN lines_removed   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN commits         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN pull_requests   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN active_seconds  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_accepted  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_rejected  INTEGER NOT NULL DEFAULT 0;
