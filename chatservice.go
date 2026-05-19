// Package main - Chat service layer
//
// chatservice.go 负责暴露给Wails前端的所有聊天相关API，分为两部分：
//
// 1. 会话管理API (session-based):
//   - ListSessions(scopeType, bookPath) 列出指定作用域的所有会话
//   - CreateSession(scopeType, bookPath, title) 创建新会话
//   - LoadSession(scopeType, bookPath, sessionID) 加载会话详情
//   - DeleteSession(scopeType, bookPath, sessionID) 删除会话
//   - SendMessageInSession(...) 同步发送消息（阻塞直到回复完成）
//   - SendMessageStreamInSession(...) 异步流式发送消息（后台线程+事件通知）
//
// 2. 全局会话API (legacy, 无作用域):
//   - SendMessage(message) 发送消息到全局会话
//   - SendMessageStream(message) 异步流式发送到全局会话
//   - GetTraceState() 获取当前会话的思路摘要
//   - ResetConversation() 重置全局会话
//
// 核心设计原则：
// - 线程安全：所有操作都通过mutex保护
// - 事件驱动：流式操作通过"chat_chunk"、"chat_done"、"chat_stream_error"事件通知前端
// - 双重持久化：LLMState(对话树) + Messages(UI展示)
// - 内容哈希：书籍使用文件内容SHA1识别，而非路径
package main

import (
	"context"
	"fmt"
	"hreader/llmkit"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// LLM提供商
	defaultProvider = llmkit.ProviderAliyun
	// 模型标识
	defaultModel = "qwen3-omni-flash"
	// 单次API调用超时时间
	chatTimeout = 120 * time.Second
)

// ChatService 聊天服务
// 字段说明：
// - mu: 互斥锁保护所有字段的并发访问
// - config: 应用配置（缓存的单例）
// - client: LLM API客户端（通过API_KEY初始化）
// - conversation: 全局会话对象（仅用于非作用域的消息发送）
type ChatService struct {
	mu           sync.Mutex
	config       *Config
	client       *llmkit.Client
	conversation *llmkit.TracedConversation
}

// NewChatService 创建并初始化一个ChatService实例
// 在启动时尝试加载保存的API_KEY并创建client
// 如果API_KEY无效或未配置，client保持nil，需要通过SaveAPIKey后续设置
func NewChatService() *ChatService {
	config := GetConfig()
	cs := &ChatService{
		config:       config,
		client:       nil,
		conversation: nil,
	}

	// 尝试使用已保存的API_KEY和模型配置初始化client
	if config.API_KEY != "" {
		provider := config.Provider
		if provider == "" {
			provider = string(defaultProvider)
		}

		model := config.Model
		if model == "" {
			model = defaultModel
		}

		if client, err := llmkit.NewClient(llmkit.Config{
			Provider: llmkit.ProviderType(provider),
			APIKey:   config.API_KEY,
			Model:    model,
		}); err == nil {
			cs.client = client
		}
	}

	return cs
}

// ListSessions 列出指定作用域内的所有会话
// 返回的会话列表按UpdatedAt时间倒序排列（最新优先）
func (c *ChatService) ListSessions(scopeType, bookPath string) ([]ChatSessionSummary, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return nil, err
	}

	if err := ensureDir(scope.ScopeDir); err != nil {
		return nil, err
	}

	idx, err := c.loadScopeIndex(scope)
	if err != nil {
		return nil, err
	}

	return idx.Sessions, nil
}

// CreateSession 创建新的会话
// title为空时默认使用"新会话"作为名称
func (c *ChatService) CreateSession(scopeType, bookPath, title string) (ChatSessionSummary, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return ChatSessionSummary{}, err
	}

	if err := ensureDir(scope.sessionsDir()); err != nil {
		return ChatSessionSummary{}, err
	}

	now := time.Now().Format(time.RFC3339)
	sessionID := makeSessionID()
	name := strings.TrimSpace(title)
	if name == "" {
		name = "新会话"
	}

	stored := &storedChatSession{
		SchemaVersion:      chatStoreSchemaVersion,
		SessionID:          sessionID,
		ScopeType:          scope.ScopeType,
		BookHash:           scope.BookHash,
		BookPath:           scope.BookPath,
		Title:              name,
		CreatedAt:          now,
		UpdatedAt:          now,
		MessageCount:       0,
		LastMessagePreview: "",
		LLMStateJSON:       "",
		Messages:           []ChatMessageRecord{},
	}

	if err := writeJSONAtomic(scope.sessionFilePath(sessionID), stored); err != nil {
		return ChatSessionSummary{}, err
	}

	idx, err := c.loadScopeIndex(scope)
	if err != nil {
		return ChatSessionSummary{}, err
	}

	summary := summaryFromStoredSession(stored)
	upsertSessionSummary(idx, summary)
	if err := c.saveScopeIndex(scope, idx); err != nil {
		return ChatSessionSummary{}, err
	}

	return summary, nil
}

