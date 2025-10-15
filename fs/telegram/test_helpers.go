package telegram

import (
	"sync"
	"testing"

	log "github.com/fclairamb/go-log"
	"github.com/fclairamb/ftpserver/config/confpar"
	tele "gopkg.in/telebot.v3"
)

// testLogger implements log.Logger for testing
type testLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level   string
	message string
	fields  []interface{}
}

func newTestLogger() *testLogger {
	return &testLogger{
		logs: make([]logEntry, 0),
	}
}

func (l *testLogger) Info(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{"info", msg, fields})
}

func (l *testLogger) Error(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{"error", msg, fields})
}

func (l *testLogger) Warn(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{"warn", msg, fields})
}

func (l *testLogger) Debug(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{"debug", msg, fields})
}

func (l *testLogger) Panic(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{"panic", msg, fields})
}

func (l *testLogger) With(fields ...interface{}) log.Logger {
	// Return self for simplicity in tests
	return l
}

func (l *testLogger) hasError() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.logs {
		if entry.level == "error" {
			return true
		}
	}
	return false
}

func (l *testLogger) hasWarn() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.logs {
		if entry.level == "warn" {
			return true
		}
	}
	return false
}

func (l *testLogger) getLogs() []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]logEntry{}, l.logs...)
}

func (l *testLogger) findLog(level, message string) *logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.logs {
		if entry.level == level && entry.message == message {
			return &entry
		}
	}
	return nil
}

// MockBot implements a mock Telegram bot for testing
type MockBot struct {
	sentMessages []sentMessage
	sendError    error
	mu           sync.Mutex
}

type sentMessage struct {
	recipient tele.Recipient
	what      interface{}
	options   []interface{}
}

func newMockBot() *MockBot {
	return &MockBot{
		sentMessages: make([]sentMessage, 0),
	}
}

func (m *MockBot) Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendError != nil {
		return nil, m.sendError
	}

	m.sentMessages = append(m.sentMessages, sentMessage{
		recipient: to,
		what:      what,
		options:   opts,
	})

	// Return a fake successful message
	return &tele.Message{
		ID: len(m.sentMessages),
		Chat: &tele.Chat{ID: to.(*tele.Chat).ID},
	}, nil
}

func (m *MockBot) Start() {}
func (m *MockBot) Stop()  {}

func (m *MockBot) setSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendError = err
}

func (m *MockBot) getSentMessages() []sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sentMessage{}, m.sentMessages...)
}

func (m *MockBot) getLastMessage() *sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sentMessages) == 0 {
		return nil
	}
	return &m.sentMessages[len(m.sentMessages)-1]
}

func (m *MockBot) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = make([]sentMessage, 0)
	m.sendError = nil
}

// createTestBot creates an offline bot for testing
func createTestBot(t *testing.T) *tele.Bot {
	t.Helper()

	bot, err := tele.NewBot(tele.Settings{
		Token:       "test-token-" + t.Name(),
		Offline:     true,
		Synchronous: true,
	})
	if err != nil {
		t.Fatalf("Failed to create test bot: %v", err)
	}

	return bot
}

// createTestAccess creates test access configuration
func createTestAccess(chatID string) *confpar.Access {
	return &confpar.Access{
		Params: map[string]string{
			"Token":  "test-token-12345",
			"ChatID": chatID,
		},
	}
}

// createTestAccessWithToken creates test access with custom token
func createTestAccessWithToken(token, chatID string) *confpar.Access {
	return &confpar.Access{
		Params: map[string]string{
			"Token":  token,
			"ChatID": chatID,
		},
	}
}
