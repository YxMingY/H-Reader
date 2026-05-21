// Package chat - 聊天服务层
//
// chatservice.go 负责暴露给 Wails 前端的所有聊天相关 API，分为三部分：
//
// 1. 会话管理 API（基于作用域）：
//   - ListSessions(scopeType, bookPath)     列出指定作用域的所有会话
//   - CreateSession(scopeType, bookPath, title) 创建新会话
//   - LoadSession(scopeType, bookPath, sessionID) 加载会话详情
//   - DeleteSession(scopeType, bookPath, sessionID) 删除会话
//   - SendMessageInSession(...)             同步发送消息（阻塞直到回复完成）
//   - SendMessageStreamInSession(...)       异步流式发送消息（后台线程 + 事件通知）
//
// 2. 全局会话 API（遗留接口，无作用域）：
//   - SendMessage(message)                  发送消息到全局会话
//   - SendMessageStream(message)            异步流式发送到全局会话
//   - GetTraceState()                       获取当前会话的思路摘要
//   - ResetConversation()                   重置全局会话
//
// 3. 配置管理 API：
//   - GetAPIKey()                           获取当前 API 密钥
//   - SaveAPIKey(apiKey)                    保存新的 API 密钥（会验证有效性）（会自动重建 client）
//   - GetModelConfig()                      获取模型配置（提供商和模型名称）
//   - SaveModelConfig(config)               保存模型配置（会自动重建 client）
//
// 核心设计原则：
// - 线程安全：所有操作都通过 mutex 保护
// - 事件驱动：流式操作通过 "chat_chunk"、"chat_done"、"chat_stream_error" 事件通知前端
// - 双重持久化：LLMState（对话树）+ Messages（UI 展示）
// - 内容哈希：书籍使用文件内容 SHA1 识别，而非路径
package chat

/*
 * 模型配置说明：
 *
 * 模型信息和 API Key 保存在后端的 Client 对象中。
 * 当用户在前端保存新的模型配置时：
 *   1. 调用 SaveModelConfig() 更新配置
 *   2. 后端替换内部的 Client 实例
 *   3. 下次发送消息时使用新的 Client 生成 Conversation
 *
 * 这意味着更换模型配置后会自动应用到后续的对话中，无需手动重建会话。
 */
import (
	"context"
	"fmt"
	"hreader/llmkit"
	"hreader/services/config"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// LLM 提供商默认值（阿里云）
	defaultProvider = llmkit.ProviderAliyun
	// 默认使用的模型标识
	defaultModel = "qwen3-omni-flash"
	// 单次 API 调用超时时间（120秒）
	chatTimeout = 120 * time.Second
)

// ChatService 聊天服务
//
// 字段说明：
//   - mu:           互斥锁，保护所有字段的并发访问
//   - config:       应用配置（从 services/config 包获取的单例缓存）
//   - client:       LLM API 客户端（通过 API_KEY 初始化，用于创建对话）
//   - conversation: 全局会话对象（仅用于非作用域的旧版消息发送，不持久化）
type ChatService struct {
	mu           sync.Mutex
	config       *config.Config
	client       *llmkit.Client
	conversation *llmkit.TracedConversation
}

