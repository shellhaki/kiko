CREATE TABLE workspace_text_drafts (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    base_hash TEXT NOT NULL,
    content TEXT NOT NULL,
    PRIMARY KEY (project_id,path)
);