// LoadSession 加载会话详情（包括完整消息历史和LLM状态）
func (c *ChatService) LoadSession(scopeType, bookPath, sessionID string) (ChatSessionDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return ChatSessionDetail{}, err
	}

	stored, err := c.loadStoredSession(scope, sessionID)
	if err != nil {
		return ChatSessionDetail{}, err
	}

	detail := ChatSessionDetail{
		Summary:  summaryFromStoredSession(stored),
		Messages: stored.Messages,
		LLMState: stored.LLMStateJSON,
	}

	return detail, nil
}

// DeleteSession 删除指定会话
// 删除会话文件和索引中的记录
func (c *ChatService) DeleteSession(scopeType, bookPath, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return err
	}

	if err := os.Remove(scope.sessionFilePath(sessionID)); err != nil && !os.IsNotExist(err) {
		return err
	}

	idx, err := c.loadScopeIndex(scope)
	if err != nil {
		return err
	}

	removeSessionSummary(idx, sessionID)
	return c.saveScopeIndex(scope, idx)
}

// SendMessageInSession 在指定会话中同步发送消息（阻塞直到LLM回复）
// 参数：
//   - scopeType: "library" 或 "book"
//   - bookPath: 书籍路径（scopeType=="book"时必需）
//   - sessionID: 会话ID
//   - message: 文本消息
//   - imagePaths: 附加的图片路径列表（可为nil）
//
// 流程：
//  1. 从文件加载会话状态
//  2. 恢复TracedConversation对象
//  3. 添加图片和文本到消息缓冲
//  4. 调用LLM API（阻塞120秒）
//  5. 序列化新状态到文件（原子操作）
//  6. 更新索引（最新会话优先）
func (c *ChatService) SendMessageInSession(scopeType, bookPath, sessionID, message string, imagePaths []string) (string, error) {
	text := strings.TrimSpace(message)
	if text == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return "", err
	}

	stored, err := c.loadStoredSession(scope, sessionID)
	if err != nil {
		return "", err
	}

	conv, err := c.newSessionConversation(stored.LLMStateJSON)
	if err != nil {
		return "", err
	}

	for _, imgPath := range imagePaths {
		conv.AddImage(imgPath)
	}

	conv.AddText(text)
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	reply, err := conv.Chat(ctx)
	if err != nil {
		return "", err
	}

	state, err := conv.ExportJSON()
	if err != nil {
		return "", err
	}

	now := time.Now().Format(time.RFC3339)
	stored.LLMStateJSON = state
	stored.UpdatedAt = now
	stored.Messages = append(stored.Messages,
		ChatMessageRecord{Role: "user", Content: text, CreatedAt: now},
		ChatMessageRecord{Role: "assistant", Content: reply, CreatedAt: now},
	)
	stored.MessageCount = len(stored.Messages)
	stored.LastMessagePreview = clippedPreview(reply)

	if err := c.persistSession(scope, stored); err != nil {
		return "", err
	}

	return reply, nil
}

