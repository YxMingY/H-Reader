package chat

import (
	"hreader/llmkit"
	"hreader/services/book"
	"hreader/services/config"
	"sync"
)

type ChatService struct {
	mu             sync.Mutex
	config         *config.Config
	newClient      func(llmkit.Config) (*llmkit.Client, error)
	sidebar        apiRoute
	inline         apiRoute
	conversation   *llmkit.TracedConversation // Legacy non-persistent chat.
	notes          *book.NoteService
	inlineRequests sync.Map // Absolute session path -> active request.
}

type apiRoute struct {
	id     string
	config llmkit.Config
	client *llmkit.Client
}

func NewChatService(notes *book.NoteService) *ChatService {
	service := newChatService(config.GetConfig(), llmkit.NewClient)
	service.notes = notes
	return service
}

func newChatService(cfg *config.Config, newClient func(llmkit.Config) (*llmkit.Client, error)) *ChatService {
	service := &ChatService{config: cfg, newClient: newClient}
	service.applyAPISettingsLocked(cfg.ReadAPISettings())
	return service
}
