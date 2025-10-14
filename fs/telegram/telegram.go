// Package telegram provides a telegram access layer
package telegram

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	log "github.com/fclairamb/go-log"
	"github.com/spf13/afero"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"

	"github.com/fclairamb/ftpserver/config/confpar"
)

// ErrNotImplemented is returned when something is not implemented
var ErrNotImplemented = errors.New("not implemented")

// ErrNotFound is returned when something is not found
var ErrNotFound = errors.New("not found")

// ErrInvalidParameter is returned when a parameter is invalid
var ErrInvalidParameter = errors.New("invalid parameter")

// ErrFileTooLarge is returned when a file exceeds Telegram's size limit
var ErrFileTooLarge = errors.New("file size exceeds Telegram limit")

const (
	// maxFileSize is Telegram's file size limit (50MB)
	maxFileSize = 50 * 1024 * 1024
	// maxTextSize is Telegram's message size limit (4096 characters)
	maxTextSize = 4096
)

// Fs is a write-only afero.Fs implementation using telegram as backend
type Fs struct {
	// Bot is the telegram bot instance
	Bot *tele.Bot
	// ChatID is the telegram chat ID to send files to
	ChatID int64
	// Logger is the logger, obviously
	Logger log.Logger

	// fakeFs is a lightweight fake filesystem intended for store temporary info about files
	// since some ftp clients expect to perform mkdir() + stat() on files and directories before upload
	fakeFs *fakeFilesystem

	// stopChan is used to signal bot shutdown
	stopChan chan struct{}
}

// File is the afero.File implementation
type File struct {
	// Path is the file path
	Path string
	// Content is the file content
	Content []byte
	// Fs is the parent Fs
	Fs *Fs
	// At is the current position in the file
	At int64
	// mu protects Content and At from concurrent access
	mu sync.Mutex
}

// imageExtensions is the list of supported image extensions
var imageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif"}

// videoExtensions is the list of supported video extensions
var videoExtensions = []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".mpeg", ".mpg", ".m4v", ".3gp", ".3g2"}

// textExtensions is the list of supported text extensions
var textExtensions = []string{".txt", ".md"}

// audioExtensions is the list of supported audio extensions
var audioExtensions = []string{".mp3", ".ogg", ".flac", ".wav", ".m4a", ".opus"}

// LoadFs loads a file system from an access description
func LoadFs(access *confpar.Access, logger log.Logger) (afero.Fs, error) {

	token := access.Params["Token"]
	if token == "" {
		return nil, fmt.Errorf("parameter Token is empty")
	}

	chatID, err := strconv.ParseInt(access.Params["ChatID"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ChatID parameter: %v", err)
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		logger.Error("telegram bot initialization", "err", err)
		return nil, err
	}
	bot.Use(middleware.Logger())
	bot.Use(middleware.AutoRespond())

	bot.Handle("/start", startHandler)
	bot.Handle("/help", helpHandler)

	fs := &Fs{
		Bot:      bot,
		Logger:   logger,
		ChatID:   chatID,
		fakeFs:   newFakeFilesystem(),
		stopChan: make(chan struct{}),
	}

	go func() {
		// Run bot in the background
		logger.Info("Starting telegram bot")
		bot.Start()
		logger.Info("Telegram bot stopped")
	}()

	return fs, nil
}

// Name of the file
func (f *File) Name() string { return f.Path }

// Close closes the file transfer and does the actual transfer to telegram
func (f *File) Close() error {
	if f.Fs == nil {
		return ErrNotFound
	}

	chat := tele.Chat{ID: f.Fs.ChatID}
	var err error
	basePath := filepath.Base(f.Path)

	if isExtension(f.Path, imageExtensions) {
		photo := tele.Photo{File: tele.FromReader(f), Caption: basePath}
		_, err = f.Fs.Bot.Send(&chat, &photo)
	} else if isExtension(f.Path, videoExtensions) {
		video := tele.Video{File: tele.FromReader(f), Caption: basePath}
		_, err = f.Fs.Bot.Send(&chat, &video)
	} else if isExtension(f.Path, audioExtensions) {
		audio := tele.Audio{File: tele.FromReader(f), Caption: basePath}
		_, err = f.Fs.Bot.Send(&chat, &audio)
	} else if isExtension(f.Path, textExtensions) && len(f.Content) < maxTextSize {
		// Validate UTF-8 for text files
		if !utf8.Valid(f.Content) {
			f.Fs.Logger.Warn("Invalid UTF-8 in text file, sending as document", "path", f.Path)
			document := tele.Document{File: tele.FromReader(f), Caption: basePath}
			document.FileName = basePath
			document.FileLocal = basePath
			_, err = f.Fs.Bot.Send(&chat, &document)
		} else if isExtension(f.Path, []string{".md"}) {
			_, err = f.Fs.Bot.Send(&chat, string(f.Content), tele.ModeMarkdown)
		} else {
			_, err = f.Fs.Bot.Send(&chat, string(f.Content))
		}
	} else {
		document := tele.Document{File: tele.FromReader(f), Caption: basePath}
		document.FileName = basePath
		document.FileLocal = basePath
		_, err = f.Fs.Bot.Send(&chat, &document)
	}
	f.Fs.Logger.Info("telegram Bot.Send()", "path", f.Path)

	if err != nil {
		f.Fs.Logger.Error("telegram Bot.Send()", "err", err)
		return err
	}

	f.Fs.fakeFs.create(f.Path)
	f.Fs.fakeFs.setSize(f.Path, int64(len(f.Content)))

	f.Content = []byte{}
	f.At = 0

	return nil
}

// Read stores the received file content into the local buffer
func (f *File) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(b) > 0 && int(f.At) == len(f.Content) {
		return 0, io.EOF
	}

	n := len(b)
	if len(f.Content)-int(f.At) < n {
		n = len(f.Content) - int(f.At)
	}

	copy(b, f.Content[f.At:f.At+int64(n)])
	f.At += int64(n)

	return n, nil
}

