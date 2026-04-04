#!/usr/bin/env bash
# Bootstrap script for mairu on Linux or macOS without Docker
set -euo pipefail

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

die() {
  log_error "$1"
  exit 1
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  echo "Usage: ./bootstrap.sh"
  echo "Bootstraps Mairu on a Unix machine without Docker."
  exit 0
fi

# 1. System checks
OS="$(uname -s)"
ARCH="$(uname -m)"
log_info "Detected OS: ${OS}, Architecture: ${ARCH}"

if [[ "$OS" != "Linux" && "$OS" != "Darwin" ]]; then
  die "This script only supports Linux and macOS. Detected: $OS"
fi

# 2. Dependency checks
check_dependency() {
  local cmd=$1
  local name=$2
  local install_url=$3

  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_warning "$name is not installed or not in PATH."
    log_info "Please install $name: $install_url"
    return 1
  fi
  log_success "$name is installed ($(command -v "$cmd"))"
  return 0
}

DEPS_MISSING=0
check_dependency bun "Bun (JavaScript runtime)" "https://bun.sh/" || DEPS_MISSING=1
check_dependency go "Go (Programming language)" "https://go.dev/doc/install" || DEPS_MISSING=1

if [[ $DEPS_MISSING -eq 1 ]]; then
  die "Please install the missing dependencies and run this script again."
fi

# 3. Environment Setup
log_info "Setting up environment configuration..."
if [[ ! -f ".env" ]]; then
  if [[ -f ".env.example" ]]; then
    cp .env.example .env
    log_success "Created .env from .env.example"
  else
    die ".env.example not found. Are you in the root of the mairu project?"
  fi
else
  log_info ".env file already exists, keeping existing configuration."
fi

# 4. Gemini API Key Configuration
if grep -q "GEMINI_API_KEY=" .env; then
  # Extract the value, remove quotes if present
  CURRENT_KEY=$(grep "^GEMINI_API_KEY=" .env | cut -d '=' -f2 | tr -d '"'\'' ')
  
  if [[ -z "$CURRENT_KEY" || "$CURRENT_KEY" == "your_gemini_api_key" ]]; then
    echo -e "\n${YELLOW}To use mairu effectively, you need a Gemini API Key.${NC}"
    echo "You can get one at: https://aistudio.google.com/app/apikey"
    read -rp "Enter your Gemini API Key (or press Enter to skip and configure later): " api_key
    
    if [[ -n "$api_key" ]]; then
      # Replace the key in the .env file
      if [[ "$OS" == "Darwin" ]]; then
        sed -i '' "s|^GEMINI_API_KEY=.*|GEMINI_API_KEY=${api_key}|" .env
      else
        sed -i "s|^GEMINI_API_KEY=.*|GEMINI_API_KEY=${api_key}|" .env
      fi
      log_success "Gemini API key configured in .env"
    else
      log_warning "Skipped Gemini API key setup. Remember to set it in .env later!"
    fi
  else
    log_info "Gemini API key already configured."
  fi
fi

# 5. Build and Installation
log_info "Installing TypeScript dependencies..."
bun install || die "Failed to install dependencies with bun"

log_info "Installing Dashboard dependencies..."
bun install --cwd mairu/ui || die "Failed to install dashboard dependencies"

# 6. Start local Meilisearch and setup
log_info "Setting up Meilisearch locally (no Docker)..."
make meili-up || die "Failed to start local Meilisearch"

# Wait a moment for Meilisearch to fully start up and be ready for API calls
sleep 2

log_info "Initializing Meilisearch indexes..."
make setup || die "Failed to initialize Meilisearch indexes"

# 7. Build Go CLI/Agent
log_info "Building the mairu Go agent binary..."
make mairu-build || die "Failed to build the mairu binary"

log_success "Mairu bootstrap complete!"

echo ""
echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}             Mairu Successfully Installed!          ${NC}"
echo -e "${GREEN}====================================================${NC}"
echo ""
echo "You can now use Mairu locally without Docker."
echo ""
echo "Commands to try:"
echo -e "  ${BLUE}make dashboard${NC}       - Start the context server & unified web UI"
echo -e "  ${BLUE}make mairu-web${NC}       - Start the Mairu agent web UI"
echo -e "  ${BLUE}make dev-no-docker${NC}   - Start local Meilisearch & dashboard"
echo -e "  ${BLUE}./mairu/bin/mairu-agent tui${NC} - Launch the Mairu terminal UI"
echo ""
echo "To manage the local Meilisearch instance:"
echo -e "  ${BLUE}make meili-status${NC}    - Check if it's running"
echo -e "  ${BLUE}make meili-down${NC}      - Stop Meilisearch"
echo ""
echo "Enjoy using Mairu!"
