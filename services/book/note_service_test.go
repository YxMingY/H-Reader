package book

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNotesFollowBookContents(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	book := filepath.Join(dir, "original.pdf")
	if err := os.WriteFile(book, []byte("note test document"), 0600); err != nil {
		t.Fatal(err)
	}
	service := &NoteService{}
	initial, err := service.GetNotes(book)
	if err != nil || len(initial) != 0 {
		t.Fatalf("expected an empty notebook, got %+v, %v", initial, err)
	}
	note := Note{ID: "note-1", PageNum: 2, X: 0.2, Y: 0.6, Text: "初稿", Quote: "PDF excerpt"}
	saved, err := service.SaveNote(book, note)
	if err != nil || saved.CreatedAt == 0 || saved.UpdatedAt != saved.CreatedAt {
		t.Fatalf("save failed: %+v, %v", saved, err)
	}
	bookMoved := filepath.Join(dir, "renamed.pdf")
	if err := os.Rename(book, bookMoved); err != nil {
		t.Fatal(err)
	}
	reopened := &NoteService{}
	notes, err := reopened.GetNotes(bookMoved)
	if err != nil || len(notes) != 1 || notes[0] != saved {
		t.Fatalf("reloaded note mismatch: %+v, %v", notes, err)
	}

	saved.Text = "修改后的笔记"
	updated, err := reopened.SaveNote(bookMoved, saved)
	if err != nil || updated.CreatedAt != saved.CreatedAt || updated.Text != saved.Text {
		t.Fatalf("update failed: %+v, %v", updated, err)
	}
	if err := reopened.RemoveNote(bookMoved, updated.ID); err != nil {
		t.Fatal(err)
	}
	notes, err = service.GetNotes(bookMoved)
	if err != nil || len(notes) != 0 {
		t.Fatalf("expected the note to be deleted, got %+v, %v", notes, err)
	}
}
