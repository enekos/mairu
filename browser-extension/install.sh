#!/bin/bash
# install.sh
# Registers the native messaging host with Google Chrome for macOS/Linux

set -e

DIR="$( cd "$( dirname "$0" )" && pwd )"
HOST_NAME="com.mairu.browser_context"
TARGET_DIR=""

if [ "$(uname -s)" = "Darwin" ]; then
    TARGET_DIR="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts"
else
    TARGET_DIR="$HOME/.config/google-chrome/NativeMessagingHosts"
fi

mkdir -p "$TARGET_DIR"

# Build the native host binary
echo "Building native host binary..."
cargo build --release -p browser-extension-host
cp target/release/browser-extension-host "$DIR/browser-extension-host"

# Replace this with the actual extension ID when published/loaded
EXTENSION_ID="pknfbcjkpblgldiholapamifojijabge" 

cat << EOF > "$TARGET_DIR/$HOST_NAME.json"
{
  "name": "$HOST_NAME",
  "description": "Mairu Browser Context Native Host",
  "path": "$DIR/browser-extension-host",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://$EXTENSION_ID/"
  ]
}
EOF

echo "Native messaging host installed to $TARGET_DIR/$HOST_NAME.json"
