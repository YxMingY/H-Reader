package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hreader/llmkit"
	"hreader/services/book"

	openai "github.com/sashabaranov/go-openai"
)

func TestInlineConversationPersistsFullHistoryAndSendsWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	bookPath := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(bookPath, []byte("sample paper"), 0600); err != nil {
		t.Fatal(err)
	}

	requests := make(chan openai.ChatCompletionRequest, 8)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"示例回答"}}]}`)
	}))
	defer api.Close()

	client, err := llmkit.NewClient(llmkit.Config{Provider: llmkit.ProviderOpenAI, APIKey: "test", Model: "test-model", BaseURL: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	service := &ChatService{inline: apiRoute{client: client}}
	anchor := InlineAnchor{PageNum: 2, X: 0.25, Y: 0.5, Quote: "independent samples"}
	thread, err := service.CreateInlineConversation(bookPath, anchor)
	if err != nil {
		t.Fatal(err)
	}
	for round := 1; round <= 8; round++ {
		if _, err := service.SendInlineQuestion(context.Background(), bookPath, thread.ID, fmt.Sprintf("question %d", round)); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
	}

	for round := 1; round <= 8; round++ {
		req := <-requests
		if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, anchor.Quote) {
			t.Errorf("round %d lost the selected excerpt", round)
		}
		window := min(round-1, inlineWindowRounds)
		if got, want := len(req.Messages), 2+2*window; got != want {
			t.Fatalf("round %d: sent %d messages, want %d", round, got, want)
		}
		if got := req.Messages[1].Content; got != fmt.Sprintf("question %d", round-window) {
			t.Errorf("round %d: first retained question was %q", round, got)
		}
	}

	reopened := &ChatService{}
	markers, err := reopened.ListInlineConversations(bookPath)
	if err != nil || len(markers) != 1 || markers[0].ID != thread.ID || markers[0].PageNum != anchor.PageNum {
		t.Fatalf("marker not recovered: %+v, %v", markers, err)
	}
	saved, err := reopened.LoadInlineConversation(bookPath, thread.ID)
	if err != nil || saved.Anchor != anchor || len(saved.Messages) != 16 || saved.Messages[0].Content != "question 1" {
		t.Fatalf("full transcript not recovered: %+v, %v", saved, err)
	}
}

func TestInlineStreamSendsAndPersistsImages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	bookPath := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(bookPath, []byte("sample paper"), 0600); err != nil {
		t.Fatal(err)
	}

	requests := make(chan openai.ChatCompletionRequest, 1)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Formula $x\"}}]}\n\n")
		w.(http.Flusher).Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"^2$.\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer api.Close()

	client, err := llmkit.NewClient(llmkit.Config{Provider: llmkit.ProviderOpenAI, APIKey: "test", Model: "test-model", BaseURL: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	service := &ChatService{inline: apiRoute{client: client}}
	thread, err := service.CreateInlineConversation(bookPath, InlineAnchor{PageNum: 1, X: 0.25, Y: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	const image = "data:image/jpeg;base64,/9j/2Q=="
	var chunks []string
	_, err = service.sendInlineQuestion(context.Background(), bookPath, thread.ID, "", []string{image}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil || len(chunks) != 2 || chunks[0] != "Formula $x" || chunks[1] != "^2$." {
		t.Fatalf("unexpected streamed chunks %q, %v", chunks, err)
	}

	request := <-requests
	parts := request.Messages[len(request.Messages)-1].MultiContent
	if !request.Stream || len(parts) != 2 || parts[0].ImageURL == nil || parts[0].ImageURL.URL != image || parts[1].Text != "请解释图片" {
		t.Fatalf("unexpected streamed request: %+v", request)
	}

	saved, err := (&ChatService{}).LoadInlineConversation(bookPath, thread.ID)
	if err != nil || len(saved.Messages) != 2 || saved.Messages[0].Content != "请解释图片" ||
		len(saved.Messages[0].Attachments) != 1 || saved.Messages[0].Attachments[0] != image ||
		saved.Messages[1].Content != "Formula $x^2$." {
		t.Fatalf("unexpected saved transcript: %+v, %v", saved.Messages, err)
	}
}

func TestDeleteInlineConversationKeepsNotesEditableAfterRestart(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Chdir(dir)
	home, err := os.UserHomeDir()
	if err != nil || home != dir {
		t.Fatalf("refusing to use a non-test home: %q, %v", home, err)
	}
	bookPath := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(bookPath, []byte("sample paper"), 0600); err != nil {
		t.Fatal(err)
	}

	notes := &book.NoteService{}
	service := &ChatService{notes: notes}
	anchor := InlineAnchor{PageNum: 1, X: 0.5, Y: 0.5, Quote: "excerpt"}
	removed, err := service.CreateInlineConversation(bookPath, anchor)
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.CreateInlineConversation(bookPath, anchor)
	if err != nil {
		t.Fatal(err)
	}
	note := func(id, threadID string) book.Note {
		t.Helper()
		saved, err := notes.SaveNote(bookPath, book.Note{
			ID: id, Text: "kept note " + id, Quote: "original quote", PageNum: 1, X: 0.5, Y: 0.5,
			InlineConversationID: threadID,
		})
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	first := note("first", removed.ID)
	second := note("second", removed.ID)
	unrelated := note("unrelated", other.ID)

	path, err := service.inlinePath(bookPath, removed.ID)
	if err != nil {
		t.Fatal(err)
	}
	service.inlineRequests.Store(path, struct{}{})
	if err := service.DeleteInlineConversation(bookPath, removed.ID); err == nil {
		t.Fatal("deleting a busy conversation succeeded")
	}
	service.inlineRequests.Delete(path)

	// Simulate a deletion failure after the note file is written.
	staged := path + ".fixture"
	if err := os.Rename(path, staged); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteInlineConversation(bookPath, removed.ID); err == nil {
		t.Fatal("expected deletion to fail with a directory in place of the conversation")
	}
	preserved, err := notes.GetNotes(bookPath)
	if err != nil || len(preserved) != 3 || preserved[0] != first || preserved[1] != second || preserved[2] != unrelated {
		t.Fatalf("failed deletion modified notes: %+v, %v", preserved, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staged, path); err != nil {
		t.Fatal(err)
	}

	if err := service.DeleteInlineConversation(bookPath, removed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted conversation file should be absent: %v", err)
	}
	reopened := &ChatService{notes: &book.NoteService{}}
	markers, err := reopened.ListInlineConversations(bookPath)
	if err != nil || len(markers) != 1 || markers[0].ID != other.ID {
		t.Fatalf("deletion affected the wrong markers: %+v, %v", markers, err)
	}
	saved, err := reopened.notes.GetNotes(bookPath)
	wantFirst, wantSecond := first, second
	wantFirst.InlineConversationID, wantSecond.InlineConversationID = "", ""
	if err != nil || len(saved) != 3 || saved[0] != wantFirst || saved[1] != wantSecond || saved[2] != unrelated {
		t.Fatalf("deletion affected note text or anchors: %+v, %v", saved, err)
	}

	// A stale editing draft must not restore the removed link.
	first.Text = "edited after restart"
	edited, err := reopened.notes.SaveNote(bookPath, first)
	if err != nil || edited.Text != first.Text || edited.InlineConversationID != "" {
		t.Fatalf("could not save the original note without restoring its link: %+v, %v", edited, err)
	}
}
