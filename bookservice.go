package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type BookService struct {
	scanDir string
}

// BookInfo 书籍信息结构
type BookInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// ReadingProgress 阅读进度结构
//
// 设计说明：
// - 使用 BookHash（SHA1）作为唯一标识，而非文件路径
// - 优势：即使文件被移动或重命名，阅读进度依然有效
// - BookPath 仅用于日志显示和调试，不参与查找逻辑
type ReadingProgress struct {
	BookHash  string `json:"book_hash"`  // 书籍内容 SHA1 哈希（主键，用于查找）
	BookPath  string `json:"book_path"`  // 书籍路径（辅助信息，用于显示）
	Page      int    `json:"page"`       // 页码
	UpdatedAt string `json:"updated_at"` // 更新时间（RFC3339 格式）
}

// readingProgressStore 阅读进度存储
//
// 实现说明：
// - 使用内存 map 存储所有书籍的阅读进度
// - key 为书籍的 SHA1 哈希，value 为 ReadingProgress
// - 支持异步保存到 JSON 文件，避免阻塞主线程
// - 使用 sync.RWMutex 保证并发安全
type readingProgressStore struct {
	mu       sync.RWMutex
	progress map[string]*ReadingProgress // key: book_hash (SHA1)
	filePath string                      // JSON 文件存储路径
}

// 全局阅读进度存储实例
var progressStore = &readingProgressStore{
	progress: make(map[string]*ReadingProgress),
}

// initProgressStore 初始化阅读进度存储
//
// 调用时机：应用启动时（main.go 中调用）
// 功能：
// 1. 确定数据存储目录（~/.hreader/）
// 2. 加载已保存的阅读进度到内存
func initProgressStore() {
	// 获取应用数据目录
	dataDir := getAppDataDir()
	progressStore.filePath = filepath.Join(dataDir, "reading_progress.json")

	// 加载已保存的进度
	progressStore.loadFromFile()
}

// getAppDataDir 获取应用数据目录
//
// 返回：用户主目录下的 .hreader 文件夹路径
// 示例：C:\Users\YxMin\.hreader\ 或 /home/user/.hreader/
func getAppDataDir() string {
	// 使用用户主目录下的 .hreader 文件夹
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取主目录，使用当前目录
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".hreader")
	os.MkdirAll(dataDir, 0755)
	return dataDir
}

// loadFromFile 从文件加载阅读进度
//
// 流程：
// 1. 读取 JSON 文件
// 2. 反序列化为 map[string]*ReadingProgress
// 3. 赋值给 progressStore.progress
//
// 容错处理：
// - 文件不存在：静默失败，使用空进度
// - JSON 解析失败：静默失败，使用空进度
func (s *readingProgressStore) loadFromFile() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		// 文件不存在或读取失败，使用空进度
		return
	}

	var saved map[string]*ReadingProgress
	if err := json.Unmarshal(data, &saved); err != nil {
		// JSON 解析失败，使用空进度
		return
	}

	s.progress = saved
}

// saveToFile 保存阅读进度到文件
//
// 注意：调用者需要持有读锁（s.mu.RLock）
// 格式：JSON 缩进格式，便于人工阅读和调试
func (s *readingProgressStore) saveToFile() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.progress, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// SaveProgress 保存阅读进度
//
// 参数：
//   - bookHash: 书籍内容的 SHA1 哈希（作为 map 的 key）
//   - bookPath: 书籍文件路径（用于显示和调试）
//   - page: 当前页码
//
// 实现策略：
// 1. 更新内存中的进度数据
// 2. 异步保存到文件（不阻塞主线程）
// 3. 保存失败只打印错误，不影响用户体验
func (s *readingProgressStore) SaveProgress(bookHash string, bookPath string, page int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	s.progress[bookHash] = &ReadingProgress{
		BookHash:  bookHash,
		BookPath:  bookPath,
		Page:      page,
		UpdatedAt: now,
	}

	// 异步保存到文件（不阻塞主线程）
	go func() {
		if err := s.saveToFile(); err != nil {
			println("保存阅读进度失败:", err.Error())
		}
	}()

	return nil
}

