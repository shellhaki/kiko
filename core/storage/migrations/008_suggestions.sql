CREATE TABLE suggestions (
 id TEXT PRIMARY KEY NOT NULL,
 document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
 base_rev INTEGER NOT NULL,
 ops TEXT NOT NULL CHECK(json_valid(ops)),
 before_markdown TEXT NOT NULL,
 after_markdown TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','rejected')),
 created_at INTEGER NOT NULL
);
CREATE INDEX suggestions_document ON suggestions(document_id,status,created_at);
