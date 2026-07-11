// Package chat - 消息处理
//
// message_handler.go 负责消息的发送和流式传输逻辑。
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ============================================================================
// 同步消息发送
// ============================================================================

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

// ============================================================================
// 异步流式消息发送
// ============================================================================

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

		// 如果是会话的第一条消息（2条：user + assistant），自动提取标题
		if stored.MessageCount == 2 {
			// 检查是否应该自动设置标题：
			// 1. 标题是默认的"新会话"
			// 2. 标题是以"对话"结尾的自动生成标题（如"xxx 对话"）
			shouldAutoTitle := strings.TrimSpace(stored.Title) == "新会话" ||
				strings.HasSuffix(strings.TrimSpace(stored.Title), " 对话")

			if shouldAutoTitle {
				stored.Title = extractTitleFromFirstMessage(text)
			}
		}

		if err := c.persistSession(scope, stored); err != nil {
			_ = application.Get().Event.Emit("chat_stream_error", err.Error())
			return
		}

		_ = application.Get().Event.Emit("chat_done", full)
	}()

	return nil
}
