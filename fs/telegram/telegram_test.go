package telegram

import (
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// TestFakeFilesystem tests the in-memory fake filesystem
func TestFakeFilesystem(t *testing.T) {
	fs := newFakeFilesystem()

	// Test mkdir
	fs.mkdir("/test", 0755)
	info := fs.stat("/test")
	if info == nil {
		t.Fatal("Expected directory to exist")
	}
	if !info.IsDir() {
		t.Error("Expected /test to be a directory")
	}
	if info.Mode() != 0755 {
		t.Errorf("Expected mode 0755, got %v", info.Mode())
	}

	// Test create
	fs.create("/test/file.txt")
	info = fs.stat("/test/file.txt")
	if info == nil {
		t.Fatal("Expected file to exist")
	}
	if info.IsDir() {
		t.Error("Expected /test/file.txt to be a file")
	}

	// Test setSize
	fs.setSize("/test/file.txt", 1234)
	info = fs.stat("/test/file.txt")
	if info.Size() != 1234 {
		t.Errorf("Expected size 1234, got %d", info.Size())
	}

	// Test non-existent file
	info = fs.stat("/nonexistent")
	if info != nil {
		t.Error("Expected nil for non-existent file")
	}
}

// TestFakeFilesystemConcurrency tests thread safety of fake filesystem
func TestFakeFilesystemConcurrency(t *testing.T) {
	fs := newFakeFilesystem()
	var wg sync.WaitGroup

	// Create multiple goroutines to test concurrent access
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			path := "/concurrent/file"
			fs.mkdir("/concurrent", 0755)
			fs.create(path)
			fs.setSize(path, int64(n))
			_ = fs.stat(path)
		}(i)
	}

	wg.Wait()
}

// TestFileInfo tests the FileInfo implementation
func TestFileInfo(t *testing.T) {
	// Test directory
	dirInfo := &FileInfo{&FileData{
		name: "/path/to/dir",
		dir:  true,
		mode: 0755,
	}}

	if dirInfo.Name() != "dir" {
		t.Errorf("Expected name 'dir', got '%s'", dirInfo.Name())
	}
	if !dirInfo.IsDir() {
		t.Error("Expected IsDir to be true")
	}
	if dirInfo.Mode() != 0755 {
		t.Errorf("Expected mode 0755, got %v", dirInfo.Mode())
	}
	// Directory size is returned as a constant value (42 in the current implementation)
	if !dirInfo.IsDir() || dirInfo.Size() <= 0 {
		t.Errorf("Expected positive dir size, got %d", dirInfo.Size())
	}
	if dirInfo.Sys() != nil {
		t.Error("Expected Sys() to return nil")
	}

	// Test file
	fileInfo := &FileInfo{&FileData{
		name: "/path/to/file.txt",
		dir:  false,
		mode: 0644,
		size: 5678,
	}}

	if fileInfo.Name() != "file.txt" {
		t.Errorf("Expected name 'file.txt', got '%s'", fileInfo.Name())
	}
	if fileInfo.IsDir() {
		t.Error("Expected IsDir to be false")
	}
	if fileInfo.Size() != 5678 {
		t.Errorf("Expected size 5678, got %d", fileInfo.Size())
	}
}

// TestIsExtension tests the file extension matching
func TestIsExtension(t *testing.T) {
	tests := []struct {
		filename   string
		extensions []string
		expected   bool
	}{
		{"test.jpg", imageExtensions, true},
		{"test.JPG", imageExtensions, true},
		{"test.jpeg", imageExtensions, true},
		{"TEST.PNG", imageExtensions, true},
		{"test.txt", imageExtensions, false},
		{"test.mp4", videoExtensions, true},
		{"test.MP4", videoExtensions, true},
		{"test.avi", videoExtensions, true},
		{"test.jpg", videoExtensions, false},
		{"test.mp3", audioExtensions, true},
		{"test.ogg", audioExtensions, true},
		{"test.txt", textExtensions, true},
		{"test.md", textExtensions, true},
		{"test", imageExtensions, false},
		{"", imageExtensions, false},
	}

	for _, tt := range tests {
		result := isExtension(tt.filename, tt.extensions)
		if result != tt.expected {
			t.Errorf("isExtension(%q, extensions) = %v, want %v", tt.filename, result, tt.expected)
		}
	}
}

// TestFileReadWrite tests concurrent read/write operations on File
func TestFileReadWrite(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file := &File{
		Path: "/test.txt",
		Fs:   fs,
	}

	// Test Write
	data := []byte("Hello, World!")
	n, err := file.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Test Read
	buf := make([]byte, 5)
	n, err = file.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, read %d", n)
	}
	if string(buf) != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", string(buf))
	}

	// Test Read remainder
	buf = make([]byte, 20)
	n, err = file.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 8 {
		t.Errorf("Expected to read 8 bytes, read %d", n)
	}
	if string(buf[:n]) != ", World!" {
		t.Errorf("Expected ', World!', got '%s'", string(buf[:n]))
	}

	// Test EOF
	n, err = file.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes at EOF, got %d", n)
	}
}

