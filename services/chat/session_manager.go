// Package chat - 会话管理
//
// session_manager.go 负责会话的生命周期管理（CRUD 操作）。
package chat

import (
	"fmt"
	"hreader/llmkit"
	"os"
	"strings"
	"time"
)

// ============================================================================
// 会话 CRUD 操作
// ============================================================================

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

// UpdateSessionTitle 更新会话标题
//
// 参数：
//   - scopeType: 作用域类型
//   - bookPath:  书籍路径
//   - sessionID: 会话 ID
//   - title:     新的会话标题
//
// 流程：
//  1. 加载会话数据
//  2. 更新标题字段
//  3. 原子性保存会话和索引
//
// 返回：
//   - 错误信息（如果会话不存在或写入失败）
func (c *ChatService) UpdateSessionTitle(scopeType, bookPath, sessionID, title string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return err
	}

	stored, err := c.loadStoredSession(scope, sessionID)
	if err != nil {
		return err
	}

	// 更新标题
	stored.Title = strings.TrimSpace(title)
	if stored.Title == "" {
		stored.Title = "新会话"
	}

	// 持久化保存
	if err := c.persistSession(scope, stored); err != nil {
		return err
	}

	return nil
}

// RollbackSessionMessages 回滚会话中最新的 n 条消息对
//
// 参数：
//   - scopeType: 作用域类型（"library" 或 "book"）
//   - bookPath:  书籍路径（scopeType=="book" 时必需）
//   - sessionID: 会话 ID
//   - n:         要删除的消息对数量（1 = 删除最近的 user + assistant 各一条）
//
// 返回：
//   - 实际删除的消息对数量
//   - 错误信息（如果会话不存在或操作失败）
//
// 注意：
//   - 每条消息对包含一条用户消息和一条助手回复
//   - 会同时回滚 History 和 Trace State
//   - 如果 n 大于实际消息对数，会删除所有消息
func (c *ChatService) RollbackSessionMessages(scopeType, bookPath, sessionID string, n int) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	scope, err := c.resolveScope(scopeType, bookPath)
	if err != nil {
		return 0, err
	}

	stored, err := c.loadStoredSession(scope, sessionID)
	if err != nil {
		return 0, err
	}

	// 恢复 TracedConversation
	conv, err := c.newSessionConversation(stored.LLMStateJSON)
	if err != nil {
		return 0, err
	}

	// 执行回滚
	deletedPairs := conv.RollbackMessages(n)
	if deletedPairs == 0 {
		return 0, nil // 没有可删除的消息
	}

	// 重新导出状态
	newState, err := conv.ExportJSON()
	if err != nil {
		return 0, err
	}

	// 更新存储
	now := time.Now().Format(time.RFC3339)
	stored.LLMStateJSON = newState
	stored.UpdatedAt = now

	// 删除对应的 Messages 记录（每条消息对 = 2 条记录）
	messagesToRemove := deletedPairs * 2
	if messagesToRemove > len(stored.Messages) {
		messagesToRemove = len(stored.Messages)
	}
	stored.Messages = stored.Messages[:len(stored.Messages)-messagesToRemove]
	stored.MessageCount = len(stored.Messages)

	// 更新预览
	if len(stored.Messages) > 0 {
		lastMsg := stored.Messages[len(stored.Messages)-1]
		stored.LastMessagePreview = clippedPreview(lastMsg.Content)
	} else {
		stored.LastMessagePreview = ""
	}

	// 持久化保存
	if err := c.persistSession(scope, stored); err != nil {
		return 0, err
	}

	return deletedPairs, nil
}

// ============================================================================
// 辅助方法
// ============================================================================

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
//   - 使用与全局会话相同的上下文预算。
func (c *ChatService) newSessionConversation(llmStateJSON string) (*llmkit.TracedConversation, error) {
	if c.sidebar.client == nil {
		return nil, fmt.Errorf("请先在设置中为右侧栏对话选择供应商和模型")
	}

	conv := c.sidebar.client.NewTracedConversation(nil)

	// 如果有保存的状态，恢复到 conversation 中
	if strings.TrimSpace(llmStateJSON) != "" {
		if err := conv.ImportJSON(llmStateJSON); err != nil {
			return nil, fmt.Errorf("restore session failed: %w", err)
		}
	}

	configureConversationBudget(conv)
	return conv, nil
}

// Use a conservative default for user-selected models whose context limits
// are not available to the chat service.
func configureConversationBudget(conv *llmkit.TracedConversation) {
	conv.SetMaxTokens(4096)
	conv.SetContextBudget(16_384, 4)
}
