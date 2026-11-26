package book

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const notesVersion = 1

type Note struct {
	ID        string  `json:"id"`
	PageNum   int     `json:"pageNum"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Text      string  `json:"text"`
	Quote     string  `json:"quote,omitempty"`
	CreatedAt int64   `json:"createdAt"`
	UpdatedAt int64   `json:"updatedAt"`
}

type notesFile struct {
	Version int    `json:"version"`
	Notes   []Note `json:"notes"`
}

type NoteService struct {
	mu sync.Mutex
}

func notesPath(bookPath string) (string, error) {
	hash, err := CalculateFileHash(bookPath)
	if err != nil {
		return "", fmt.Errorf("计算书籍哈希失败: %w", err)
	}
	return filepath.Join(getAppDataDir(), "notes_"+hash+".json"), nil
}

func readNotes(path string) ([]Note, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Note{}, nil
	}
	if err != nil {
		return nil, err
	}
	var stored notesFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	if stored.Version != notesVersion || stored.Notes == nil {
		return nil, fmt.Errorf("无法读取笔记文件 %q，版本为 %d", path, stored.Version)
	}
	return stored.Notes, nil
}

func writeNotes(path string, notes []Note) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".notes-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := errors.Join(encoder.Encode(notesFile{Version: notesVersion, Notes: notes}), file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (s *NoteService) GetNotes(bookPath string) ([]Note, error) {
	path, err := notesPath(bookPath)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return readNotes(path)
}

func (s *NoteService) SaveNote(bookPath string, note Note) (Note, error) {
	if strings.TrimSpace(note.ID) == "" || note.PageNum < 1 ||
		!validNoteCoordinate(note.X) || !validNoteCoordinate(note.Y) {
		return Note{}, errors.New("笔记位置或标识无效")
	}
	path, err := notesPath(bookPath)
	if err != nil {
		return Note{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	notes, err := readNotes(path)
	if err != nil {
		return Note{}, err
	}
	note.UpdatedAt = time.Now().UnixMilli()
	for i, existing := range notes {
		if existing.ID == note.ID {
			note.CreatedAt = existing.CreatedAt
			notes[i] = note
			return note, writeNotes(path, notes)
		}
	}
	if strings.TrimSpace(note.Text) == "" {
		return Note{}, errors.New("新笔记正文为空")
	}
	note.CreatedAt = note.UpdatedAt
	return note, writeNotes(path, append(notes, note))
}

func validNoteCoordinate(value float64) bool {
	return !math.IsNaN(value) && value >= 0 && value <= 1
}

func (s *NoteService) RemoveNote(bookPath, id string) error {
	path, err := notesPath(bookPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	notes, err := readNotes(path)
	if err != nil {
		return err
	}
	for i, note := range notes {
		if note.ID == id {
			return writeNotes(path, append(notes[:i], notes[i+1:]...))
		}
	}
	return nil
}
