// Package chat - 聊天会话存储层
//
// store.go 负责所有聊天会话的文件持久化和索引管理，采用"作用域分区"设计：
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

// ============================================================================
// 工具函数
// ============================================================================

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

// extractTitleFromFirstMessage 从用户的第一条消息中提取会话标题
//
// 提取规则：
//   - 取第一句话（以句号、问号、感叹号或换行符为界）
//   - 最多保留 30 个字符
//   - 如果消息过短，直接使用整个消息
//
// 参数：
//   - message: 用户的消息文本
//
// 返回：
//   - 提取的标题字符串
func extractTitleFromFirstMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "新会话"
	}

	// 查找第一个句子结束符（中文/英文标点或换行）
	endIndex := len(trimmed)
	for i, char := range trimmed {
		if char == '。' || char == '？' || char == '！' || char == '.' || char == '?' || char == '!' || char == '\n' {
			endIndex = i + 1
			break
		}
	}

	// 提取第一句话
	firstSentence := trimmed[:endIndex]
	firstSentence = strings.TrimSpace(firstSentence)

	// 限制长度不超过 30 字符
	if len(firstSentence) > 30 {
		firstSentence = firstSentence[:30] + "..."
	}

	// 如果结果为空，使用默认值
	if firstSentence == "" {
		return "新会话"
	}

	return firstSentence
}

// ============================================================================
// 路径解析
// ============================================================================

// chatStoreRootDir 获取聊天存储根目录
//
// 返回：
//   - ~/.config/hreader/chat_store/
//   - 错误信息（如果无法获取用户配置目录）
func chatStoreRootDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "hreader", "chat_store"), nil
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

// ============================================================================
// 索引操作
// ============================================================================

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

// ============================================================================
// 会话加载/保存
// ============================================================================

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
