// Package main - Chat session storage layer
//
// chatstore.go 负责所有聊天会话的文件持久化和索引管理，采用"作用域分区"设计：
//
// 存储结构:
//
//	~/.config/hreader/chat_store/
//	├── scopes/
//	│   ├── library/              (全局会话)
//	│   │   ├── sessions_index.json
//	│   │   └── sessions/
//	│   │       ├── s_123456.json
//	│   │       └── s_789012.json
//	│   └── books/                (单书会话，按内容哈希分区)
//	│       ├── abc123.../        (书籍内容的SHA1哈希)
//	│       │   ├── sessions_index.json
//	│       │   └── sessions/
//	│       │       └── s_456789.json
//	│       └── def456.../
//
// 核心设计原则：
// 1. 内容哈希识别书籍：使用文件内容SHA1而非路径，防止书籍移动导致会话丢失
// 2. 原子文件操作：通过临时文件+rename保证crash时数据不损坏
// 3. 双重持久化：LLMState(conversation state) + Messages(UI展示)
// 4. 索引加速：每个作用域维护sessions_index.json快速列表查询，避免遍历文件系统
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// 作用域类型：全局库级会话
	chatScopeLibrary = "library"
	// 作用域类型：单个书籍的会话
	chatScopeBook = "book"
	// 存储格式版本，用于前向/后向兼容性
	chatStoreSchemaVersion = 1
)

// ChatSessionSummary 会话摘要，用于列表显示（不包含完整消息历史）
// 用途：快速展示所有会话列表，避免加载完整的消息历史
type ChatSessionSummary struct {
	SessionID          string `json:"session_id"`           // 唯一标识，格式: s_<nanoseconds>
	ScopeType          string `json:"scope_type"`           // "library" 或 "book"
	BookHash           string `json:"book_hash,omitempty"`  // 书籍内容SHA1（仅book作用域）
	BookPath           string `json:"book_path,omitempty"`  // 书籍文件路径（仅book作用域）
	Title              string `json:"title"`                // 用户设置的会话名称
	CreatedAt          string `json:"created_at"`           // RFC3339时间戳
	UpdatedAt          string `json:"updated_at"`           // 最后更新时间（用于排序）
	MessageCount       int    `json:"message_count"`        // 消息总数
	LastMessagePreview string `json:"last_message_preview"` // 最后一条消息的前80字符预览
}

// ChatMessageRecord 存储在会话中的单条消息记录
// 用途：向前端返回完整对话历史供UI渲染，独立于LLM状态存储
type ChatMessageRecord struct {
	Role      string `json:"role"`       // "user" 或 "assistant"
	Content   string `json:"content"`    // 消息内容（仅文本，图片已消费）
	CreatedAt string `json:"created_at"` // RFC3339时间戳
}

// ChatSessionDetail 完整会话详情（前端查看会话时返回）
// 包含摘要、消息历史、LLM状态三部分
type ChatSessionDetail struct {
	Summary  ChatSessionSummary  `json:"summary"`        // 会话元数据
	Messages []ChatMessageRecord `json:"messages"`       // UI显示的消息列表
	LLMState string              `json:"llm_state_json"` // TracedConversation.ExportJSON()序列化后的状态
}

// chatScopeMeta 作用域元数据，用于路径解析和文件操作
// 私有结构体，不直接暴露给前端
type chatScopeMeta struct {
	ScopeType string // "library" 或 "book"
	BookHash  string // 书籍内容SHA1哈希（仅book作用域）
	BookPath  string // 原始书籍文件路径（仅book作用域）
	ScopeDir  string // 该作用域对应的文件系统目录
}

// storedChatSession 磁盘上保存的完整会话文件格式
// 一个JSON文件即为一个会话，包含LLM状态和UI消息列表
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
	LLMStateJSON       string              `json:"llm_state_json"` // TracedConversation状态，恢复时用ImportJSON()反序列化
	Messages           []ChatMessageRecord `json:"messages"`       // 消息历史（应用层维护）
}

// chatScopeIndex 索引文件结构，存储在 sessions_index.json
// 用于快速列表查询，避免遍历sessions/目录下的所有文件
type chatScopeIndex struct {
	SchemaVersion int                  `json:"schema_version"`
	Sessions      []ChatSessionSummary `json:"sessions"` // 按UpdatedAt倒序排列
}

// normalizeScopeType 规范化作用域类型，空值默认为library
func normalizeScopeType(scopeType string) string {
	v := strings.ToLower(strings.TrimSpace(scopeType))
	if v == "" {
		return chatScopeLibrary
	}
	return v
}

// hashBookContent 计算书籍文件内容的SHA1哈希
// 相比路径哈希的优势：
// - 文件移动不会改变哈希值，会话保持可用
// - 文件替换会得到不同哈希，自动分离不同书籍的会话
// 性能：只在书籍首次打开时计算一次，后续使用缓存的哈希值
func hashBookContent(bookPath string) (string, error) {
	data, err := os.ReadFile(bookPath)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:]), nil
}

func chatStoreRootDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "hreader", "chat_store"), nil
}

// resolveScope 根据作用域类型和书籍信息解析得到作用域元数据
// 对于book作用域，会计算书籍内容哈希以确定唯一的存储目录
func (c *ChatService) resolveScope(scopeType, bookPath string) (*chatScopeMeta, error) {
	normalizedScope := normalizeScopeType(scopeType)
	root, err := chatStoreRootDir()
	if err != nil {
		return nil, err
	}

	if normalizedScope == chatScopeLibrary {
		return &chatScopeMeta{
			ScopeType: chatScopeLibrary,
			ScopeDir:  filepath.Join(root, "scopes", "library"),
		}, nil
	}

	if normalizedScope != chatScopeBook {
		return nil, fmt.Errorf("unsupported scope type: %s", scopeType)
	}

	cleanPath := strings.TrimSpace(bookPath)
	if cleanPath == "" {
		return nil, fmt.Errorf("book path is required for book scope")
	}

	bookHash, err := hashBookContent(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash book content: %w", err)
	}

	return &chatScopeMeta{
		ScopeType: chatScopeBook,
		BookHash:  bookHash,
		BookPath:  cleanPath,
		ScopeDir:  filepath.Join(root, "scopes", "books", bookHash),
	}, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func (s *chatScopeMeta) sessionsDir() string {
	return filepath.Join(s.ScopeDir, "sessions")
}

func (s *chatScopeMeta) indexFilePath() string {
	return filepath.Join(s.ScopeDir, "sessions_index.json")
}

func (s *chatScopeMeta) sessionFilePath(sessionID string) string {
	return filepath.Join(s.sessionsDir(), sessionID+".json")
}

// writeJSONAtomic 原子写入JSON文件，防止部分写入时数据损坏
// 实现：先写到.tmp临时文件，再原子性rename到目标路径
func writeJSONAtomic(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// loadScopeIndex 加载指定作用域的会话索引
// 如果索引文件不存在，返回空索引（新作用域）
func (c *ChatService) loadScopeIndex(scope *chatScopeMeta) (*chatScopeIndex, error) {
	indexPath := scope.indexFilePath()
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return &chatScopeIndex{
			SchemaVersion: chatStoreSchemaVersion,
			Sessions:      []ChatSessionSummary{},
		}, nil
	}

	var idx chatScopeIndex
	if err := readJSONFile(indexPath, &idx); err != nil {
		return nil, err
	}
	if idx.Sessions == nil {
		idx.Sessions = []ChatSessionSummary{}
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = chatStoreSchemaVersion
	}
	return &idx, nil
}

func (c *ChatService) saveScopeIndex(scope *chatScopeMeta, idx *chatScopeIndex) error {
	idx.SchemaVersion = chatStoreSchemaVersion
	return writeJSONAtomic(scope.indexFilePath(), idx)
}

func summaryFromStoredSession(s *storedChatSession) ChatSessionSummary {
	return ChatSessionSummary{
		SessionID:          s.SessionID,
		ScopeType:          s.ScopeType,
		BookHash:           s.BookHash,
		BookPath:           s.BookPath,
		Title:              s.Title,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
		MessageCount:       s.MessageCount,
		LastMessagePreview: s.LastMessagePreview,
	}
}

// upsertSessionSummary 更新或插入会话摘要到索引
// 更新后按UpdatedAt倒序排列（最新的会话排在前面）
func upsertSessionSummary(idx *chatScopeIndex, summary ChatSessionSummary) {
	replaced := false
	for i := range idx.Sessions {
		if idx.Sessions[i].SessionID == summary.SessionID {
			idx.Sessions[i] = summary
			replaced = true
			break
		}
	}
	if !replaced {
		idx.Sessions = append(idx.Sessions, summary)
	}

	// 新的/最近更新的会话优先展示
	sort.SliceStable(idx.Sessions, func(i, j int) bool {
		return idx.Sessions[i].UpdatedAt > idx.Sessions[j].UpdatedAt
	})
}

// removeSessionSummary 从索引中删除指定会话（原地修改）
func removeSessionSummary(idx *chatScopeIndex, sessionID string) {
	out := idx.Sessions[:0]
	for _, s := range idx.Sessions {
		if s.SessionID != sessionID {
			out = append(out, s)
		}
	}
	idx.Sessions = out
}

// makeSessionID 生成唯一的会话ID，基于纳秒级时间戳
func makeSessionID() string {
	return fmt.Sprintf("s_%d", time.Now().UnixNano())
}

// clippedPreview 生成消息预览文本，最多80字符
func clippedPreview(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 80 {
		return trimmed
	}
	return trimmed[:80]
}
