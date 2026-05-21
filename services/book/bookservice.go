package book

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hreader/services/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// BookInfo 书籍信息结构
type BookInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// BookService 书籍服务
type BookService struct {
	scanDir string
}

func (a *BookService) GetScanDir() string {
	// 优先从 Config 中读取默认文件夹
	cfg := config.GetConfig()
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
	cfg := config.GetConfig()
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
