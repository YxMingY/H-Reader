package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BookService struct{}

// BookInfo 书籍信息结构
type BookInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// ScanBooks: 扫描文件夹
func (a *BookService) ScanBooks(dirPath string) []BookInfo {
	var books []BookInfo

	if dirPath == "" {
		home, _ := os.UserHomeDir()
		dirPath = filepath.Join(home, "Documents", "Papers")
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
