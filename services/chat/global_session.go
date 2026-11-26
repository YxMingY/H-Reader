// Package chat - 全局会话（遗留接口）
//
// global_session.go 包含不基于作用域的全局会话 API。
// 这些是遗留接口，推荐使用基于作用域的新 API。
package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ============================================================================
// 全局会话 API（遗留）
// ============================================================================

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

// ResetConversation 重置全局会话对象
//
// 清空内存中的 conversation，下次调用 SendMessage 时会创建新的会话。
// 不影响基于作用域的会话（那些会话存储在磁盘中）。
func (c *ChatService) ResetConversation() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conversation = nil
}

// ============================================================================
// 内部辅助方法
// ============================================================================

// ensureConversationLocked 确保 conversation 已初始化
//
// 必须在持有 mutex 的情况下调用（由调用者负责加锁）。
//
// 前提条件：
//   - 已为右侧栏对话选择供应商和模型
//
// 行为：
//   - 如果 conversation 已存在，直接复用
//   - 如果 conversation 为 nil，从现有 client 创建新的 conversation 对象
//   - 设置 maxTokens=4096（提高最大输出 token 限制）
//
// 返回：
//   - 错误信息（如果 client 未初始化）
func (c *ChatService) ensureConversationLocked() error {
	if c.sidebar.client == nil {
		return fmt.Errorf("请先在设置中为右侧栏对话选择供应商和模型")
	}

	// 如果 conversation 已存在，直接复用
	if c.conversation != nil {
		return nil
	}

	// 从 client 创建新的 conversation
	conversation := c.sidebar.client.NewTracedConversation(nil)
	configureConversationBudget(conversation)
	c.conversation = conversation

	return nil
}
