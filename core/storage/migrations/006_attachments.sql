CREATE TABLE attachments (
 id TEXT PRIMARY KEY NOT NULL,
 chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 mime TEXT NOT NULL,
 size INTEGER NOT NULL CHECK(size>0),
 pages TEXT NOT NULL CHECK(json_valid(pages)),
 data BLOB NOT NULL,
 thumbnail BLOB,
 created_at INTEGER NOT NULL
);
CREATE INDEX attachments_chat ON attachments(chat_id,created_at);
CREATE VIRTUAL TABLE attachment_text USING fts5(attachment_id UNINDEXED,chat_id UNINDEXED,page UNINDEXED,name UNINDEXED,content,tokenize='unicode61');
CREATE TRIGGER attachments_cleanup AFTER DELETE ON attachments BEGIN DELETE FROM attachment_text WHERE attachment_id=OLD.id; END;
