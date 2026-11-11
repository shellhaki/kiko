CREATE TABLE project_roots (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    path TEXT NOT NULL UNIQUE
);
CREATE TABLE chat_permissions (
    chat_id TEXT PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    mode TEXT NOT NULL DEFAULT 'ask' CHECK (mode IN ('ask','full'))
);
CREATE TABLE file_permissions (
    id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    run_id TEXT NOT NULL REFERENCES chat_runs(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','denied','cancelled'))
);
CREATE TABLE workspace_files (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    source_id TEXT NOT NULL DEFAULT '',
    hash TEXT NOT NULL,
    PRIMARY KEY (project_id,path)
);
CREATE UNIQUE INDEX workspace_file_source ON workspace_files(source_id) WHERE source_id!='';
CREATE TABLE workspace_sync_errors (
    source_id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    message TEXT NOT NULL
);
