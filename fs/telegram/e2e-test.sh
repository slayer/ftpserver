#!/bin/bash
#
# End-to-End Test Script for Telegram FTP Server
#
# This script:
# 1. Builds the FTP server
# 2. Starts the server with e2e-test.conf
# 3. Creates test files (image, video, text, document)
# 4. Uploads files via FTP client
# 5. Verifies upload success
# 6. Cleans up
#
# Prerequisites:
# - lftp (FTP client) - install with: brew install lftp (macOS) or apt-get install lftp (Linux)
# - Valid Telegram bot token and chat ID in e2e-test.conf
# - ImageMagick (optional, for creating test image) - brew install imagemagick
#
# Usage:
#   ./e2e-test.sh
#

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_FILE="$SCRIPT_DIR/e2e-test.conf"
TEST_DIR="$SCRIPT_DIR/e2e-test-files"
FTP_HOST="localhost"
FTP_PORT="2121"
FTP_USER="test"
FTP_PASS="test"
SERVER_PID=""

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"

    # Kill server if running
    if [ -n "$SERVER_PID" ]; then
        echo "Stopping FTP server (PID: $SERVER_PID)..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi

    # Remove test files
    if [ -d "$TEST_DIR" ]; then
        echo "Removing test files..."
        rm -rf "$TEST_DIR"
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set up trap to cleanup on exit
trap cleanup EXIT INT TERM

