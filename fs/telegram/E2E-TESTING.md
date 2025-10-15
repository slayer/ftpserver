# End-to-End Testing for Telegram FTP Server

This directory contains end-to-end testing tools for the Telegram filesystem backend.

## Overview

The E2E test suite:
- ✅ Builds and starts the FTP server
- ✅ Creates test files (images, videos, text, documents)
- ✅ Uploads files via FTP client
- ✅ Verifies files are sent to Telegram
- ✅ Automatically cleans up after testing

## Prerequisites

### 1. Install FTP Client

**macOS:**
```bash
brew install lftp
```

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install lftp
```

**Linux (RHEL/CentOS):**
```bash
sudo yum install lftp
```

### 2. Configure Telegram Bot

You need a Telegram bot token and chat ID.

#### Create a Telegram Bot:
1. Open Telegram and search for `@BotFather`
2. Send `/newbot` command
3. Follow instructions to create a bot
4. Copy the bot token (format: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

#### Get Your Chat ID:
1. Send a message to your bot
2. Visit: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
3. Look for `"chat":{"id":123456789}` in the response
4. Copy the chat ID number

#### Update Configuration:
Edit `e2e-test.conf` and replace the token and chat ID:
```json
{
  "version": 1,
  "accesses": [
    {
      "fs": "telegram",
      "shared": true,
      "user": "test",
      "pass": "test",
      "params": {
        "Token": "YOUR_BOT_TOKEN_HERE",
        "ChatID": "YOUR_CHAT_ID_HERE"
      }
    }
  ],
  "passive_transfer_port_range": {
    "start": 2122,
    "end": 2130
  }
}
```

### 3. Optional: Install ImageMagick

For better test image generation:

**macOS:**
```bash
brew install imagemagick
```

**Linux:**
```bash
sudo apt-get install imagemagick
```

**Note:** The script works without ImageMagick but creates a minimal PNG file instead.

## Running the E2E Test

### Quick Start

```bash
cd fs/telegram
./e2e-test.sh
```

### What the Script Does

1. **Prerequisites Check**
   - Verifies `lftp` is installed
   - Checks configuration file exists
   - Validates Telegram token and chat ID are set

2. **Build Server**
   - Compiles the FTP server binary
   - Exits if build fails

3. **Create Test Files**
   - `test-image.png` - Test image (sent as Photo)
   - `test-text.txt` - Plain text file (sent as Text message)
   - `test-readme.md` - Markdown file (sent as formatted Markdown)
   - `test-video.mp4` - Video file (sent as Video/Document)
   - `test-data.json` - JSON document (sent as Document)

4. **Start FTP Server**
   - Starts server on port 2121
   - Uses configuration from `e2e-test.conf`
   - Runs in background

5. **Upload Files**
   - Connects to FTP server
   - Uploads all test files
   - Reports success/failure

6. **Verify Uploads**
   - Prompts you to check Telegram chat
   - Lists expected files
   - Asks for manual confirmation

7. **Cleanup**
   - Stops FTP server
   - Removes test files
   - Runs automatically on script exit

## Test Files Created

| File | Type | Sent As | Description |
|------|------|---------|-------------|
| `test-image.png` | Image | Photo | 100x100 blue image with text |
| `test-text.txt` | Text | Text Message | Multi-line text with UTF-8 |
| `test-readme.md` | Markdown | Formatted Text | Markdown with formatting |
| `test-video.mp4` | Video | Video/Document | Small test video file |
| `test-data.json` | JSON | Document | JSON test data |

## Expected Results

After running the test, check your Telegram chat. You should see:

1. **Photo** - Blue square image with "Test Image" text
2. **Text message** - Plain text content from test-text.txt
3. **Formatted message** - Markdown-rendered content with bold/italic
4. **Video or Document** - test-video.mp4 file
5. **Document** - test-data.json file

## Troubleshooting

### "lftp: command not found"
**Solution:** Install lftp (see Prerequisites)

### "Server failed to start"
**Possible causes:**
- Port 2121 is already in use
- Invalid Telegram token
- Invalid chat ID
- Network connectivity issues

**Debug:**
```bash
# Check if port is in use
lsof -i :2121

# Test server manually
go run . --conf fs/telegram/e2e-test.conf
```

### "Upload failed"
**Possible causes:**
- Server not running
- Firewall blocking connection
- Invalid credentials in config

**Debug:**
```bash
# Test FTP connection manually
lftp -u test,test localhost:2121
```

### "No files in Telegram"
**Possible causes:**
- Bot not started (send `/start` to your bot)
- Wrong chat ID
- Invalid bot token
- Bot doesn't have permission to send messages

**Debug:**
```bash
# Test Telegram API
curl "https://api.telegram.org/bot<YOUR_TOKEN>/getMe"

# Test sending message
curl -X POST "https://api.telegram.org/bot<YOUR_TOKEN>/sendMessage" \
     -d "chat_id=<YOUR_CHAT_ID>" \
     -d "text=Test from curl"
```

## Manual Testing

If you want to test manually without the script:

### 1. Start Server
```bash
cd /path/to/ftpserver
go build
./ftpserver --conf fs/telegram/e2e-test.conf
```

### 2. Connect with FTP Client
```bash
lftp -u test,test localhost:2121
```

### 3. Upload Files
```bash
# In lftp prompt:
put /path/to/image.jpg
put /path/to/document.pdf
```

### 4. Check Telegram
Open your Telegram chat and verify files were received.

## Automated Testing in CI/CD

**Note:** This E2E test requires real Telegram credentials and is **not suitable for CI/CD** by default.

For CI/CD, consider:
1. Using unit and integration tests (see `telegram_test.go`)
2. Setting up a dedicated test bot with secrets management
3. Running E2E tests in a separate nightly build

## Cleaning Up

The script automatically cleans up on exit, but if interrupted:

```bash
# Kill server manually
pkill ftpserver

# Remove test files
rm -rf fs/telegram/e2e-test-files
```

## Security Notes

⚠️ **Important:**
- **Never commit** your real bot token to git
- The `e2e-test.conf` file contains placeholder credentials
- Replace with your own token before testing
- Consider using environment variables for tokens:

```bash
# Set token via environment variable
export TELEGRAM_BOT_TOKEN="your-token-here"
export TELEGRAM_CHAT_ID="your-chat-id-here"

# Modify script to use env vars if needed
```

## Contributing

When modifying the E2E test:
1. Test with your own Telegram bot first
2. Ensure cleanup happens on all exit paths
3. Add new test files to `create_test_files()` function
4. Update this documentation with new test cases

## Related Documentation

- [README.md](README.md) - Telegram filesystem documentation
- [telegram_test.go](telegram_test.go) - Unit tests
- [telegram_integration_test.go](telegram_integration_test.go) - Integration tests
