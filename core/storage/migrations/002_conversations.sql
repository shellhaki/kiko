ALTER TABLE messages ADD COLUMN ordinal INTEGER;
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY chat_id ORDER BY created_at, rowid) AS position FROM messages
)
UPDATE messages SET ordinal = (SELECT position FROM ranked WHERE ranked.id = messages.id);
CREATE UNIQUE INDEX messages_ordinal ON messages(chat_id, ordinal);

CREATE TABLE chat_runs (
    id TEXT PRIMARY KEY NOT NULL,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_message_id TEXT NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    assistant_message_id TEXT NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running','complete','stopped','error','interrupted','truncated','filtered')),
    error TEXT NOT NULL DEFAULT '',
    error_kind TEXT NOT NULL DEFAULT '',
    attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0),
    sequence INTEGER NOT NULL DEFAULT 1 CHECK (sequence > 0),
    previous_attempts TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(previous_attempts)),
    usage TEXT CHECK (usage IS NULL OR json_valid(usage)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX chat_runs_active ON chat_runs(chat_id) WHERE status='running';
CREATE TABLE chat_requests (
    id TEXT PRIMARY KEY NOT NULL,
    run_id TEXT NOT NULL REFERENCES chat_runs(id) ON DELETE CASCADE,
    attempt INTEGER NOT NULL
);
CREATE INDEX chat_requests_run ON chat_requests(run_id);
CREATE TABLE drafts (
    scope TEXT PRIMARY KEY NOT NULL,
    chat_id TEXT REFERENCES chats(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK ((scope='home' AND chat_id IS NULL AND project_id IS NULL) OR
           (chat_id IS NOT NULL AND project_id IS NULL AND scope='chat:'||chat_id) OR
           (project_id IS NOT NULL AND chat_id IS NULL AND scope='project:'||project_id))
);
CREATE INDEX drafts_chat ON drafts(chat_id);
CREATE INDEX drafts_project ON drafts(project_id);
