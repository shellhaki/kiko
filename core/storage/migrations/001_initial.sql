CREATE TABLE providers (
    id TEXT PRIMARY KEY NOT NULL,
    format TEXT NOT NULL,
    base_url TEXT NOT NULL,
    model TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1))
);
CREATE UNIQUE INDEX providers_default ON providers(is_default) WHERE is_default = 1;
INSERT INTO providers VALUES ('muse', 'openai-chat', 'https://api.meta.ai/v1', 'muse-spark-1.3', 1);

CREATE TABLE projects (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description TEXT,
    created_at INTEGER NOT NULL
);
CREATE TABLE chats (
    id TEXT PRIMARY KEY NOT NULL,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    created_at INTEGER NOT NULL
);
CREATE INDEX chats_project ON chats(project_id, created_at);
CREATE INDEX chats_provider ON chats(provider_id);

CREATE TABLE messages (
    id TEXT PRIMARY KEY NOT NULL,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content TEXT,
    tool_calls TEXT CHECK (tool_calls IS NULL OR json_valid(tool_calls)),
    created_at INTEGER NOT NULL
);
CREATE INDEX messages_chat ON messages(chat_id, created_at);

CREATE TABLE documents (
    id TEXT PRIMARY KEY NOT NULL,
    chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    rev INTEGER NOT NULL DEFAULT 0 CHECK (rev >= 0),
    updated_at INTEGER NOT NULL
);
CREATE INDEX documents_chat ON documents(chat_id, updated_at);

CREATE TABLE blocks (
    id TEXT NOT NULL,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    parent_id TEXT,
    position REAL NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('heading', 'paragraph', 'list', 'listItem', 'table', 'row', 'cell', 'code', 'embed', 'pending')),
    attrs TEXT CHECK (attrs IS NULL OR json_valid(attrs)),
    content TEXT CHECK (content IS NULL OR json_valid(content)),
    h TEXT NOT NULL CHECK (length(h) = 8 AND h NOT GLOB '*[^0-9a-f]*'),
    born_rev INTEGER NOT NULL CHECK (born_rev >= 1),
    changed_rev INTEGER NOT NULL CHECK (changed_rev >= born_rev),
    gone_rev INTEGER CHECK (gone_rev IS NULL OR gone_rev >= changed_rev),
    PRIMARY KEY (document_id, id),
    FOREIGN KEY (document_id, parent_id) REFERENCES blocks(document_id, id) ON DELETE CASCADE
);
CREATE INDEX blocks_position ON blocks(document_id, parent_id, position);
CREATE INDEX blocks_changed ON blocks(document_id, changed_rev);
CREATE INDEX blocks_gone ON blocks(document_id, gone_rev);

CREATE TABLE revisions (
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    rev INTEGER NOT NULL CHECK (rev >= 1),
    author TEXT NOT NULL CHECK (author IN ('user', 'agent')),
    ops TEXT NOT NULL CHECK (json_valid(ops)),
    created_at INTEGER NOT NULL,
    PRIMARY KEY (document_id, rev)
);

CREATE TABLE nodes (
    id TEXT PRIMARY KEY NOT NULL,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    code TEXT NOT NULL,
    draft TEXT,
    pub INTEGER NOT NULL DEFAULT 1 CHECK (pub IN (0, 1))
);
CREATE INDEX nodes_document ON nodes(document_id);

CREATE TABLE comments (
    id TEXT PRIMARY KEY NOT NULL,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    parent_id TEXT,
    anchor_block TEXT,
    anchor_start INTEGER,
    anchor_end INTEGER,
    body TEXT NOT NULL,
    suggested_replies TEXT CHECK (suggested_replies IS NULL OR json_valid(suggested_replies)),
    author TEXT NOT NULL CHECK (author IN ('user', 'agent')),
    resolved INTEGER NOT NULL DEFAULT 0 CHECK (resolved IN (0, 1)),
    created_at INTEGER NOT NULL,
    UNIQUE (document_id, id),
    FOREIGN KEY (document_id, parent_id) REFERENCES comments(document_id, id) ON DELETE CASCADE,
    FOREIGN KEY (document_id, anchor_block) REFERENCES blocks(document_id, id) ON DELETE CASCADE,
    CHECK ((anchor_block IS NULL AND anchor_start IS NULL AND anchor_end IS NULL) OR
           (anchor_block IS NOT NULL AND anchor_start IS NOT NULL AND anchor_end IS NOT NULL AND anchor_start >= 0 AND anchor_end > anchor_start))
);
CREATE INDEX comments_parent ON comments(document_id, parent_id);
CREATE INDEX comments_anchor ON comments(document_id, anchor_block);

CREATE TABLE app_settings (
    key TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL CHECK (json_valid(value))
);
CREATE TABLE chat_view_state (
    chat_id TEXT PRIMARY KEY NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    document_id TEXT REFERENCES documents(id) ON DELETE SET NULL,
    state TEXT NOT NULL CHECK (json_valid(state))
);
CREATE INDEX chat_view_document ON chat_view_state(document_id);

CREATE TRIGGER chat_view_owner_insert BEFORE INSERT ON chat_view_state
WHEN NEW.document_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM documents WHERE id = NEW.document_id AND chat_id = NEW.chat_id
)
BEGIN SELECT RAISE(ABORT, 'document belongs to a different chat'); END;
CREATE TRIGGER chat_view_owner_update BEFORE UPDATE ON chat_view_state
WHEN NEW.document_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM documents WHERE id = NEW.document_id AND chat_id = NEW.chat_id
)
BEGIN SELECT RAISE(ABORT, 'document belongs to a different chat'); END;
CREATE TRIGGER document_view_owner_update BEFORE UPDATE OF chat_id ON documents
WHEN EXISTS (
    SELECT 1 FROM chat_view_state WHERE document_id = OLD.id AND chat_id != NEW.chat_id
)
BEGIN SELECT RAISE(ABORT, 'document is open in its owning chat'); END;

CREATE TABLE tool_steps (
    id TEXT PRIMARY KEY NOT NULL,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    tool TEXT NOT NULL,
    label TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'ok', 'refused', 'error', 'cancelled')),
    detail TEXT CHECK (detail IS NULL OR json_valid(detail)),
    elapsed_ms INTEGER CHECK (elapsed_ms IS NULL OR elapsed_ms >= 0),
    created_at INTEGER NOT NULL
);
CREATE INDEX tool_steps_message ON tool_steps(message_id, created_at);

CREATE TABLE workspace_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    chat_id TEXT REFERENCES chats(id) ON DELETE SET NULL
);
INSERT INTO workspace_state(id) VALUES (1);