// SendMessageStreamInSession 在指定会话中异步流式发送消息
// 与SendMessageInSession的区别：
//   - 返回立即（在后台goroutine中执行）
//   - 通过Wails事件通知前端进度
//
// 事件：
//   - "chat_chunk" (string): 流式回复的每个文本块
//   - "chat_done" (string): 完整回复（所有块拼接）
//   - "chat_stream_error" (string): 发生错误
func (c *ChatService) SendMessageStreamInSession(scopeType, bookPath, sessionID, message string, imagePaths []string) error {
	text := strings.TrimSpace(message)
	if text == "" {
		return fmt.Errorf("message cannot be empty")
	}

	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		scope, err := c.resolveScope(scopeType, bookPath)
		if err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		stored, err := c.loadStoredSession(scope, sessionID)
		if err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		conv, err := c.newSessionConversation(stored.LLMStateJSON)
		if err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		for _, imgPath := range imagePaths {
			conv.AddImage(imgPath)
		}

		conv.AddText(text)
		ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
		defer cancel()

		var full string
		cb := func(chunk string) error {
			if chunk == "" {
				return nil
			}
			full += chunk
			_ = application.Get().Event.Emit("chat_chunk", chunk)
			return nil
		}

		if err := conv.SendStream(ctx, cb); err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		state, err := conv.ExportJSON()
		if err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		now := time.Now().Format(time.RFC3339)
		stored.LLMStateJSON = state
		stored.UpdatedAt = now
		// 保存用户消息时记录附件（图片）
		stored.Messages = append(stored.Messages,
			ChatMessageRecord{Role: "user", Content: text, Attachments: imagePaths, CreatedAt: now},
			ChatMessageRecord{Role: "assistant", Content: full, CreatedAt: now},
		)
		stored.MessageCount = len(stored.Messages)
		stored.LastMessagePreview = clippedPreview(full)

		if err := c.persistSession(scope, stored); err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		_ = application.Get().Event.Emit("chat_done", full)
	}()

	return nil
}

// GetAPIKey 从缓存配置中读取API密钥
// 无需重新加载文件，使用单例缓存
func (c *ChatService) GetAPIKey() string {
	if c.config == nil {
		return ""
	}
	return c.config.API_KEY
}

// SaveAPIKey 保存新的API密钥到配置文件
// 流程：
//  1. 验证新API_KEY是否有效（创建临时client测试）
//  2. 保存到配置文件
//  3. 替换当前client对象
//  4. 重置conversation（会在下次ensureConversationLocked时重建）
//
// 如果API_KEY无效，保存失败且不会修改现有client
func (c *ChatService) SaveAPIKey(apiKey string) error {
	cleanKey := strings.TrimSpace(apiKey)
	if cleanKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	// 先验证API_KEY是否有效（创建临时client）
	provider := c.config.Provider
	if provider == "" {
		provider = string(defaultProvider)
	}

	model := c.config.Model
	if model == "" {
		model = defaultModel
	}

	newClient, err := llmkit.NewClient(llmkit.Config{
		Provider: llmkit.ProviderType(provider),
		APIKey:   cleanKey,
		Model:    model,
	})
	if err != nil {
		return fmt.Errorf("invalid api key: %w", err)
	}

	// 保存到配置文件（同时更新内存缓存）
	c.config.API_KEY = cleanKey
	if err := c.config.SaveToFile(); err != nil {
		return err
	}

	// 更新client并重置conversation
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client = newClient
	c.conversation = nil

	return nil
}

// GetModelConfig 获取模型配置（提供商和具体模型）
func (c *ChatService) GetModelConfig() map[string]string {
	if c.config == nil {
		return map[string]string{
			"provider": "aliyun",
			"model":    defaultModel,
		}
	}

	provider := c.config.Provider
	if provider == "" {
		provider = "aliyun"
	}

	model := c.config.Model
	if model == "" {
		model = defaultModel
	}

	return map[string]string{
		"provider": provider,
		"model":    model,
	}
}

// SaveModelConfig 保存模型配置到配置文件
func (c *ChatService) SaveModelConfig(config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	provider := config["provider"]
	model := config["model"]

	// 更新配置
	c.config.Provider = provider
	c.config.Model = model

	// 保存到文件
	if err := c.config.SaveToFile(); err != nil {
		return err
	}

	// 如果 client 已初始化，需要重新创建以使用新配置
	if c.client != nil && c.config.API_KEY != "" {
		newClient, err := llmkit.NewClient(llmkit.Config{
			Provider: llmkit.ProviderType(provider),
			APIKey:   c.config.API_KEY,
			Model:    model,
		})
		if err == nil {
			c.client = newClient
			c.conversation = nil // 重置会话
		}
	}

	return nil
}

// SendMessage 全局会话API：同步发送消息（不涉及存储）
// 用途：快速测试、对话框交互场景不需要持久化的情形
func (c *ChatService) SendMessage(message string) (string, error) {
	text := strings.TrimSpace(message)
	if text == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConversationLocked(); err != nil {
		return "", err
	}

	c.conversation.AddText(text)
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	defer cancel()

	return c.conversation.Chat(ctx)
}

