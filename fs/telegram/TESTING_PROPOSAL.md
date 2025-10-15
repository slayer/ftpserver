# Telegram Integration Testing Proposal

## Executive Summary

This document proposes comprehensive testing strategies for the Telegram filesystem backend. Based on analysis of `gopkg.in/telebot.v3` documentation and testing patterns, we can implement robust testing without requiring actual Telegram credentials or network access.

---

## Current Testing Gap

**What we have:**
- ✅ Unit tests for fake filesystem
- ✅ Unit tests for File operations (read/write)
- ✅ Unit tests for helper functions
- ✅ 50% code coverage

**What we're missing:**
- ❌ Bot initialization testing
- ❌ Message sending (Photo, Video, Document, Audio)
- ❌ Command handler testing (/start, /help)
- ❌ Error handling from Telegram API
- ❌ Integration testing with telebot library

---

## Testing Strategies

### Strategy 1: Offline Bot Testing (Recommended)

**Key Discovery:** Telebot supports `Offline: true` mode for testing!

```go
// From telebot documentation
bot, err := tele.NewBot(tele.Settings{
    Token:  "test-token",  // Can be any string in offline mode
    Offline: true,         // Skips getMe API call
})
```

#### Benefits:
- ✅ No network required
- ✅ No credentials required
- ✅ Fast execution
- ✅ Deterministic results
- ✅ CI/CD friendly

#### Implementation:

```go
// fs/telegram/telegram_integration_test.go

func TestLoadFs_Offline(t *testing.T) {
    // Create a test logger
    logger := &testLogger{}

    access := &confpar.Access{
        Params: map[string]string{
            "Token":  "test-token-offline",
            "ChatID": "123456789",
        },
    }

    // This should work in offline mode
    fs, err := LoadFs(access, logger)
    if err != nil {
        t.Fatalf("LoadFs failed: %v", err)
    }

    // Verify bot was created
    if fs.(*Fs).Bot == nil {
        t.Error("Bot should not be nil")
    }

    // Verify ChatID was set correctly
    if fs.(*Fs).ChatID != 123456789 {
        t.Errorf("Expected ChatID 123456789, got %d", fs.(*Fs).ChatID)
    }

    // Clean up
    fs.(*Fs).Stop()
}
```

---

### Strategy 2: Mock Bot Interface

Create a mock bot that implements the minimal interface we need.

```go
// fs/telegram/telegram_mock_test.go

type MockBot struct {
    sentMessages []sentMessage
    sendError    error
    mu           sync.Mutex
}

type sentMessage struct {
    recipient interface{}
    what      interface{}
    options   []interface{}
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
        Chat: &tele.Chat{ID: to.Recipient()},
    }, nil
}

func (m *MockBot) Start() {}
func (m *MockBot) Stop()  {}

func (m *MockBot) getSentMessages() []sentMessage {
    m.mu.Lock()
    defer m.mu.Unlock()
    return append([]sentMessage{}, m.sentMessages...)
}
```

#### Usage:

```go
func TestFileClose_WithMockBot(t *testing.T) {
    mockBot := &MockBot{}
    logger := &testLogger{}

    fs := &Fs{
        Bot:    mockBot,
        Logger: logger,
        ChatID: 123456789,
        fakeFs: newFakeFilesystem(),
    }

    file := &File{
        Path:    "/test.jpg",
        Content: []byte("fake image data"),
        Fs:      fs,
    }

    // Test Close() which triggers Send()
    err := file.Close()
    if err != nil {
        t.Fatalf("Close failed: %v", err)
    }

    // Verify Send was called
    messages := mockBot.getSentMessages()
    if len(messages) != 1 {
        t.Errorf("Expected 1 message sent, got %d", len(messages))
    }

    // Verify it was sent as a photo
    msg := messages[0]
    if _, ok := msg.what.(*tele.Photo); !ok {
        t.Errorf("Expected Photo, got %T", msg.what)
    }
}
```

---

### Strategy 3: Test Poller Pattern

Use telebot's test poller pattern for command handler testing.

