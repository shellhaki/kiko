CREATE TABLE artifacts (
 id TEXT PRIMARY KEY NOT NULL,
 chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('sheet','deck')),
 title TEXT NOT NULL,
 rev INTEGER NOT NULL CHECK(rev>=1),
 payload TEXT NOT NULL CHECK(json_valid(payload)),
 draft TEXT CHECK(draft IS NULL OR json_valid(draft)),
 updated_at INTEGER NOT NULL
);
CREATE INDEX artifacts_chat ON artifacts(chat_id,updated_at);
CREATE TABLE artifact_revisions (
 artifact_id TEXT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
 rev INTEGER NOT NULL,
 author TEXT NOT NULL CHECK(author IN ('user','agent')),
 payload TEXT NOT NULL CHECK(json_valid(payload)),
 created_at INTEGER NOT NULL,
 PRIMARY KEY(artifact_id,rev)
);
CREATE TRIGGER artifact_view_cleanup AFTER DELETE ON artifacts BEGIN UPDATE chat_view_state SET state=json_remove(state,'$.artifactId') WHERE json_extract(state,'$.artifactId')=OLD.id; END;
