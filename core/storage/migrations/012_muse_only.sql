CREATE TABLE chat_models (
    chat_id TEXT PRIMARY KEY NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    model TEXT NOT NULL
);
INSERT INTO chat_models SELECT id, (SELECT model FROM providers WHERE id='muse') FROM chats;
CREATE TRIGGER chats_default_model AFTER INSERT ON chats BEGIN
    INSERT INTO chat_models VALUES (new.id, (SELECT model FROM providers WHERE id='muse'));
END;
UPDATE chats SET provider_id='muse';
DELETE FROM providers WHERE id!='muse';
DELETE FROM app_settings WHERE key='image-models';