```go
// fs/telegram/telegram_handlers_test.go

type testPoller struct {
    updates chan tele.Update
    done    chan struct{}
}

func newTestPoller() *testPoller {
    return &testPoller{
        updates: make(chan tele.Update, 10),
        done:    make(chan struct{}, 1),
    }
}

func (p *testPoller) Poll(b *tele.Bot, updates chan tele.Update, stop chan struct{}) {
    for {
        select {
        case upd := <-p.updates:
            updates <- upd
        case <-stop:
            return
        default:
        }
    }
}

func TestBotCommands(t *testing.T) {
    logger := &testLogger{}

    bot, err := tele.NewBot(tele.Settings{
        Token:       "test-token",
        Offline:     true,
        Synchronous: true, // Process updates synchronously for testing
    })
    if err != nil {
        t.Fatal(err)
    }

    // Register our handlers
    bot.Handle("/start", startHandler)
    bot.Handle("/help", helpHandler)

    // Create test context
    tp := newTestPoller()
    bot.Poller = tp

    // Send a /start command
    go func() {
        tp.updates <- tele.Update{
            Message: &tele.Message{
                Text: "/start",
                Chat: &tele.Chat{ID: 123456789},
                Sender: &tele.User{
                    ID:        987654321,
                    FirstName: "TestUser",
                },
            },
        }
    }()

    // Start bot and wait for processing
    go bot.Start()
    time.Sleep(100 * time.Millisecond)
    bot.Stop()

    // Verify handlers were called (would need to add instrumentation)
}
```

---

### Strategy 4: Integration Tests with ProcessUpdate

Use `ProcessUpdate` to simulate Telegram updates directly.

```go
// fs/telegram/telegram_update_test.go

func TestProcessUpdate_PhotoMessage(t *testing.T) {
    logger := &testLogger{}

    bot, err := tele.NewBot(tele.Settings{
        Token:       "test-token",
        Offline:     true,
        Synchronous: true,
    })
    if err != nil {
        t.Fatal(err)
    }

    var received bool
    bot.Handle(tele.OnPhoto, func(c tele.Context) error {
        received = true
        assert.NotNil(t, c.Message().Photo)
        return nil
    })

    // Simulate receiving a photo
    bot.ProcessUpdate(tele.Update{
        Message: &tele.Message{
            Photo: &tele.Photo{
                File: tele.File{FileID: "test-photo-id"},
            },
        },
    })

    if !received {
        t.Error("Photo handler was not called")
    }
}
```

---

### Strategy 5: Error Simulation

Test error handling from Telegram API.

```go
// fs/telegram/telegram_error_test.go

func TestFileClose_TelegramError(t *testing.T) {
    mockBot := &MockBot{
        sendError: tele.ErrTooLarge, // Simulate Telegram error
    }

    logger := &testLogger{}

    fs := &Fs{
        Bot:    mockBot,
        Logger: logger,
        ChatID: 123456789,
        fakeFs: newFakeFilesystem(),
    }

    file := &File{
        Path:    "/large.bin",
        Content: make([]byte, 60*1024*1024), // 60MB, exceeds limit
        Fs:      fs,
    }

    err := file.Close()
    if err == nil {
        t.Error("Expected error for oversized file")
    }

    // Verify error was logged
    if !logger.hasError() {
        t.Error("Error should have been logged")
    }
}
```

---

## Proposed Test Structure

```
fs/telegram/
├── telegram.go               # Main implementation
├── fake_fs.go                # Fake filesystem
├── file_info.go              # FileInfo implementation
├── telegram_test.go          # Existing unit tests
├── telegram_integration_test.go  # NEW: Integration tests with offline bot
├── telegram_mock_test.go     # NEW: Mock bot implementation
├── telegram_handlers_test.go # NEW: Command handler tests
├── telegram_error_test.go    # NEW: Error handling tests
└── test_helpers.go           # NEW: Shared test utilities
```

---

## Test Helpers to Implement

```go
// fs/telegram/test_helpers.go

// testLogger implements log.Logger for testing
type testLogger struct {
    logs   []logEntry
    mu     sync.Mutex
}

type logEntry struct {
    level   string
    message string
    fields  []interface{}
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

func (l *testLogger) With(fields ...interface{}) log.Logger {
    return l // Simplified for testing
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

func (l *testLogger) getLogs() []logEntry {
    l.mu.Lock()
    defer l.mu.Unlock()
    return append([]logEntry{}, l.logs...)
}

// createTestBot creates a bot for testing
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
            "Token":  "test-token",
            "ChatID": chatID,
        },
    }
}
```

---

## Coverage Goals

With these strategies, we can achieve:

| Component | Current | Target | Strategy |
|-----------|---------|--------|----------|
| LoadFs() | 0% | 90% | Offline bot |
| File.Close() | 0% | 85% | Mock bot |
| Bot handlers | 0% | 80% | ProcessUpdate |
| Error paths | 30% | 90% | Error simulation |
| **Overall** | **50%** | **75-80%** | Combined |

---

## Implementation Priority

### Phase 1: Foundation (High Priority)
1. ✅ Create `test_helpers.go` with testLogger and utilities
2. ✅ Implement offline bot tests for LoadFs()
3. ✅ Test bot initialization and configuration