// ReadAt is not implemented
func (f *File) ReadAt(_ []byte, _ int64) (int, error) {
	return 0, ErrNotImplemented
}

// Truncate is not implemented
func (f *File) Truncate(_ int64) error {
	return nil
}

// Readdir is not implemented
func (f *File) Readdir(_ int) ([]os.FileInfo, error) {
	return []os.FileInfo{}, nil
}

// Readdirnames is not implemented
func (f *File) Readdirnames(_ int) ([]string, error) {
	return []string{}, nil
}

// Seek is not implemented
func (f *File) Seek(_ int64, _ int) (int64, error) {
	return 0, nil
}

// Stat for the file relies on the fake filesystem
func (f *File) Stat() (os.FileInfo, error) {
	fileInfo := f.Fs.fakeFs.stat(f.Path)

	if fileInfo == nil {
		return nil, &os.PathError{Op: "stat", Path: f.Path, Err: os.ErrNotExist}
	}
	return fileInfo, nil
}

// Sync is not implemented
func (f *File) Sync() error {
	return nil
}

// WriteString is not implemented
func (f *File) WriteString(s string) (int, error) {
	return 0, ErrNotImplemented
}

// WriteAt is not implemented
func (f *File) WriteAt(b []byte, off int64) (int, error) {
	return 0, ErrNotImplemented
}

func (f *File) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if writing would exceed Telegram's file size limit
	newSize := int64(len(f.Content)) + int64(len(b))
	if newSize > maxFileSize {
		return 0, fmt.Errorf("%w: attempted size %d bytes, limit is %d bytes", ErrFileTooLarge, newSize, maxFileSize)
	}

	f.Content = append(f.Content, b...)

	return len(b), nil
}

// Name of the filesystem
func (m *Fs) Name() string {
	return "telegram"
}

// Stop gracefully shuts down the telegram bot
func (m *Fs) Stop() error {
	if m.Bot != nil {
		m.Logger.Info("Stopping telegram bot")
		m.Bot.Stop()
		close(m.stopChan)
	}
	return nil
}

// Chtimes is not implemented
func (m *Fs) Chtimes(name string, atime, mtime time.Time) error {
	return nil
}

// Chmod is not implemented
func (m *Fs) Chmod(name string, mode os.FileMode) error {
	return nil
}

// Rename is not implemented
func (m *Fs) Rename(name string, newname string) error {
	return nil
}

// Chown is not implemented
func (m *Fs) Chown(string, int, int) error {
	return nil
}

// RemoveAll is not implemented
func (m *Fs) RemoveAll(name string) error {
	return nil
}

// Remove is not implemented
func (m *Fs) Remove(name string) error {
	return nil
}

// Mkdir
func (m *Fs) Mkdir(name string, mode os.FileMode) error {
	m.fakeFs.mkdir(name, mode)
	return nil
}

// MkdirAll creates full path of directories
// like mkdir -p
func (m *Fs) MkdirAll(name string, mode os.FileMode) error {
	path := strings.Split(name, "/")
	for i := 0; i < len(path); i++ {
		dir := strings.Join(path[:i+1], "/")
		m.fakeFs.mkdir(dir, mode)
	}
	return nil
}

// Open opens a file buffer
func (m *Fs) Open(name string) (afero.File, error) {
	return &File{Path: name, Fs: m}, nil
}

// Create creates a file buffer
func (m *Fs) Create(name string) (afero.File, error) {
	m.fakeFs.create(name)
	return &File{Path: name, Fs: m}, nil
}

// OpenFile opens a file buffer
func (m *Fs) OpenFile(name string, flag int, mode os.FileMode) (afero.File, error) {
	return &File{Path: name, Fs: m}, nil
}

// Stat() fake implementation
func (m *Fs) Stat(name string) (os.FileInfo, error) {
	fileInfo := m.fakeFs.stat(name)

	if fileInfo == nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}
	return fileInfo, nil
}

// LstatIfPossible is not implemented
func (m *Fs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	return nil, false, &os.PathError{Op: "lstat", Path: name, Err: os.ErrNotExist}
}

func isExtension(filename string, extensions []string) bool {
	extension := strings.ToLower(filepath.Ext(filename))
	for _, ext := range extensions {
		if extension == ext {
			return true
		}
	}
	return false
}

const readMeURL = "https://github.com/slayer/ftpserver"

// /start command handler
// Note: This handler responds to any user. For production use, consider adding
// authorization checks to restrict access to specific chat IDs only.
func startHandler(c tele.Context) error {
	err := helpHandler(c)
	if err != nil {
		return err
	}
	var chatID int64
	chat := c.Chat()
	if chat != nil {
		chatID = chat.ID
	}

	// Warning: This reveals the chat ID to any user who sends /start
	// In a production environment, consider restricting this information
	err = c.Send(fmt.Sprintf("Current `ChatID` is `%d`", chatID), tele.ModeMarkdown)
	return err
}

// helpHandler responds to /help command
// Note: This handler responds to any user. For production use, consider adding
// authorization checks to restrict access to specific chat IDs only.
func helpHandler(c tele.Context) error {
	firstName := "<unknown>"
	if c.Sender() != nil {
		firstName = c.Sender().FirstName
	}
	message := fmt.Sprintf("Hello %s!, you can read more about me at %s", firstName, readMeURL)
	return c.Send(message)
}
