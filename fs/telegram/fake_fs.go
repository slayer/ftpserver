package telegram

import (
	"os"
	"sync"
)

// fakeFilesystem is a minimal in-memory filesystem implementation that stores temporary metadata
// about files and directories. It provides basic operations (mkdir, create, stat) to satisfy
// FTP client expectations that require directory creation and file stat operations before uploads.
// This is not a persistent filesystem - all data is lost when the process terminates.
type fakeFilesystem struct {
	sync.Mutex
	dict map[string]*FileInfo
}

// newFakeFilesystem creates a new fake filesystem
func newFakeFilesystem() *fakeFilesystem {
	return &fakeFilesystem{
		dict: map[string]*FileInfo{},
	}
}

// mkdir creates a directory
func (f *fakeFilesystem) mkdir(name string, mode os.FileMode) {
	f.Lock()
	defer f.Unlock()
	f.dict[name] = &FileInfo{&FileData{
		name: name,
		dir:  true,
		mode: mode,
	}}
}

// create creates a file
func (f *fakeFilesystem) create(name string) {
	f.Lock()
	defer f.Unlock()
	f.dict[name] = &FileInfo{&FileData{
		name: name,
		dir:  false,
	}}
}

// setSize sets the size of a file
func (f *fakeFilesystem) setSize(name string, size int64) {
	f.Lock()
	defer f.Unlock()
	if fileInfo, found := f.dict[name]; found {
		fileInfo.size = size
	}
}

// stat returns a file info
func (f *fakeFilesystem) stat(name string) *FileInfo {
	f.Lock()
	defer f.Unlock()
	return f.dict[name]
}
