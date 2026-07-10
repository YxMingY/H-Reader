package book

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Annotation 大头针数据结构
type Annotation struct {
	ID           string  `json:"id"`             // 唯一标识
	SessionID    string  `json:"sessionId"`      // 关联的会话 ID
	MessageIndex int     `json:"messageIndex"`   // 消息在会话中的索引位置
	PageNum      int     `json:"pageNum"`        // PDF 页码
	X            float64 `json:"x"`              // X 坐标（归一化，0-1）
	Y            float64 `json:"y"`              // Y 坐标（归一化，0-1）
	CreatedAt    int64   `json:"createdAt"`      // 创建时间戳
	Note         string  `json:"note,omitempty"` // 可选的备注文字
}

// AnnotationService 大头针服务
type AnnotationService struct {
	mu sync.RWMutex
}

var annotationStore = &AnnotationService{}

// getAnnotationsFilePath 获取大头针文件路径
func (s *AnnotationService) getAnnotationsFilePath(bookHash string) string {
	dataDir := getAppDataDir()
	filename := fmt.Sprintf("annotations_%s.json", bookHash)
	return filepath.Join(dataDir, filename)
}

// loadAnnotations 从文件加载大头针列表
func (s *AnnotationService) loadAnnotations(bookHash string) ([]Annotation, error) {
	filePath := s.getAnnotationsFilePath(bookHash)

	// 文件不存在，返回空列表
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Annotation{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取大头针文件失败: %w", err)
	}

	var annotations []Annotation
	if err := json.Unmarshal(data, &annotations); err != nil {
		return nil, fmt.Errorf("解析大头针数据失败: %w", err)
	}

	return annotations, nil
}

// saveAnnotations 保存大头针列表到文件
func (s *AnnotationService) saveAnnotations(bookHash string, annotations []Annotation) error {
	filePath := s.getAnnotationsFilePath(bookHash)

	data, err := json.MarshalIndent(annotations, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化大头针数据失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入大头针文件失败: %w", err)
	}

	return nil
}

// AddAnnotation 添加大头针（对外 API）
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//   - annotation: 大头针数据
//
// 返回：
//   - error: 操作失败时的错误
func (s *AnnotationService) AddAnnotation(bookPath string, annotation Annotation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算文件哈希
	bookHash, err := CalculateFileHash(bookPath)
	if err != nil {
		return fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 加载现有大头针
	annotations, err := s.loadAnnotations(bookHash)
	if err != nil {
		return err
	}

	// 检查是否已存在相同 ID 的大头针
	for i, a := range annotations {
		if a.ID == annotation.ID {
			// 更新现有大头针
			annotations[i] = annotation
			return s.saveAnnotations(bookHash, annotations)
		}
	}

	// 添加新大头针
	annotations = append(annotations, annotation)
	return s.saveAnnotations(bookHash, annotations)
}

// GetAnnotations 获取指定 PDF 的所有大头针（对外 API）
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//
// 返回：
//   - []Annotation: 大头针列表
//   - error: 操作失败时的错误
func (s *AnnotationService) GetAnnotations(bookPath string) ([]Annotation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 计算文件哈希
	bookHash, err := CalculateFileHash(bookPath)
	if err != nil {
		return []Annotation{}, fmt.Errorf("计算文件哈希失败: %w", err)
	}

	return s.loadAnnotations(bookHash)
}

// RemoveAnnotation 删除大头针（对外 API）
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//   - annotationID: 要删除的大头针 ID
//
// 返回：
//   - error: 操作失败时的错误
func (s *AnnotationService) RemoveAnnotation(bookPath string, annotationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算文件哈希
	bookHash, err := CalculateFileHash(bookPath)
	if err != nil {
		return fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 加载现有大头针
	annotations, err := s.loadAnnotations(bookHash)
	if err != nil {
		return err
	}

	// 查找并删除
	found := false
	filtered := make([]Annotation, 0, len(annotations))
	for _, a := range annotations {
		if a.ID == annotationID {
			found = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if !found {
		return fmt.Errorf("未找到 ID 为 %s 的大头针", annotationID)
	}

	return s.saveAnnotations(bookHash, filtered)
}

// UpdateAnnotation 更新大头针（对外 API）
//
// 参数：
//   - bookPath: PDF 文件的完整路径
//   - annotation: 更新后的大头针数据
//
// 返回：
//   - error: 操作失败时的错误
func (s *AnnotationService) UpdateAnnotation(bookPath string, annotation Annotation) error {
	// 复用 AddAnnotation 的逻辑（它会检查 ID 并更新）
	return s.AddAnnotation(bookPath, annotation)
}
