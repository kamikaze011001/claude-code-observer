package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

const sessionMetadataUpsert = `INSERT INTO sessions (
    session_id, started_at, last_seen_at,
    project_name, project_cwd, app_version, os_type, user_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    started_at   = MIN(started_at, excluded.started_at),
    last_seen_at = MAX(last_seen_at, excluded.last_seen_at),
    project_name = COALESCE(project_name, excluded.project_name),
    project_cwd  = COALESCE(project_cwd,  excluded.project_cwd),
    app_version  = COALESCE(app_version,  excluded.app_version),
    os_type      = COALESCE(os_type,      excluded.os_type),
    user_id      = COALESCE(user_id,      excluded.user_id)`

const sessionEndUpsert = `INSERT INTO sessions (
    session_id, started_at, last_seen_at, ended_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    last_seen_at = MAX(last_seen_at, excluded.last_seen_at),
    ended_at     = excluded.ended_at`

func applySessionStart(ev domain.Event) []Op {
	return []Op{{
		Query: sessionMetadataUpsert,
		Args: []any{
			ev.SessionID,
			ev.TS, ev.TS,
			attrString(ev.Attrs, "project.name"),
			attrString(ev.Attrs, "project.cwd"),
			attrString(ev.Attrs, "app.version"),
			attrString(ev.Attrs, "os.type"),
			attrString(ev.Attrs, "user.id"),
		},
	}}
}

func applySessionEnd(ev domain.Event) []Op {
	return []Op{{
		Query: sessionEndUpsert,
		Args:  []any{ev.SessionID, ev.TS, ev.TS, ev.TS},
	}}
}

func init() {
	updaters[domain.EventSessionStart] = applySessionStart
	updaters[domain.EventSessionEnd] = applySessionEnd
}