// GetProgress 获取阅读进度
//
// 参数：
//   - bookHash: 书籍内容的 SHA1 哈希
//
// 返回值：
//   - int: 页码（未找到时返回默认值 1）
//   - bool: 是否存在记录
//
// 注意：未找到记录时返回 (1, false)，表示从第 1 页开始
func (s *readingProgressStore) GetProgress(bookHash string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	progress, exists := s.progress[bookHash]
	if !exists {
		return 1, false // 默认第 1 页
	}

	return progress.Page, true
}

// CalculateFileHash 计算文件的 SHA1 哈希
//
// 用途：
// - 为每本 PDF 生成唯一的标识符
// - 基于文件内容而非路径，确保文件移动后进度依然有效
//
// 参数：
//   - filePath: PDF 文件的完整路径
//
// 返回：
//   - string: 40 字符的十六进制哈希字符串
//   - error: 读取文件失败时的错误
//
// 示例：
//
//	hash, err := CalculateFileHash("C:\\books\\test.pdf")
//	// hash = "a1b2c3d4e5f6..." (40 字符)
func CalculateFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha1.Sum(data)
	return hex.EncodeToString(hash[:]), nil
}

func (a *BookService) GetScanDir() string {
	// 优先从 Config 中读取默认文件夹
	cfg := GetConfig()
	if cfg.DefaultFolder != "" {
		return cfg.DefaultFolder
	}

	// 如果 Config 中没有设置，返回空字符串（触发引导界面）
	// 不再使用默认路径，确保用户必须主动选择文件夹
	if a.scanDir != "" {
		return a.scanDir
	}
	return ""
}

func (a *BookService) ChooseDir() (string, error) {
	path, err := application.Get().Dialog.OpenFile().
		SetTitle("Select Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	a.scanDir = path

	// 将选择的文件夹保存到 Config
	cfg := GetConfig()
	cfg.DefaultFolder = path
	if err := cfg.SaveToFile(); err != nil {
		// 保存失败不影响选择结果，只记录错误
		println("保存默认文件夹失败:", err.Error())
	}

	return path, nil
}

// ScanBooks: 扫描文件夹
func (a *BookService) ScanBooks(dirPath string) []BookInfo {
	var books []BookInfo

	if dirPath == "" {
		dirPath = a.GetScanDir()
		os.MkdirAll(dirPath, 0755)
	}

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".pdf" {
			title := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			books = append(books, BookInfo{
				ID:    path,
				Title: title,
				Path:  path,
			})
		}
		return nil
	})

	return books
}

// LoadPDF: 读取文件内容返回给前端
func (a *BookService) LoadPDF(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// OpenInExplorer: 在资源管理器中打开文件所在目录
func (a *BookService) OpenInExplorer(path string) {
	cmd := exec.Command("explorer.exe", "/select,", path) // Windows 专用
	cmd.Start()
}

// SaveReadingProgress: 保存阅读进度（对外 API）
//
// 工作流程：
// 1. 根据文件路径计算 SHA1 哈希
// 2. 调用 progressStore.SaveProgress 保存
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//   - page: 当前页码
//
// 返回：
//   - error: 计算哈希或保存失败时的错误
func (a *BookService) SaveReadingProgress(bookPath string, page int) error {
	// 计算文件哈希
	bookHash, err := CalculateFileHash(bookPath)
	if err != nil {
		return err
	}

	return progressStore.SaveProgress(bookHash, bookPath, page)
}

// GetReadingProgress: 获取阅读进度（对外 API）
//
// 工作流程：
// 1. 根据文件路径计算 SHA1 哈希
// 2. 调用 progressStore.GetProgress 查询
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//
// 返回：
//   - int: 保存的页码（未找到或出错时返回 1）
//
// 注意：
// - 出错时返回默认值 1，不影响正常阅读
// - 前端无需处理错误，直接使用返回值即可
func (a *BookService) GetReadingProgress(bookPath string) int {
	// 计算文件哈希
	bookHash, err := CalculateFileHash(bookPath)
	if err != nil {
		return 1 // 出错时返回默认值
	}

	page, _ := progressStore.GetProgress(bookHash)
	return page
}
