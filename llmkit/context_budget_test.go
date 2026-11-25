package llmkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestBudgetCompactionAndRollback(t *testing.T) {
	var summaries int
	var fail bool
	var last []openai.ChatCompletionMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		reply := "answer " + strings.Repeat("a", 380)
		if strings.HasPrefix(req.Messages[0].Content, "Summarize") {
			summaries++
			reply = "summary of old turns"
		} else {
			last = req.Messages
			if fail {
				http.Error(w, "test failure", 500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleAssistant, Content: reply,
			}}},
		})
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "test", Model: "test", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	conv := client.NewTracedConversation(nil)
	conv.SetMaxTokens(400)
	conv.SetContextBudget(3600, 4)
	for i := 0; i < 5; i++ {
		conv.AddText(fmt.Sprintf("question %d %s", i, strings.Repeat("q", 380)))
		if _, err := conv.Send(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if summaries != 0 {
		t.Fatalf("short conversation made %d summary requests", summaries)
	}
	conv.AddText(strings.Repeat("n", 2300))
	fail = true
	if _, err := conv.Send(context.Background()); err == nil || conv.covered != 0 || len(conv.History) != 10 {
		t.Fatalf("failed send changed state: covered=%d history=%d err=%v", conv.covered, len(conv.History), err)
	}
	fail = false
	if _, err := conv.Send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if summaries != 2 || conv.covered != 2 || len(last) != 10 {
		t.Fatalf("unexpected compaction: calls=%d covered=%d request=%d", summaries, conv.covered, len(last))
	}
	for i := 1; i < 5; i++ {
		if last[2*i-1].Content != conv.History[2*i].Content ||
			last[2*i].Content != conv.History[2*i+1].Content {
			t.Fatalf("turn %d was not retained verbatim", i)
		}
	}
	saved, err := conv.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	restored := client.NewTracedConversation(nil)
	if err := restored.ImportJSON(saved); err != nil {
		t.Fatal(err)
	}
	restored.SetContextBudget(3600, 4)
	if restored.covered != 2 || restored.GetTrace() != conv.GetTrace() {
		t.Fatal("restored summary did not match")
	}
	if restored.RollbackMessages(6) != 6 || restored.covered != 0 || restored.GetTrace() != EmptyTraceState {
		t.Fatal("rollback did not invalidate the summary")
	}
	changed, err := conv.CompactHistory(context.Background())
	if err != nil || !changed || conv.covered != 4 || len(conv.History) != 12 {
		t.Fatalf("manual compaction: changed=%v covered=%d history=%d err=%v", changed, conv.covered, len(conv.History), err)
	}
	if changed, err := conv.CompactHistory(context.Background()); changed || err != nil {
		t.Fatalf("compaction with only four eligible recent turns: %v %v", changed, err)
	}
	summaryCalls := summaries
	window := client.NewWindowConversation()
	window.AddText("first window turn")
	if _, err := window.Chat(context.Background()); err != nil {
		t.Fatal(err)
	}
	window.SetWindowSize(0)
	window.AddText("question with no history")
	if _, err := window.Chat(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(last) != 2 || strings.Contains(last[0].Content, "TRACE_STATE") || summaries != summaryCalls {
		t.Fatalf("zero window sent unexpected context: %d messages, %d summaries", len(last), summaries)
	}
	saved, err = window.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	restoredWindow := client.NewWindowConversation()
	legacyWindow := strings.Replace(saved, "{", `{"trace":"legacy empty trace",`, 1)
	if err := restoredWindow.ImportJSON(legacyWindow); err != nil {
		t.Fatal(err)
	}
	if restoredWindow.windowSize != 0 {
		t.Fatal("zero window did not survive reload")
	}
	restoredWindow.SetWindowSize(2)
	restoredWindow.AddText("question with two rounds")
	if _, err := restoredWindow.Chat(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(last) != 6 || last[3].Content != "question with no history" || summaries != summaryCalls {
		t.Fatalf("resized window did not recover recent history: %d messages, %d summaries", len(last), summaries)
	}
}