// NewChatService 创建并初始化一个 ChatService 实例
//
// 启动时尝试从配置文件中加载已保存的 API_KEY 和模型配置，并创建 LLM 客户端。
// 如果 API_KEY 无效或未配置，client 保持 nil，需要通过 SaveAPIKey() 后续设置。
//
// 返回：
//   - 初始化后的 ChatService 实例
func NewChatService() *ChatService {
	config := config.GetConfig()
	cs := &ChatService{
		config:       config,
		client:       nil,
		conversation: nil,
	}

	// 尝试使用已保存的 API_KEY 和模型配置初始化 client
	if config.API_KEY != "" {
		provider := config.Provider
		if provider == "" {
			provider = string(defaultProvider)
		}

		model := config.Model
		if model == "" {
			model = defaultModel
		}

		// 创建 LLM 客户端，如果失败则保持 client 为 nil
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
//
// 参数：
//   - scopeType: 作用域类型（"library" 或 "book"）
//   - bookPath:  书籍路径（scopeType=="book" 时必需）
//
// 返回：
//   - 按 UpdatedAt 时间倒序排列的会话列表（最新优先）
//   - 错误信息（如果作用域解析失败）
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
//
// 参数：
//   - scopeType: 作用域类型（"library" 或 "book"）
//   - bookPath:  书籍路径（scopeType=="book" 时必需）
//   - title:     会话标题（为空时默认使用"新会话"）
//
// 流程：
//  1. 解析作用域并创建目录
//  2. 生成唯一的会话 ID（基于纳秒时间戳）
//  3. 创建空的会话文件（包含元数据但无消息）
//  4. 更新索引文件（sessions_index.json）
//
// 返回：
//   - 新创建的会话摘要
//   - 错误信息（如果文件写入失败）
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

// LoadSession 加载会话详情（包括完整消息历史和 LLM 状态）
//
// 参数：
//   - scopeType: 作用域类型
//   - bookPath:  书籍路径
//   - sessionID: 会话 ID
//
// 返回：
//   - 完整的会话详情（包含 Summary、Messages、LLMState）
//   - 错误信息（如果会话不存在或读取失败）
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
//
// 参数：
//   - scopeType: 作用域类型
//   - bookPath:  书籍路径
//   - sessionID: 要删除的会话 ID
//
// 流程：
//  1. 删除会话文件（s_xxx.json）
//  2. 从索引中移除该会话的记录
//  3. 保存更新后的索引
//
// 注意：如果会话文件不存在，不会报错（静默成功）
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

// SendMessageInSession 在指定会话中同步发送消息（阻塞直到 LLM 回复）
//
// 参数：
//   - scopeType:  "library" 或 "book"
//   - bookPath:   书籍路径（scopeType=="book" 时必需）
//   - sessionID:  会话 ID
//   - message:    文本消息内容
//   - imagePaths: 附加的图片路径列表（可为 nil）
//
// 流程：
//  1. 从文件加载会话状态（包括 LLMStateJSON 和 Messages）
//  2. 恢复 TracedConversation 对象（从 LLMStateJSON 反序列化）
//  3. 添加图片和文本到消息缓冲
//  4. 调用 LLM API（阻塞最多 120 秒）
//  5. 序列化新状态到文件（原子操作：临时文件 + rename）
//  6. 更新索引（将该会话移到列表顶部）
//
// 返回：
//   - LLM 的完整回复文本
//   - 错误信息（如果 API 调用失败或文件写入失败）
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

	// 将用户消息和助手回复添加到消息历史
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
//
// 与 SendMessageInSession 的区别：
//   - 立即返回（在后台 goroutine 中执行，不阻塞调用者）
//   - 通过 Wails 事件通知前端进度（实时显示回复）
//
// 参数：
//   - scopeType:  作用域类型
//   - bookPath:   书籍路径
//   - sessionID:  会话 ID
//   - message:    文本消息
//   - imagePaths: 图片路径列表
//
// 触发的事件：
//   - "chat_chunk"        (string): 流式回复的每个文本块（实时推送）
//   - "chat_done"         (string): 完整回复（所有块拼接后的最终结果）
//   - "chat_stream_error" (string): 发生错误时的错误信息
//
// 注意：
//   - 即使返回 nil，实际的消息发送可能在后台失败（通过 chat_stream_error 事件通知）
//   - 保存用户消息时会记录附件（imagePaths）
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

		// 流式回调：每收到一个文本块就发送给前端
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

		// 保存完整的对话历史和 LLM 状态
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

// GetAPIKey 从缓存配置中读取 API 密钥
//
// 无需重新加载文件，直接使用内存中的单例缓存。
// 如果配置未初始化，返回空字符串。
//
// 返回：
//   - 当前配置的 API_KEY
func (c *ChatService) GetAPIKey() string {
	if c.config == nil {
		return ""
	}
	return c.config.API_KEY
}

// SaveAPIKey 保存新的 API 密钥到配置文件
//
// 流程：
//  1. 验证新 API_KEY 是否有效（创建临时 client 测试连接）
//  2. 保存到配置文件（同时更新内存缓存）
//  3. 替换当前 client 对象
//  4. 重置 conversation（会在下次 ensureConversationLocked 时重建）
//
// 参数：
//   - apiKey: 新的 API 密钥
//
// 返回：
//   - 错误信息（如果 API_KEY 无效或文件写入失败）
//
// 注意：
//   - 如果 API_KEY 无效，保存失败且不会修改现有 client
//   - 保存成功后，后续对话会自动使用新的 API_KEY
func (c *ChatService) SaveAPIKey(apiKey string) error {
	cleanKey := strings.TrimSpace(apiKey)
	if cleanKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	// 先验证 API_KEY 是否有效（创建临时 client）
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

	// 更新 client 并重置 conversation
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client = newClient
	c.conversation = nil

	return nil
}

// GetModelConfig 获取模型配置（提供商和具体模型）
//
// 返回：
//   - map[string]string{"provider": 提供商, "model": 模型名称}
//   - 如果配置未初始化，返回默认值（aliyun, qwen3-omni-flash）
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
//
// 参数：
//   - config: map[string]string{"provider": 提供商, "model": 模型名称}
//
// 流程：
//  1. 更新内存中的配置
//  2. 保存到文件
//  3. 如果 client 已初始化，重新创建以使用新配置
//  4. 重置 conversation（下次对话时重建）
//
// 返回：
//   - 错误信息（如果文件写入失败）
//
// 注意：
//   - 更换模型后，现有会话的 LLMState 仍然有效
//   - 新消息会使用新的模型生成回复
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

// SendMessage 全局会话 API：同步发送消息（不涉及存储）
//
// 用途：
//   - 快速测试 LLM 连接
//   - 对话框交互场景不需要持久化的情形
//   - 遗留接口，推荐使用基于作用域的 SendMessageInSession
//
// 参数：
//   - message: 文本消息
//
// 返回：
//   - LLM 的完整回复
//   - 错误信息（如果 client 未初始化或 API 调用失败）
//
// 注意：
//   - 此方法不保存任何数据到磁盘
//   - 会话状态仅保存在内存中，重启后丢失
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
//
// 清空内存中的 conversation，下次调用 SendMessage 时会创建新的会话。
// 不影响基于作用域的会话（那些会话存储在磁盘中）。
func (c *ChatService) ResetConversation() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conversation = nil
}

// GetTraceState 获取全局会话的当前思路摘要（trace state）
//
// Trace 是 LLM 长期记忆的核心，用于压缩冗长的对话历史。
// 当对话轮数超过 maxHistory 时，LLM 会自动生成一个思路摘要，
// 保留关键上下文信息，避免 token 超限。
//
// 返回：
//   - 当前的 trace 摘要文本（如果会话不存在，返回空字符串）
func (c *ChatService) GetTraceState() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conversation == nil {
		return ""
	}

	return c.conversation.GetTrace()
}

