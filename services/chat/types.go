// Package chat - 聊天服务数据类型定义
//
// types.go 包含所有公开和私有的数据结构、常量定义。
// 这些类型被其他模块（store、session_manager、message_handler 等）使用。
package chat

import (
	"time"
)

const (
	// 作用域类型：全局库级会话
	chatScopeLibrary = "library"
	// 作用域类型：单个书籍的会话
	chatScopeBook = "book"
	// 存储格式版本，用于前向/后向兼容性
	chatStoreSchemaVersion = 1

	// 单次 API 调用超时时间（120秒）
	chatTimeout = 120 * time.Second

	streamFirstTimeout = 3 * time.Minute
	streamIdleTimeout  = 90 * time.Second
	streamTotalTimeout = 10 * time.Minute
)

// ============================================================================
// 公开的数据结构（Wails 前端可见）
// ============================================================================

// ChatSessionSummary 会话摘要，用于列表显示（不包含完整消息历史）
//
// 用途：
//   - 快速展示所有会话列表，避免加载完整的消息历史
//   - 在 UI 中显示会话卡片（标题、时间、预览）
type ChatSessionSummary struct {
	SessionID          string `json:"session_id"`           // 唯一标识，格式: s_<nanoseconds>
	ScopeType          string `json:"scope_type"`           // "library" 或 "book"
	BookHash           string `json:"book_hash,omitempty"`  // 书籍内容 SHA1（仅 book 作用域）
	BookPath           string `json:"book_path,omitempty"`  // 书籍文件路径（仅 book 作用域）
	Title              string `json:"title"`                // 用户设置的会话名称
	CreatedAt          string `json:"created_at"`           // RFC3339 时间戳
	UpdatedAt          string `json:"updated_at"`           // 最后更新时间（用于排序）
	MessageCount       int    `json:"message_count"`        // 消息总数
	LastMessagePreview string `json:"last_message_preview"` // 最后一条消息的前 80 字符预览
}

// ChatMessageRecord 存储在会话中的单条消息记录
//
// 用途：
//   - 向前端返回完整对话历史供 UI 渲染
//   - 独立于 LLM 状态存储（应用层维护的消息列表）
type ChatMessageRecord struct {
	Role        string   `json:"role"`                  // "user" 或 "assistant"
	Content     string   `json:"content"`               // 消息内容（文本）
	Attachments []string `json:"attachments,omitempty"` // 附件列表（图片路径或 base64）
	CreatedAt   string   `json:"created_at"`            // RFC3339 时间戳
}

// ChatSessionDetail 完整会话详情（前端查看会话时返回）
//
// 包含三部分：
//   - Summary:  会话元数据（标题、时间等）
//   - Messages: UI 显示的消息列表
//   - LLMState: TracedConversation.ExportJSON() 序列化后的状态
type ChatSessionDetail struct {
	Summary  ChatSessionSummary  `json:"summary"`        // 会话元数据
	Messages []ChatMessageRecord `json:"messages"`       // UI 显示的消息列表
	LLMState string              `json:"llm_state_json"` // TracedConversation 状态
}

// ============================================================================
// 私有的数据结构（内部使用）
// ============================================================================

// chatScopeMeta 作用域元数据，用于路径解析和文件操作
//
// 私有结构体，不直接暴露给前端。
// 封装了作用域的目录路径和书籍信息。
type chatScopeMeta struct {
	ScopeType string // "library" 或 "book"
	BookHash  string // 书籍内容 SHA1 哈希（仅 book 作用域）
	BookPath  string // 原始书籍文件路径（仅 book 作用域）
	ScopeDir  string // 该作用域对应的文件系统目录
}

// storedChatSession 磁盘上保存的完整会话文件格式
//
// 一个 JSON 文件即为一个会话，包含：
//   - LLM 状态（用于恢复对话上下文）
//   - UI 消息列表（用于前端渲染）
type storedChatSession struct {
	SchemaVersion      int                 `json:"schema_version"` // 兼容性版本号
	SessionID          string              `json:"session_id"`
	ScopeType          string              `json:"scope_type"`
	BookHash           string              `json:"book_hash,omitempty"`
	BookPath           string              `json:"book_path,omitempty"`
	Title              string              `json:"title"`
	CreatedAt          string              `json:"created_at"`
	UpdatedAt          string              `json:"updated_at"`
	MessageCount       int                 `json:"message_count"`
	LastMessagePreview string              `json:"last_message_preview"`
	LLMStateJSON       string              `json:"llm_state_json"` // TracedConversation 状态，恢复时用 ImportJSON() 反序列化
	Messages           []ChatMessageRecord `json:"messages"`       // 消息历史（应用层维护）
}

// chatScopeIndex 索引文件结构，存储在 sessions_index.json
//
// 用途：
//   - 快速列表查询，避免遍历 sessions/ 目录下的所有文件
//   - 按 UpdatedAt 倒序排列（最新会话优先）
type chatScopeIndex struct {
	SchemaVersion int                  `json:"schema_version"`
	Sessions      []ChatSessionSummary `json:"sessions"` // 按UpdatedAt倒序排列
}
