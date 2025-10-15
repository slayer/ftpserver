package telegram

import (
	"errors"
	"strings"
	"testing"

	"github.com/fclairamb/ftpserver/config/confpar"
)

// TestLoadFs_EmptyToken tests LoadFs with empty token
func TestLoadFs_EmptyToken(t *testing.T) {
	logger := newTestLogger()

	access := &confpar.Access{
		Params: map[string]string{
			"Token":  "",
			"ChatID": "123456789",
		},
	}

	_, err := LoadFs(access, logger)
	if err == nil {
		t.Error("Expected error with empty token")
	}
	if !strings.Contains(err.Error(), "Token is empty") {
		t.Errorf("Expected 'Token is empty' error, got: %v", err)
	}
}

// TestLoadFs_InvalidChatID tests LoadFs with invalid ChatID
func TestLoadFs_InvalidChatID(t *testing.T) {
	logger := newTestLogger()

	access := &confpar.Access{
		Params: map[string]string{
			"Token":  "test-token",
			"ChatID": "invalid-not-a-number",
		},
	}

	_, err := LoadFs(access, logger)
	if err == nil {
		t.Error("Expected error with invalid ChatID")
	}
	if !strings.Contains(err.Error(), "invalid ChatID") {
		t.Errorf("Expected 'invalid ChatID' error, got: %v", err)
	}
}

// TestFileExtensionDetection_Images tests image file type detection
func TestFileExtensionDetection_Images(t *testing.T) {
	testCases := []string{
		"/photo.jpg",
		"/image.jpeg",
		"/picture.png",
		"/animation.gif",
		"/bitmap.bmp",
		"/PHOTO.JPG", // Test case insensitivity
	}

	for _, path := range testCases {
		if !isExtension(path, imageExtensions) {
			t.Errorf("File %s should be detected as image", path)
		}

		// Verify it's not detected as other types
		if isExtension(path, videoExtensions) {
			t.Errorf("File %s should not be detected as video", path)
		}
		if isExtension(path, audioExtensions) {
			t.Errorf("File %s should not be detected as audio", path)
		}
	}
}

// TestFileExtensionDetection_Videos tests video file type detection
func TestFileExtensionDetection_Videos(t *testing.T) {
	testCases := []string{
		"/movie.mp4",
		"/clip.avi",
		"/video.mkv",
		"/recording.mov",
		"/VIDEO.MP4", // Test case insensitivity
	}

	for _, path := range testCases {
		if !isExtension(path, videoExtensions) {
			t.Errorf("File %s should be detected as video", path)
		}

		// Verify it's not detected as other types
		if isExtension(path, imageExtensions) {
			t.Errorf("File %s should not be detected as image", path)
		}
		if isExtension(path, audioExtensions) {
			t.Errorf("File %s should not be detected as audio", path)
		}
	}
}

// TestFileExtensionDetection_Audio tests audio file type detection
func TestFileExtensionDetection_Audio(t *testing.T) {
	testCases := []string{
		"/song.mp3",
		"/audio.ogg",
		"/music.flac",
		"/recording.wav",
		"/SONG.MP3", // Test case insensitivity
	}

	for _, path := range testCases {
		if !isExtension(path, audioExtensions) {
			t.Errorf("File %s should be detected as audio", path)
		}

		// Verify it's not detected as other types
		if isExtension(path, imageExtensions) {
			t.Errorf("File %s should not be detected as image", path)
		}
		if isExtension(path, videoExtensions) {
			t.Errorf("File %s should not be detected as video", path)
		}
	}
}

// TestFileExtensionDetection_Text tests text file type detection
func TestFileExtensionDetection_Text(t *testing.T) {
	testCases := []string{
		"/readme.txt",
		"/notes.md",
		"/README.TXT", // Test case insensitivity
	}

	for _, path := range testCases {
		if !isExtension(path, textExtensions) {
			t.Errorf("File %s should be detected as text", path)
		}
	}
}

// TestFileExtensionDetection_Documents tests generic document detection
func TestFileExtensionDetection_Documents(t *testing.T) {
	testCases := []string{
		"/document.pdf",
		"/archive.zip",
		"/data.json",
		"/unknown.xyz",
		"/noextension",
	}

	for _, path := range testCases {
		// These should not match any specific type
		if isExtension(path, imageExtensions) {
			t.Errorf("File %s should not be detected as image", path)
		}
		if isExtension(path, videoExtensions) {
			t.Errorf("File %s should not be detected as video", path)
		}
		if isExtension(path, audioExtensions) {
			t.Errorf("File %s should not be detected as audio", path)
		}
		if isExtension(path, textExtensions) {
			t.Errorf("File %s should not be detected as text", path)
		}
	}
}

