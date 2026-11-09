package storage

import (
	"context"
	"testing"

	"kiko/core/attachment"
)

func TestAttachmentsAreScopedSearchableAndCascaded(t *testing.T) {
	s, chat := documentStore(t)
	ctx := context.Background()
	data := []byte("A searchable source with reliable evidence.")
	parsed, err := attachment.Extract("notes.md", data)
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.AddAttachment(ctx, chat, "notes.md", data, parsed)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := s.SearchAttachments(ctx, chat, "reliable evidence")
	if err != nil || len(hits) != 1 || hits[0].AttachmentID != file.ID {
		t.Fatal(err, hits)
	}
	if _, err = s.ReadAttachment(ctx, "other", file.ID, 1); err == nil {
		t.Fatal("cross-chat attachment read accepted")
	}
	if err = s.Delete(ctx, "chat", chat); err != nil {
		t.Fatal(err)
	}
	var remaining int
	s.DB().QueryRow("SELECT count(*) FROM attachment_text").Scan(&remaining)
	if remaining != 0 {
		t.Fatal("attachment search entries leaked after deletion")
	}
}