// ensureConversationLocked 确保 conversation 已初始化
//
// 必须在持有 mutex 的情况下调用（由调用者负责加锁）。
//
// 前提条件：
//   - client 已由 SaveAPIKey 验证并初始化
//
// 行为：
//   - 如果 conversation 已存在，直接复用
//   - 如果 conversation 为 nil，从现有 client 创建新的 conversation 对象
//   - 设置 maxHistory=8（最多保留 8 轮对话历史）
//   - 设置 maxTokens=4096（提高最大输出 token 限制）
//
// 返回：
//   - 错误信息（如果 client 未初始化）
func (c *ChatService) ensureConversationLocked() error {
	if c.client == nil {
		return fmt.Errorf("client not initialized; call SaveAPIKey first")
	}

	// 如果 conversation 已存在，直接复用
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

// newSessionConversation 创建会话专用的 TracedConversation 对象
//
// 参数 llmStateJSON 可以是：
//   - 空字符串：创建全新会话（无历史）
//   - 有效 JSON：从保存的状态恢复会话（包括完整对话历史和思路摘要）
//
// 返回：
//   - 初始化后的 TracedConversation 对象
//   - 错误信息（如果 client 未初始化或 JSON 解析失败）
//
// 注意：
//   - 每个会话都有独立的 conversation 对象，互不影响
//   - maxHistory=8, maxTokens=4096（与全局会话相同配置）
func (c *ChatService) newSessionConversation(llmStateJSON string) (*llmkit.TracedConversation, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not initialized; call SaveAPIKey first")
	}

	conv := c.client.NewTracedConversation(nil)
	conv.SetMaxHistory(8)
	conv.SetMaxTokens(4096) // 提高最大输出 token 限制

	// 如果有保存的状态，恢复到 conversation 中
	if strings.TrimSpace(llmStateJSON) != "" {
		if err := conv.ImportJSON(llmStateJSON); err != nil {
			return nil, fmt.Errorf("restore session failed: %w", err)
		}
	}

	return conv, nil
}