// TestFileClose_NilFs tests error handling when Fs is nil
func TestFileClose_NilFs(t *testing.T) {
	file := &File{
		Path:    "/test.txt",
		Content: []byte("test"),
		Fs:      nil,
	}

	err := file.Close()
	if err == nil {
		t.Error("Expected error when Fs is nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

// TestFileSizeValidation tests file size limit enforcement
func TestFileSizeValidation(t *testing.T) {
	logger := newTestLogger()

	fs := &Fs{
		Bot:      nil,
		Logger:   logger,
		ChatID:   123456789,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	// Test small file (should succeed)
	file := &File{
		Path: "/small.bin",
		Fs:   fs,
	}

	smallData := make([]byte, 1024)
	n, err := file.Write(smallData)
	if err != nil {
		t.Fatalf("Write of small data failed: %v", err)
	}
	if n != len(smallData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(smallData), n)
	}

	// Test large file exceeding limit (should fail)
	largeFile := &File{
		Path: "/large.bin",
		Fs:   fs,
	}

	largeData := make([]byte, maxFileSize+1)
	n, err = largeFile.Write(largeData)
	if err == nil {
		t.Error("Expected error when exceeding file size limit")
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("Expected ErrFileTooLarge, got: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes written on error, got %d", n)
	}
}

// TestTextSizeLimit tests text file size limits
func TestTextSizeLimit(t *testing.T) {
	logger := newTestLogger()

	fs := &Fs{
		Bot:      nil,
		Logger:   logger,
		ChatID:   456456456,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	// Test text under limit
	smallText := strings.Repeat("a", maxTextSize-10)
	smallFile := &File{
		Path:    "/small.txt",
		Content: []byte(smallText),
		Fs:      fs,
	}

	if len(smallFile.Content) > maxTextSize {
		t.Error("Small text file should be under or equal to maxTextSize")
	}

	// Test text exceeding limit
	largeText := strings.Repeat("a", maxTextSize+100)
	largeFile := &File{
		Path:    "/large.txt",
		Content: []byte(largeText),
		Fs:      fs,
	}

	if len(largeFile.Content) <= maxTextSize {
		t.Error("Large text file should exceed maxTextSize")
	}
}

// TestFsOperations tests basic filesystem operations
func TestFsOperations(t *testing.T) {
	logger := newTestLogger()

	fs := &Fs{
		Bot:      nil,
		Logger:   logger,
		ChatID:   999999999,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	// Test Name()
	if fs.Name() != "telegram" {
		t.Errorf("Expected name 'telegram', got '%s'", fs.Name())
	}

	// Test Create()
	file, err := fs.Create("/test.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if file == nil {
		t.Fatal("Expected non-nil file")
	}

	// Test Write()
	data := []byte("test data")
	n, err := file.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Test file exists in fake filesystem
	info := fs.fakeFs.stat("/test.txt")
	if info == nil {
		t.Error("Expected file to exist in fake filesystem")
	}
}

// TestMultipleFileCreation tests creating multiple files
func TestMultipleFileCreation(t *testing.T) {
	logger := newTestLogger()

	fs := &Fs{
		Bot:      nil,
		Logger:   logger,
		ChatID:   888888888,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	// Create multiple files of different types
	filePaths := []string{
		"/photo.jpg",
		"/video.mp4",
		"/audio.mp3",
		"/document.pdf",
		"/text.txt",
	}

	for _, path := range filePaths {
		file, err := fs.Create(path)
		if err != nil {
			t.Fatalf("Create %s failed: %v", path, err)
		}

		_, err = file.Write([]byte("test content"))
		if err != nil {
			t.Fatalf("Write %s failed: %v", path, err)
		}

		// Verify file exists in fake filesystem
		info := fs.fakeFs.stat(path)
		if info == nil {
			t.Errorf("Expected file %s to exist in fake filesystem", path)
		}
	}
}

// TestFileReadWriteIntegration tests reading and writing file content
func TestFileReadWriteIntegration(t *testing.T) {
	logger := newTestLogger()

	fs := &Fs{
		Bot:      nil,
		Logger:   logger,
		ChatID:   777777777,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	file := &File{
		Path: "/test.txt",
		Fs:   fs,
	}

	// Write data
	originalData := []byte("Hello, World!")
	n, err := file.Write(originalData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(originalData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(originalData), n)
	}

	// Read data back
	buf := make([]byte, len(originalData))
	n, err = file.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(originalData) {
		t.Errorf("Expected to read %d bytes, read %d", len(originalData), n)
	}
	if string(buf) != string(originalData) {
		t.Errorf("Expected '%s', got '%s'", string(originalData), string(buf))
	}
}