// ResetConversation 重置全局会话对象
func (c *ChatService) ResetConversation() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conversation = nil
}

// GetTraceState 获取全局会话的当前思路摘要（trace state）
// Trace是LLM长期记忆的核心，用于压缩冗长的对话历史
func (c *ChatService) GetTraceState() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conversation == nil {
		return ""
	}

	return c.conversation.GetTrace()
}

// ensureConversationLocked 确保conversation已初始化
// 必须在持有mutex的情况下调用
// 前提：client已由SaveAPIKey验证并初始化
// 如果conversation为nil，从现有client创建新的conversation对象
func (c *ChatService) ensureConversationLocked() error {
	if c.client == nil {
		return fmt.Errorf("client not initialized; call SaveAPIKey first")
	}

	// 如果conversation已存在，直接复用
	if c.conversation != nil {
		return nil
	}

	// 从 client 创建新的 conversation
	conversation := c.client.NewTracedConversation(nil)
	conversation.SetMaxHistory(8)
	conversation.SetMaxTokens(4096) // 提高最大输出 token 限制
	c.conversation = conversation

	return nil
}

// newSessionConversation 创建会话专用的TracedConversation对象
// 参数llmStateJSON可以是：
//   - 空字符串：创建全新会话
//   - 有效JSON：从保存的状态恢复会话（包括完整对话历史和思路摘要）
func (c *ChatService) newSessionConversation(llmStateJSON string) (*llmkit.TracedConversation, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not initialized; call SaveAPIKey first")
	}

	conv := c.client.NewTracedConversation(nil)
	conv.SetMaxHistory(8)
	conv.SetMaxTokens(4096) // 提高最大输出 token 限制
	if strings.TrimSpace(llmStateJSON) != "" {
		if err := conv.ImportJSON(llmStateJSON); err != nil {
			return nil, fmt.Errorf("restore session failed: %w", err)
		}
	}

	return conv, nil
}

// loadStoredSession 从文件系统加载已保存的会话数据
// 包括元数据、消息历史、LLM状态
func (c *ChatService) loadStoredSession(scope *chatScopeMeta, sessionID string) (*storedChatSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}

	var stored storedChatSession
	if err := readJSONFile(scope.sessionFilePath(sessionID), &stored); err != nil {
		return nil, err
	}

	if stored.Messages == nil {
		stored.Messages = []ChatMessageRecord{}
	}

	if stored.SchemaVersion == 0 {
		stored.SchemaVersion = chatStoreSchemaVersion
	}

	return &stored, nil
}

// persistSession 原子性保存会话数据和更新索引
// 步骤：
//  1. 写入会话文件（原子操作）
//  2. 加载索引
//  3. 更新或插入会话摘要
//  4. 保存索引（原子操作）
func (c *ChatService) persistSession(scope *chatScopeMeta, stored *storedChatSession) error {
	stored.SchemaVersion = chatStoreSchemaVersion
	if err := writeJSONAtomic(scope.sessionFilePath(stored.SessionID), stored); err != nil {
		return err
	}

	idx, err := c.loadScopeIndex(scope)
	if err != nil {
		return err
	}

	upsertSessionSummary(idx, summaryFromStoredSession(stored))
	return c.saveScopeIndex(scope, idx)
}

// SendMessageStream 全局会话API：异步流式发送消息
// 与SendMessageStreamInSession的区别：不涉及存储，仅在内存中维护全局会话对象
// 事件：
//   - "chat_chunk" (string): 流式回复的每个文本块
//   - "chat_done" (string): 完整回复
//   - "chat_stream_error" (string): 发生错误
func (c *ChatService) SendMessageStream(message string) error {
	text := strings.TrimSpace(message)
	if text == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// Start streaming in background goroutine so call returns immediately
	go func() {
		c.mu.Lock()
		if err := c.ensureConversationLocked(); err != nil {
			c.mu.Unlock()
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}
		conv := c.conversation
		c.mu.Unlock()

		var full string
		ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
		defer cancel()

		callback := func(chunk string) error {
			if chunk != "" {
				full += chunk
				_ = application.Get().Event.Emit("chat_chunk", chunk)
			}
			return nil
		}

		if err := conv.SendStream(ctx, callback); err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		_ = application.Get().Event.Emit("chat_done", full)
	}()

	return nil
}