# Print section header
print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"

    # Check if lftp is installed
    if ! command -v lftp &> /dev/null; then
        echo -e "${RED}ERROR: lftp is not installed${NC}"
        echo "Install with:"
        echo "  macOS:  brew install lftp"
        echo "  Linux:  sudo apt-get install lftp"
        exit 1
    fi
    echo -e "${GREEN}✓ lftp is installed${NC}"

    # Check if config file exists
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${RED}ERROR: Config file not found: $CONFIG_FILE${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Config file found${NC}"

    # Check if Telegram token is set (not default)
    TOKEN=$(grep -o '"Token": "[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    if [ -z "$TOKEN" ] || [ ${#TOKEN} -lt 20 ]; then
        echo -e "${YELLOW}WARNING: Telegram token may not be configured properly${NC}"
        echo "Edit $CONFIG_FILE and set your bot token"
    else
        echo -e "${GREEN}✓ Telegram token configured${NC}"
    fi

    # Check if ChatID is set
    CHAT_ID=$(grep -o '"ChatID": "[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    if [ -z "$CHAT_ID" ]; then
        echo -e "${YELLOW}WARNING: ChatID not configured${NC}"
        echo "Edit $CONFIG_FILE and set your chat ID"
    else
        echo -e "${GREEN}✓ ChatID configured${NC}"
    fi
}

# Build the FTP server
build_server() {
    print_header "Building FTP Server"

    cd "$PROJECT_ROOT"
    echo "Building ftpserver..."
    go build -o ftpserver .

    if [ ! -f "ftpserver" ]; then
        echo -e "${RED}ERROR: Build failed${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ Server built successfully${NC}"
}

# Create test files
create_test_files() {
    print_header "Creating Test Files"

    mkdir -p "$TEST_DIR"

    # 1. Create a simple test image (PNG)
    echo "Creating test image..."
    if command -v convert &> /dev/null; then
        # Use ImageMagick if available
        convert -size 100x100 xc:blue -pointsize 20 -fill white \
                -gravity center -annotate +0+0 "Test\nImage" \
                "$TEST_DIR/test-image.png"
        echo -e "${GREEN}✓ Created test-image.png (using ImageMagick)${NC}"
    else
        # Create a minimal valid PNG file (1x1 red pixel)
        printf '\x89\x50\x4e\x47\x0d\x0a\x1a\x0a\x00\x00\x00\x0d\x49\x48\x44\x52' > "$TEST_DIR/test-image.png"
        printf '\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90\x77\x53' >> "$TEST_DIR/test-image.png"
        printf '\xde\x00\x00\x00\x0c\x49\x44\x41\x54\x08\xd7\x63\xf8\xcf\xc0\x00' >> "$TEST_DIR/test-image.png"
        printf '\x00\x03\x01\x01\x00\x18\xdd\x8d\xb4\x00\x00\x00\x00\x49\x45\x4e' >> "$TEST_DIR/test-image.png"
        printf '\x44\xae\x42\x60\x82' >> "$TEST_DIR/test-image.png"
        echo -e "${GREEN}✓ Created test-image.png (minimal PNG)${NC}"
    fi

    # 2. Create a text file
    cat > "$TEST_DIR/test-text.txt" << 'EOF'
This is a test text file for E2E testing.

It contains:
- Multiple lines
- UTF-8 characters: ñ, ü, 中文
- Numbers: 123456
- Symbols: !@#$%^&*()

This file will be sent to Telegram via FTP.
EOF
    echo -e "${GREEN}✓ Created test-text.txt${NC}"

    # 3. Create a markdown file
    cat > "$TEST_DIR/test-readme.md" << 'EOF'
# E2E Test Readme

This is a **markdown** file for testing.

## Features
- *Italic text*
- **Bold text**
- `Code blocks`

## Testing
This file tests markdown rendering in Telegram.
EOF
    echo -e "${GREEN}✓ Created test-readme.md${NC}"

    # 4. Create a small "video" file (actually just a text file with .mp4 extension for testing)
    echo "Fake video file for E2E testing" > "$TEST_DIR/test-video.mp4"
    echo -e "${GREEN}✓ Created test-video.mp4${NC}"

    # 5. Create a JSON document
    cat > "$TEST_DIR/test-data.json" << 'EOF'
{
  "test": "e2e",
  "timestamp": "2025-10-14",
  "files": ["image", "video", "text", "json"],
  "success": true
}
EOF
    echo -e "${GREEN}✓ Created test-data.json${NC}"

    echo ""
    echo "Test files created in: $TEST_DIR"
    ls -lh "$TEST_DIR"
}

# Start FTP server
start_server() {
    print_header "Starting FTP Server"

    cd "$PROJECT_ROOT"
    echo "Starting server on port $FTP_PORT..."
    echo "Using config: $CONFIG_FILE"

    # Start server in background
    ./ftpserver --conf "$CONFIG_FILE" &
    SERVER_PID=$!

    echo "Server PID: $SERVER_PID"

    # Wait for server to start
    echo "Waiting for server to start..."
    sleep 3

    # Check if server is running
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        echo -e "${RED}ERROR: Server failed to start${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ Server started successfully${NC}"
}

# Upload files via FTP
upload_files() {
    print_header "Uploading Files via FTP"

    echo "Connecting to FTP server..."
    echo "Host: $FTP_HOST:$FTP_PORT"
    echo "User: $FTP_USER"

    # Use lftp to upload files
    lftp -c "
        set ftp:ssl-allow no
        set net:timeout 10
        set net:max-retries 2
        open -u $FTP_USER,$FTP_PASS $FTP_HOST:$FTP_PORT
        echo 'Connected to FTP server'
        echo 'Uploading files...'
        lcd $TEST_DIR
        mput *.png
        mput *.txt
        mput *.md
        mput *.mp4
        mput *.json
        echo 'Upload complete'
        quit
    "

    UPLOAD_STATUS=$?

    if [ $UPLOAD_STATUS -eq 0 ]; then
        echo -e "${GREEN}✓ Files uploaded successfully${NC}"
        return 0
    else
        echo -e "${RED}✗ Upload failed with status: $UPLOAD_STATUS${NC}"
        return 1
    fi
}

# Verify uploads
verify_uploads() {
    print_header "Verifying Uploads"

    echo "Files sent to Telegram:"
    ls -1 "$TEST_DIR"

    echo ""
    echo -e "${YELLOW}Please check your Telegram chat for the uploaded files.${NC}"
    echo ""
    echo "Expected files in Telegram:"
    echo "  1. test-image.png (as Photo)"
    echo "  2. test-text.txt (as Text message)"
    echo "  3. test-readme.md (as Markdown message)"
    echo "  4. test-video.mp4 (as Video or Document)"
    echo "  5. test-data.json (as Document)"

    echo ""
    read -p "Did all files appear in Telegram? (y/n): " -n 1 -r
    echo

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${GREEN}✓ Manual verification: SUCCESS${NC}"
        return 0
    else
        echo -e "${RED}✗ Manual verification: FAILED${NC}"
        return 1
    fi
}

# Main execution
main() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════╗"
    echo "║    Telegram FTP Server E2E Test        ║"
    echo "╚════════════════════════════════════════╝"
    echo -e "${NC}"

    check_prerequisites
    build_server
    create_test_files
    start_server

    # Give server a moment to fully initialize
    sleep 2

    upload_files
    UPLOAD_RESULT=$?

    if [ $UPLOAD_RESULT -eq 0 ]; then
        verify_uploads
        VERIFY_RESULT=$?
    else
        echo -e "${RED}Skipping verification due to upload failure${NC}"
        VERIFY_RESULT=1
    fi

    # Final report
    print_header "Test Summary"

    if [ $UPLOAD_RESULT -eq 0 ] && [ $VERIFY_RESULT -eq 0 ]; then
        echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║          E2E TEST PASSED ✓             ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
        exit 0
    else
        echo -e "${RED}╔════════════════════════════════════════╗${NC}"
        echo -e "${RED}║          E2E TEST FAILED ✗             ║${NC}"
        echo -e "${RED}╚════════════════════════════════════════╝${NC}"
        exit 1
    fi
}

# Run main function
main
