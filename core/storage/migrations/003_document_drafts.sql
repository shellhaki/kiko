CREATE TABLE document_drafts (
    document_id TEXT PRIMARY KEY NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    base_rev INTEGER NOT NULL CHECK (base_rev >= 0),
    blocks TEXT NOT NULL CHECK (json_valid(blocks)),
    updated_at INTEGER NOT NULL
);
