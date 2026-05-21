// Package chat - 聊天会话存储层
//
// chatstore.go 负责所有聊天会话的文件持久化和索引管理，采用“作用域分区”设计：
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
// 1. 内容哈希识别书籍：使用文件内容 SHA1 而非路径，防止书籍移动导致会话丢失
// 2. 原子文件操作：通过临时文件+rename 保证 crash 时数据不损坏
// 3. 双重持久化：LLMState（conversation state）+ Messages（UI 展示）
// 4. 索引加速：每个作用域维护 sessions_index.json 快速列表查询，避免遍历文件系统
package chat

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

// normalizeScopeType 规范化作用域类型
//
// 参数：
//   - scopeType: 原始作用域类型字符串
//
// 返回：
//   - 规范化后的类型（"library" 或 "book"）
//   - 空值默认为 "library"
func normalizeScopeType(scopeType string) string {
	v := strings.ToLower(strings.TrimSpace(scopeType))
	if v == "" {
		return chatScopeLibrary
	}
	return v
}

// hashBookContent 计算书籍文件内容的 SHA1 哈希
//
// 相比路径哈希的优势：
//   - 文件移动不会改变哈希值，会话保持可用
//   - 文件替换会得到不同哈希，自动分离不同书籍的会话
//
// 性能：
//   - 只在书籍首次打开时计算一次，后续使用缓存的哈希值
//
// 参数：
//   - bookPath: 书籍文件路径
//
// 返回：
//   - SHA1 哈希字符串（十六进制编码）
//   - 错误信息（如果文件读取失败）
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
//
// 对于 book 作用域，会计算书籍内容哈希以确定唯一的存储目录。
//
// 参数：
//   - scopeType: 作用域类型（"library" 或 "book"）
//   - bookPath:  书籍路径（scopeType=="book" 时必需）
//
// 返回：
//   - 作用域元数据（包含目录路径等信息）
//   - 错误信息（如果作用域类型不支持或书籍路径无效）
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

	// 计算书籍内容哈希
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

// ensureDir 确保目录存在，如果不存在则创建
//
// 参数：
//   - path: 目录路径
//
// 返回：
//   - 错误信息（如果创建失败）
func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// sessionsDir 获取会话文件存储目录
//
// 返回：
//   - <ScopeDir>/sessions/
func (s *chatScopeMeta) sessionsDir() string {
	return filepath.Join(s.ScopeDir, "sessions")
}

// indexFilePath 获取索引文件路径
//
// 返回：
//   - <ScopeDir>/sessions_index.json
func (s *chatScopeMeta) indexFilePath() string {
	return filepath.Join(s.ScopeDir, "sessions_index.json")
}

// sessionFilePath 获取指定会话的文件路径
//
// 参数：
//   - sessionID: 会话 ID
//
// 返回：
//   - <ScopeDir>/sessions/<sessionID>.json
func (s *chatScopeMeta) sessionFilePath(sessionID string) string {
	return filepath.Join(s.sessionsDir(), sessionID+".json")
}

// writeJSONAtomic 原子写入 JSON 文件，防止部分写入时数据损坏
//
// 实现：
//  1. 将对象序列化为 JSON
//  2. 写到 .tmp 临时文件
//  3. 原子性 rename 到目标路径（rename 是原子操作）
//
// 参数：
//   - path:  目标文件路径
//   - value: 要序列化的对象
//
// 返回：
//   - 错误信息（如果序列化或文件写入失败）
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

// readJSONFile 从文件读取并反序列化 JSON 对象
//
// 参数：
//   - path: 文件路径
//   - out:  输出对象指针（必须是指针类型）
//
// 返回：
//   - 错误信息（如果文件读取或 JSON 解析失败）
func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// loadScopeIndex 加载指定作用域的会话索引
//
// 如果索引文件不存在，返回空索引（新作用域）。
//
// 参数：
//   - scope: 作用域元数据
//
// 返回：
//   - 会话索引对象
//   - 错误信息（如果文件读取或 JSON 解析失败）
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

	// 确保 Sessions 不为 nil
	if idx.Sessions == nil {
		idx.Sessions = []ChatSessionSummary{}
	}

	// 兼容旧版本格式
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = chatStoreSchemaVersion
	}
	return &idx, nil
}

// saveScopeIndex 保存作用域索引到文件（原子操作）
//
// 参数：
//   - scope: 作用域元数据
//   - idx:   要保存的索引对象
//
// 返回：
//   - 错误信息（如果文件写入失败）
func (c *ChatService) saveScopeIndex(scope *chatScopeMeta, idx *chatScopeIndex) error {
	idx.SchemaVersion = chatStoreSchemaVersion
	return writeJSONAtomic(scope.indexFilePath(), idx)
}

// summaryFromStoredSession 从存储的会话数据生成会话摘要
//
// 用于更新索引时提取关键信息（不包含完整的 LLMState 和 Messages）。
//
// 参数：
//   - s: 存储的完整会话数据
//
// 返回：
//   - 会话摘要对象
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
//
// 行为：
//   - 如果会话已存在，更新其信息
//   - 如果会话不存在，添加到列表
//   - 更新后按 UpdatedAt 倒序排列（最新的会话排在前面）
//
// 参数：
//   - idx:     索引对象（会被原地修改）
//   - summary: 要插入/更新的会话摘要
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
//
// 参数：
//   - idx:       索引对象（会被原地修改）
//   - sessionID: 要删除的会话 ID
func removeSessionSummary(idx *chatScopeIndex, sessionID string) {
	out := idx.Sessions[:0]
	for _, s := range idx.Sessions {
		if s.SessionID != sessionID {
			out = append(out, s)
		}
	}
	idx.Sessions = out
}

// makeSessionID 生成唯一的会话 ID，基于纳秒级时间戳
//
// 格式：s_<nanoseconds>
// 例如：s_1747468800123456789
//
// 返回：
//   - 唯一的会话 ID 字符串
func makeSessionID() string {
	return fmt.Sprintf("s_%d", time.Now().UnixNano())
}

// clippedPreview 生成消息预览文本，最多 80 字符
//
// 用于在会话列表中显示最后一条消息的简短预览。
//
// 参数：
//   - text: 原始消息文本
//
// 返回：
//   - 裁剪后的预览文本（超过 80 字符会被截断）
func clippedPreview(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 80 {
		return trimmed
	}
	return trimmed[:80]
}
