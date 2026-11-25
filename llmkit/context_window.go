package llmkit

import (
	"context"
	"encoding/json"
	"errors"

	openai "github.com/sashabaranov/go-openai"
)

const windowSystemPrompt = "你是一个学习辅助 AI，与用户进行正常对话。"

// WindowConversation sends recent complete turns within its configured window.
type WindowConversation struct {
	*Conversation
	windowSize int
	budget     int
}

type windowConversationSnapshot struct {
	conversationJSONSnapshot
	WindowSize *int `json:"window_size"`
}

// SetWindowSize limits requests to the latest rounds. Zero sends no history.
func (w *WindowConversation) SetWindowSize(rounds int) {
	w.windowSize = max(0, rounds)
}

// SetContextBudget sets the total estimated token limit, including outpuw.
// A non-positive limit disables token-based trimming.
func (w *WindowConversation) SetContextBudget(limit int) {
	w.budget = limit
}

func (w *WindowConversation) Chat(ctx context.Context) (string, error) {
	return w.Send(ctx)
}

func (w *WindowConversation) Send(ctx context.Context) (string, error) {
	var reply string
	err := w.sendWindowed(func(conv *Conversation) error {
		var err error
		reply, err = conv.Send(ctx)
		return err
	})
	return reply, err
}

func (w *WindowConversation) SendStream(ctx context.Context, callback StreamCallback) error {
	return w.sendWindowed(func(conv *Conversation) error { return conv.SendStream(ctx, callback) })
}

func (w *WindowConversation) ExportJSON() (string, error) {
	data, err := json.Marshal(windowConversationSnapshot{
		conversationJSONSnapshot: w.conversationSnapshot(),
		WindowSize:               &w.windowSize,
	})
	return string(data), err
}

func (w *WindowConversation) ImportJSON(data string) error {
	var snapshot windowConversationSnapshot
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		return err
	}
	if snapshot.WindowSize == nil {
		return errors.New("window conversation state requires window_size")
	}
	w.applyConversationSnapshot(snapshot.conversationJSONSnapshot)
	w.MaxHistory = 0
	w.SetWindowSize(*snapshot.WindowSize)
	return nil
}

func (w *WindowConversation) sendWindowed(send func(*Conversation) error) error {
	if w.CurrentMsg == nil || len(w.CurrentMsg.parts) == 0 {
		return errors.New("current message is empty")
	}
	conv := *w.Conversation
	start := len(w.History)
	size := estimateMessage(w.CurrentMsg.Build()) + 8
	if conv.SystemPrompt != nil {
		size += estimateMessage(*conv.SystemPrompt)
	}
	limit := w.budget - conv.currentRequestOptions().MaxTokens
	if w.budget > 0 && size > limit {
		return errors.New("current input exceeds the context budget; shorten the message")
	}
	for rounds := 0; rounds < w.windowSize && start >= 2; rounds++ {
		user, assistant := w.History[start-2], w.History[start-1]
		if user.Role != openai.ChatMessageRoleUser || assistant.Role != openai.ChatMessageRoleAssistant {
			break
		}
		size += estimateMessage(user) + estimateMessage(assistant)
		if w.budget > 0 && size > limit {
			break
		}
		start -= 2
	}
	return w.Conversation.sendWithHistory(conv, start, send)
}
