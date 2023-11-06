// Package telegram provides a telegram access layer
package telegram

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"os"
	"sync/atomic"
	"time"

	log "github.com/fclairamb/go-log"
	tele "gopkg.in/telebot.v3"

	"github.com/spf13/afero"

	"github.com/fclairamb/ftpserver/config/confpar"
	"gopkg.in/telebot.v3/middleware"
)

// ErrNotImplemented is returned when something is not implemented
var ErrNotImplemented = errors.New("not implemented")

// ErrNotFound is returned when something is not found
var ErrNotFound = errors.New("not found")

// ErrInvalidParameter is returned when a parameter is invalid
var ErrInvalidParameter = errors.New("invalid parameter")

// Fs is a write-only afero.Fs implementation using telegram as backend
type Fs struct {
	Bot    *tele.Bot
	ChatID int64
	Logger log.Logger
}

// File is the afero.File implementation
type File struct {
	Path    string
	Content []byte
	Fs      *Fs
	At      int64
}

var imageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif"}
var videoExtensions = []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".mpeg", ".mpg", ".m4v", ".3gp", ".3g2"}
var textExtensions = []string{".txt", ".md"}
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

	// Just to check that the bot is working
	bot.Handle("/hello", func(c tele.Context) error {
		return c.Send("Hello!")
	})

	go func() {
		// Run bot in the background
		bot.Start()
	}()

	fs := &Fs{
		Bot:    bot,
		Logger: logger,
		ChatID: chatID,
	}

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
	} else if isExtension(f.Path, textExtensions) && len(f.Content) < 4096 {
		if isExtension(f.Path, []string{".md"}) {
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

	if err != nil {
		f.Fs.Logger.Error("telegram Bot.Send()", "err", err)
		return err
	}

	f.Content = []byte{}
	f.At = 0

	return nil
}

// Read stores the received file content into the local buffer
func (f *File) Read(b []byte) (int, error) {
	n := 0

	if len(b) > 0 && int(f.At) == len(f.Content) {
		return 0, io.EOF
	}

	if len(f.Content)-int(f.At) >= len(b) {
		n = len(b)
	} else {
		n = len(f.Content) - int(f.At)
	}

	copy(b, f.Content[f.At:f.At+int64(n)])
	atomic.AddInt64(&f.At, int64(n))

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

// Stat is not implemented
func (f *File) Stat() (os.FileInfo, error) {
	f.Fs.Logger.Error("telegram File.Stat() not implemented")
	return nil, ErrNotImplemented
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
	f.Content = append(f.Content, b...)

	return len(b), nil
}

// Name of the filesystem
func (m *Fs) Name() string {
	return "telegram"
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

// Mkdir is not implemented
func (m *Fs) Mkdir(name string, mode os.FileMode) error {
	return nil
}

// MkdirAll is not implemented
func (m *Fs) MkdirAll(name string, mode os.FileMode) error {
	return nil
}

// Open opens a file buffer
func (m *Fs) Open(name string) (afero.File, error) {
	return &File{Path: name, Fs: m}, nil
}

// Create creates a file buffer
func (m *Fs) Create(name string) (afero.File, error) {
	return &File{Path: name, Fs: m}, nil
}

// OpenFile opens a file buffer
func (m *Fs) OpenFile(name string, flag int, mode os.FileMode) (afero.File, error) {
	return &File{Path: name, Fs: m}, nil
}

// Stat is not implemented
func (m *Fs) Stat(name string) (os.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: name, Err: nil}
}

// LstatIfPossible is not implemented
func (m *Fs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	return nil, false, &os.PathError{Op: "lstat", Path: name, Err: nil}
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