// TestFileReadWriteConcurrency tests thread safety of File operations
func TestFileReadWriteConcurrency(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file := &File{
		Path: "/test.txt",
		Fs:   fs,
	}

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := []byte{byte(n)}
			_, _ = file.Write(data)
		}(i)
	}

	wg.Wait()

	// Verify we wrote 10 bytes
	if len(file.Content) != 10 {
		t.Errorf("Expected 10 bytes written, got %d", len(file.Content))
	}
}

// TestFileReadEmptyContent tests reading from empty file
func TestFileReadEmptyContent(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file := &File{
		Path: "/empty.txt",
		Fs:   fs,
	}

	buf := make([]byte, 10)
	n, err := file.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF for empty file, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes read, got %d", n)
	}
}

// TestFileStat tests File.Stat() method
func TestFileStat(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	// Create file in fake filesystem
	fs.fakeFs.create("/test.txt")
	fs.fakeFs.setSize("/test.txt", 100)

	file := &File{
		Path: "/test.txt",
		Fs:   fs,
	}

	info, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got '%s'", info.Name())
	}
	if info.Size() != 100 {
		t.Errorf("Expected size 100, got %d", info.Size())
	}

	// Test Stat on non-existent file
	file2 := &File{
		Path: "/nonexistent.txt",
		Fs:   fs,
	}

	_, err = file2.Stat()
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

// TestFsName tests filesystem name
func TestFsName(t *testing.T) {
	fs := &Fs{}
	if fs.Name() != "telegram" {
		t.Errorf("Expected name 'telegram', got '%s'", fs.Name())
	}
}

