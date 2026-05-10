-- 0001_initial.sql — full v1 schema for claude-code-observer.
-- See docs/DATA-MODELS.md for column-level documentation.

CREATE TABLE events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,
    session_id  TEXT NOT NULL,
    prompt_id   TEXT,
    event_name  TEXT NOT NULL,
    attrs       TEXT NOT NULL
);
CREATE INDEX idx_events_session_ts ON events(session_id, ts);
CREATE INDEX idx_events_prompt     ON events(prompt_id);
CREATE INDEX idx_events_name_ts    ON events(event_name, ts);

CREATE TABLE sessions (
    session_id            TEXT PRIMARY KEY,
    project_name          TEXT,
    project_cwd           TEXT,
    started_at            INTEGER NOT NULL,
    last_seen_at          INTEGER NOT NULL,
    ended_at              INTEGER,
    app_version           TEXT,
    os_type               TEXT,
    user_id               TEXT,
    cost_usd              REAL    NOT NULL DEFAULT 0,
    input_tokens          INTEGER NOT NULL DEFAULT 0,
    output_tokens         INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    api_requests          INTEGER NOT NULL DEFAULT 0,
    api_errors            INTEGER NOT NULL DEFAULT 0,
    subagent_requests     INTEGER NOT NULL DEFAULT 0,
    auxiliary_requests    INTEGER NOT NULL DEFAULT 0,
    tool_calls            INTEGER NOT NULL DEFAULT 0,
    tool_denied           INTEGER NOT NULL DEFAULT 0,
    prompts               INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_sessions_started         ON sessions(started_at DESC);
CREATE INDEX idx_sessions_project_started ON sessions(project_name, started_at DESC);

CREATE TABLE prompts (
    prompt_id             TEXT PRIMARY KEY,
    session_id            TEXT NOT NULL,
    started_at            INTEGER NOT NULL,
    ended_at              INTEGER,
    prompt_length         INTEGER,
    command_name          TEXT,
    command_source        TEXT,
    cost_usd              REAL    NOT NULL DEFAULT 0,
    input_tokens          INTEGER NOT NULL DEFAULT 0,
    output_tokens         INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    api_requests          INTEGER NOT NULL DEFAULT 0,
    subagent_requests     INTEGER NOT NULL DEFAULT 0,
    tool_calls            INTEGER NOT NULL DEFAULT 0,
    had_error             INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);
CREATE INDEX idx_prompts_session_started ON prompts(session_id, started_at);

CREATE TABLE metric_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,
    session_id  TEXT,
    metric_name TEXT NOT NULL,
    value       REAL NOT NULL,
    attrs       TEXT NOT NULL
);
