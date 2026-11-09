package document

import (
	"errors"
	"strings"
	"testing"
)

func pointer(s string) *string { return &s }
func testDoc(t *testing.T) Document {
	t.Helper()
	blocks, err := Parse("# Plan\n\nHello **world** and world.\n\nLast paragraph.", nil)
	if err != nil {
		t.Fatal(err)
	}
	return Document{ID: "doc", Title: "Plan", Rev: 2, Blocks: blocks}
}
func refusalCode(t *testing.T, err error, code string) {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) || r.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
func TestGuardedOperationsAreAtomic(t *testing.T) {
	doc := testDoc(t)
	b := doc.Blocks[1]
	op := Operation{Type: "replace", Target: Target{IDs: []string{b.ID}}, Markdown: pointer("Replacement")}
	_, err := Apply(doc, []Operation{op})
	refusalCode(t, err, "missing_guard")
	op.IfRev = 1
	op.IfHash = map[string]string{b.ID: b.Hash}
	_, err = Apply(doc, []Operation{op})
	refusalCode(t, err, "guard_mismatch")
	op.IfRev = 2
	after, err := Apply(doc, []Operation{op})
	if err != nil || Plain(after.Blocks[1]) != "Replacement" {
		t.Fatal(err, after)
	}
	bad := Operation{Type: "delete", Target: Target{IDs: []string{"gone"}}}
	after, err = Apply(doc, []Operation{op, bad})
	refusalCode(t, err, "block_gone")
	if withoutIDs(after.Blocks) != withoutIDs(doc.Blocks) {
		t.Fatal("partial mutation escaped")
	}
}
func TestExactTextKeepsSurroundingFormatting(t *testing.T) {
	doc := testDoc(t)
	_, err := Apply(doc, []Operation{{Type: "replace", Target: Target{Find: "world"}, Text: pointer("earth")}})
	refusalCode(t, err, "find_ambiguous")
	after, err := Apply(doc, []Operation{{Type: "replace", Target: Target{Find: "world", Nth: 1}, Text: pointer("earth")}})
	if err != nil || !strings.Contains(Markdown(after.Blocks), "**earth** and world") {
		t.Fatal(err, Markdown(after.Blocks))
	}
	if !strings.Contains(Markdown(doc.Blocks), "**world**") {
		t.Fatal("input mutated")
	}
}
func TestMoveDeleteAndPending(t *testing.T) {
	doc := testDoc(t)
	last := doc.Blocks[2]
	move := Operation{Type: "move", Target: Target{IDs: []string{last.ID}}, After: doc.Blocks[0].ID, IfRev: doc.Rev, IfHash: map[string]string{last.ID: last.Hash}}
	after, err := Apply(doc, []Operation{move})
	if err != nil || after.Blocks[1].ID != last.ID {
		t.Fatal(err, after)
	}
	move.ParentID = last.ID
	_, err = Apply(doc, []Operation{move})
	refusalCode(t, err, "block_gone")
	blocks, _ := Parse("<?kiko block p?>", map[string]Placeholder{"p": {Type: "pending", Intent: "Risks"}})
	pending := Document{Rev: 1, Blocks: blocks}
	after, err = Apply(pending, []Operation{{Type: "replace", Target: Target{IDs: []string{blocks[0].ID}}, Markdown: pointer("## Risks\n\nScope may change.")}})
	if err != nil || len(after.Blocks) != 2 {
		t.Fatal(err, after)
	}
}
