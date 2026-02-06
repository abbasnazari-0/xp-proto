#!/bin/bash

#═══════════════════════════════════════════════════════════════════════════════
#  XP Protocol - One-Line Installer
#  نصب با یک دستور از اینترنت
#  
#  Usage: curl -sSL https://raw.githubusercontent.com/xp-proto/xp/main/install-online.sh | sudo bash
#═══════════════════════════════════════════════════════════════════════════════

set -e

REPO_URL="https://github.com/xp-proto/xp"
INSTALL_DIR="/tmp/xp-install"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║          XP Protocol - Online Installer                           ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Error: Run with sudo${NC}"
    exit 1
fi

# Install git if needed
if ! command -v git &> /dev/null; then
    echo "[*] Installing git..."
    apt-get update -qq && apt-get install -y -qq git
fi

# Clone repo
echo "[*] Downloading XP Protocol..."
rm -rf $INSTALL_DIR
git clone --depth 1 $REPO_URL $INSTALL_DIR

# Run installer
echo "[*] Running installer..."
cd $INSTALL_DIR
chmod +x install.sh
./install.sh

# Cleanup
rm -rf $INSTALL_DIR

echo -e "${GREEN}[✓] Done!${NC}"