// TestFsMkdir tests Mkdir functionality
func TestFsMkdir(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	err := fs.Mkdir("/testdir", 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	info := fs.fakeFs.stat("/testdir")
	if info == nil {
		t.Fatal("Expected directory to exist")
	}
	if !info.IsDir() {
		t.Error("Expected /testdir to be a directory")
	}
}

// TestFsMkdirAll tests MkdirAll functionality
func TestFsMkdirAll(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	err := fs.MkdirAll("/path/to/deep/dir", 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Check all directories exist
	paths := []string{"/path", "/path/to", "/path/to/deep", "/path/to/deep/dir"}
	for _, path := range paths {
		info := fs.fakeFs.stat(path)
		if info == nil {
			t.Errorf("Expected directory %s to exist", path)
		}
	}
}

// TestFsCreate tests Create functionality
func TestFsCreate(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if file == nil {
		t.Fatal("Expected file to be non-nil")
	}

	info := fs.fakeFs.stat("/test.txt")
	if info == nil {
		t.Error("Expected file to exist in fake filesystem")
	}
}

// TestFsStat tests Stat functionality
func TestFsStat(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	// Test stat on non-existent file
	_, err := fs.Stat("/nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}

	// Create and stat a file
	fs.fakeFs.create("/test.txt")
	fs.fakeFs.setSize("/test.txt", 42)

	info, err := fs.Stat("/test.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Expected name 'test.txt', got '%s'", info.Name())
	}
	if info.Size() != 42 {
		t.Errorf("Expected size 42, got %d", info.Size())
	}
}

// TestFsLstatIfPossible tests LstatIfPossible functionality
func TestFsLstatIfPossible(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	_, supported, err := fs.LstatIfPossible("/test")
	if err == nil {
		t.Error("Expected error for LstatIfPossible")
	}
	if supported {
		t.Error("Expected LstatIfPossible to not be supported")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

// TestFileNotImplementedMethods tests methods that return not implemented
func TestFileNotImplementedMethods(t *testing.T) {
	file := &File{}

	// ReadAt
	_, err := file.ReadAt(nil, 0)
	if err != ErrNotImplemented {
		t.Errorf("Expected ErrNotImplemented, got %v", err)
	}

	// WriteString
	_, err = file.WriteString("test")
	if err != ErrNotImplemented {
		t.Errorf("Expected ErrNotImplemented, got %v", err)
	}

	// WriteAt
	_, err = file.WriteAt(nil, 0)
	if err != ErrNotImplemented {
		t.Errorf("Expected ErrNotImplemented, got %v", err)
	}
}

// TestFileNoOpMethods tests methods that are no-ops
func TestFileNoOpMethods(t *testing.T) {
	file := &File{}

	// Truncate
	err := file.Truncate(0)
	if err != nil {
		t.Errorf("Truncate should return nil, got %v", err)
	}

	// Sync
	err = file.Sync()
	if err != nil {
		t.Errorf("Sync should return nil, got %v", err)
	}

	// Seek
	n, err := file.Seek(0, 0)
	if err != nil {
		t.Errorf("Seek should return nil error, got %v", err)
	}
	if n != 0 {
		t.Errorf("Seek should return 0, got %d", n)
	}

	// Readdir
	infos, err := file.Readdir(0)
	if err != nil {
		t.Errorf("Readdir should return nil error, got %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("Readdir should return empty slice, got %d items", len(infos))
	}

	// Readdirnames
	names, err := file.Readdirnames(0)
	if err != nil {
		t.Errorf("Readdirnames should return nil error, got %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Readdirnames should return empty slice, got %d items", len(names))
	}
}

// TestFsNoOpMethods tests filesystem methods that are no-ops
func TestFsNoOpMethods(t *testing.T) {
	fs := &Fs{}

	// Chtimes
	err := fs.Chtimes("/test", time.Now(), time.Now())
	if err != nil {
		t.Errorf("Chtimes should return nil, got %v", err)
	}

	// Chmod
	err = fs.Chmod("/test", 0755)
	if err != nil {
		t.Errorf("Chmod should return nil, got %v", err)
	}

	// Rename
	err = fs.Rename("/old", "/new")
	if err != nil {
		t.Errorf("Rename should return nil, got %v", err)
	}

	// Chown
	err = fs.Chown("/test", 0, 0)
	if err != nil {
		t.Errorf("Chown should return nil, got %v", err)
	}

	// RemoveAll
	err = fs.RemoveAll("/test")
	if err != nil {
		t.Errorf("RemoveAll should return nil, got %v", err)
	}

	// Remove
	err = fs.Remove("/test")
	if err != nil {
		t.Errorf("Remove should return nil, got %v", err)
	}
}

// TestFsStop tests the Stop method
func TestFsStop(t *testing.T) {
	// Test Stop with nil bot (should not panic and not close channel)
	fs := &Fs{
		stopChan: make(chan struct{}),
		Bot:      nil,
	}

	err := fs.Stop()
	if err != nil {
		t.Errorf("Stop should return nil, got %v", err)
	}

	// With nil Bot, the channel should NOT be closed (this is intentional behavior)
	select {
	case <-fs.stopChan:
		t.Error("Channel should not be closed when Bot is nil")
	default:
		// Good, channel is still open
	}

	// Note: Testing with real Bot is not practical in unit tests
	// as it would require valid Telegram credentials and network access
}

// TestFsStopMultipleCalls tests that Stop() can be called multiple times safely
func TestFsStopMultipleCalls(t *testing.T) {
	// Test that calling Stop() multiple times doesn't panic
	// even when trying to close an already-closed channel
	fs := &Fs{
		Bot:      nil, // nil bot to keep test simple
		Logger:   newTestLogger(),
		stopChan: make(chan struct{}),
	}

	// Call Stop() multiple times
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fs.Stop()
			if err != nil {
				t.Errorf("Stop() returned error: %v", err)
			}
		}()
	}

	wg.Wait()

	// All calls should succeed without panic
	// The channel should remain unclosed (since Bot is nil)
	select {
	case <-fs.stopChan:
		t.Error("Channel should not be closed when Bot is nil")
	default:
		// Good, channel is still open
	}
}

// TestFileWriteSizeLimit tests that Write enforces the file size limit
func TestFileWriteSizeLimit(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file := &File{
		Path: "/large.bin",
		Fs:   fs,
	}

	// Write should succeed for data under the limit
	smallData := make([]byte, 1024)
	n, err := file.Write(smallData)
	if err != nil {
		t.Fatalf("Write of small data failed: %v", err)
	}
	if n != len(smallData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(smallData), n)
	}

	// Try to write data that would exceed the limit
	// maxFileSize is 50MB, so create data that would push us over
	largeData := make([]byte, maxFileSize)
	n, err = file.Write(largeData)
	if err == nil {
		t.Error("Expected error when exceeding file size limit")
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes written on error, got %d", n)
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("Expected ErrFileTooLarge, got %v", err)
	}
}

// TestFileWriteSizeLimitExactly tests writing exactly at the limit
func TestFileWriteSizeLimitExactly(t *testing.T) {
	fs := &Fs{
		fakeFs: newFakeFilesystem(),
	}

	file := &File{
		Path: "/exact.bin",
		Fs:   fs,
	}

	// Write exactly maxFileSize bytes (should succeed)
	data := make([]byte, maxFileSize)
	n, err := file.Write(data)
	if err != nil {
		t.Fatalf("Write of exactly max size failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Try to write one more byte (should fail)
	n, err = file.Write([]byte{0})
	if err == nil {
		t.Error("Expected error when exceeding file size limit by 1 byte")
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("Expected ErrFileTooLarge, got %v", err)
	}
}

// TestConstants tests that constants are defined correctly
func TestConstants(t *testing.T) {
	if maxFileSize != 50*1024*1024 {
		t.Errorf("Expected maxFileSize to be 50MB, got %d", maxFileSize)
	}
	if maxTextSize != 4096 {
		t.Errorf("Expected maxTextSize to be 4096, got %d", maxTextSize)
	}
	if defaultDirSize != 4096 {
		t.Errorf("Expected defaultDirSize to be 4096, got %d", defaultDirSize)
	}
}

// TestErrFileTooLarge tests the error type
func TestErrFileTooLarge(t *testing.T) {
	if ErrFileTooLarge == nil {
		t.Error("ErrFileTooLarge should not be nil")
	}
	if ErrFileTooLarge.Error() == "" {
		t.Error("ErrFileTooLarge should have an error message")
	}
}