### Phase 2: Core Functionality (High Priority)
4. ✅ Create MockBot implementation
5. ✅ Test File.Close() with different file types
6. ✅ Test UTF-8 validation logic
7. ✅ Test file size limit enforcement

### Phase 3: Command Handlers (Medium Priority)
8. ⏳ Test /start and /help handlers
9. ⏳ Verify ChatID responses
10. ⏳ Test command handler security

### Phase 4: Error Handling (Medium Priority)
11. ⏳ Test Telegram API errors
12. ⏳ Test network failures
13. ⏳ Test invalid configurations

### Phase 5: Advanced (Low Priority)
14. ⏳ Test concurrent file uploads
15. ⏳ Test bot lifecycle (Start/Stop)
16. ⏳ Performance testing

---

## Example: Complete Integration Test

```go
// fs/telegram/telegram_integration_test.go

func TestTelegramFs_EndToEnd(t *testing.T) {
    // Setup
    logger := &testLogger{}
    access := createTestAccess("123456789")

    // Create filesystem (uses offline bot)
    fs, err := LoadFs(access, logger)
    if err != nil {
        t.Fatalf("LoadFs failed: %v", err)
    }
    defer fs.(*Fs).Stop()

    telegramFs := fs.(*Fs)

    // Replace bot with mock for Send() testing
    mockBot := &MockBot{}
    telegramFs.Bot = mockBot

    // Test 1: Create and write file
    file, err := telegramFs.Create("/test.jpg")
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }

    imageData := []byte("fake jpg data")
    n, err := file.Write(imageData)
    if err != nil {
        t.Fatalf("Write failed: %v", err)
    }
    if n != len(imageData) {
        t.Errorf("Expected to write %d bytes, wrote %d", len(imageData), n)
    }

    // Test 2: Close triggers Send to Telegram
    err = file.Close()
    if err != nil {
        t.Fatalf("Close failed: %v", err)
    }

    // Test 3: Verify message was sent
    messages := mockBot.getSentMessages()
    if len(messages) != 1 {
        t.Fatalf("Expected 1 message, got %d", len(messages))
    }

    // Test 4: Verify correct message type
    photo, ok := messages[0].what.(*tele.Photo)
    if !ok {
        t.Errorf("Expected Photo, got %T", messages[0].what)
    }

    // Test 5: Verify caption
    if photo.Caption != "test.jpg" {
        t.Errorf("Expected caption 'test.jpg', got '%s'", photo.Caption)
    }

    // Test 6: Verify file exists in fake filesystem
    info, err := telegramFs.Stat("/test.jpg")
    if err != nil {
        t.Errorf("Stat failed: %v", err)
    }
    if info.Size() != int64(len(imageData)) {
        t.Errorf("Expected size %d, got %d", len(imageData), info.Size())
    }

    // Test 7: Verify logging
    logs := logger.getLogs()
    var foundSendLog bool
    for _, log := range logs {
        if log.level == "info" && log.message == "telegram Bot.Send()" {
            foundSendLog = true
            break
        }
    }
    if !foundSendLog {
        t.Error("Expected Send() to be logged")
    }
}
```

---

## Benefits of This Approach

1. **No External Dependencies**
   - Tests run without internet
   - No Telegram credentials needed
   - Fast CI/CD pipeline

2. **Comprehensive Coverage**
   - Tests all code paths
   - Simulates real scenarios
   - Catches edge cases

3. **Maintainable**
   - Clear test structure
   - Reusable helpers
   - Well-documented patterns

4. **Reliable**
   - Deterministic results
   - No flaky tests
   - Easy to debug

---

## Limitations & Considerations

### What We CAN'T Test:
- ❌ Actual message delivery to Telegram
- ❌ Telegram API rate limiting
- ❌ Network latency issues
- ❌ Real file upload behavior
- ❌ Telegram server-side validation

### What We CAN Test:
- ✅ Bot initialization logic
- ✅ File type detection
- ✅ Size limit enforcement
- ✅ Error handling paths
- ✅ Command handlers
- ✅ UTF-8 validation
- ✅ Concurrency safety
- ✅ Resource cleanup

### Workarounds:
For actual Telegram integration:
- Manual testing with test bot
- Separate E2E test suite (optional, requires credentials)
- Staging environment testing

---

## Conclusion

By leveraging telebot's `Offline` mode and mock patterns, we can achieve **75-80% code coverage** for the Telegram filesystem backend without requiring actual Telegram credentials or network access. This approach provides fast, reliable, and maintainable tests suitable for CI/CD environments.

**Next Steps:**
1. Review and approve this proposal
2. Implement Phase 1 (Foundation)
3. Iteratively add remaining phases
4. Document testing patterns for future contributors