// loadStoredSession 从文件系统加载已保存的会话数据
//
// 参数：
//   - scope:     作用域元数据（包含目录路径）
//   - sessionID: 要加载的会话 ID
//
// 返回：
//   - 完整的会话数据（包括元数据、消息历史、LLM 状态）
//   - 错误信息（如果文件不存在或 JSON 解析失败）
//
// 注意：
//   - 如果 Messages 为 nil，初始化为空切片
//   - 如果 SchemaVersion 为 0，设置为当前版本
func (c *ChatService) loadStoredSession(scope *chatScopeMeta, sessionID string) (*storedChatSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}

	var stored storedChatSession
	if err := readJSONFile(scope.sessionFilePath(sessionID), &stored); err != nil {
		return nil, err
	}

	// 确保 Messages 不为 nil
	if stored.Messages == nil {
		stored.Messages = []ChatMessageRecord{}
	}

	// 兼容旧版本格式
	if stored.SchemaVersion == 0 {
		stored.SchemaVersion = chatStoreSchemaVersion
	}

	return &stored, nil
}

// persistSession 原子性保存会话数据和更新索引
//
// 步骤：
//  1. 写入会话文件（原子操作：临时文件 + rename）
//  2. 加载索引
//  3. 更新或插入会话摘要（将该会话移到列表顶部）
//  4. 保存索引（原子操作）
//
// 参数：
//   - scope:  作用域元数据
//   - stored: 要保存的会话数据
//
// 返回：
//   - 错误信息（如果文件写入失败）
func (c *ChatService) persistSession(scope *chatScopeMeta, stored *storedChatSession) error {
	stored.SchemaVersion = chatStoreSchemaVersion
	if err := writeJSONAtomic(scope.sessionFilePath(stored.SessionID), stored); err != nil {
		return err
	}

	// 更新索引
	idx, err := c.loadScopeIndex(scope)
	if err != nil {
		return err
	}

	upsertSessionSummary(idx, summaryFromStoredSession(stored))
	return c.saveScopeIndex(scope, idx)
}

// SendMessageStream 全局会话 API：异步流式发送消息
//
// 与 SendMessageStreamInSession 的区别：
//   - 不涉及存储，仅在内存中维护全局会话对象
//   - 重启后会话丢失
//   - 遗留接口，推荐使用基于作用域的 SendMessageStreamInSession
//
// 参数：
//   - message: 文本消息
//
// 触发的事件：
//   - "chat_chunk"        (string): 流式回复的每个文本块
//   - "chat_done"         (string): 完整回复
//   - "chat_stream_error" (string): 发生错误
//
// 注意：
//   - 立即返回，实际发送在后台 goroutine 中执行
//   - 即使返回 nil，也可能通过 chat_stream_error 事件报告错误
func (c *ChatService) SendMessageStream(message string) error {
	text := strings.TrimSpace(message)
	if text == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// 在后台 goroutine 中执行流式发送，立即返回
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
