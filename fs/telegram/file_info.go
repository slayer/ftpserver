package telegram

import (
	"os"
	"path/filepath"
	"time"
)

// defaultDirSize is a standard directory size (4096 bytes, typical for filesystems)
const defaultDirSize = 4096

// FileData is a simple structure to store file information and implement os.FileInfo interface
type FileData struct {
	name    string
	dir     bool
	mode    os.FileMode
	modtime time.Time
	size    int64
}

type FileInfo struct {
	*FileData
}

// Implements os.FileInfo
func (s *FileInfo) Name() string {
	_, name := filepath.Split(s.name)
	return name
}

func (s *FileInfo) Mode() os.FileMode  { return s.mode }
func (s *FileInfo) ModTime() time.Time { return s.modtime }
func (s *FileInfo) IsDir() bool        { return s.dir }
func (s *FileInfo) Sys() interface{}   { return nil }
func (s *FileInfo) Size() int64 {
	if s.IsDir() {
		return defaultDirSize
	}
	return s.size
}
