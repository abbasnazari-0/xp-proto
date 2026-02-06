#!/bin/bash

#═══════════════════════════════════════════════════════════════════════════════
#  XP Protocol - Uninstaller
#  حذف کامل
#═══════════════════════════════════════════════════════════════════════════════

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

XP_DIR="/opt/xp-protocol"

echo -e "${RED}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                    XP Protocol Uninstaller                         ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}Error: Run with sudo${NC}"
    exit 1
fi

echo -e "${YELLOW}این عمل غیرقابل برگشت است!${NC}"
echo -e "آیا مطمئنید که می‌خواهید XP Protocol را حذف کنید؟ (yes/no)"
read -r confirm < /dev/tty

if [[ "$confirm" != "yes" ]]; then
    echo "لغو شد."
    exit 0
fi

echo "[*] Stopping containers..."
cd $XP_DIR 2>/dev/null && docker compose down -v 2>/dev/null || true

echo "[*] Removing Docker image..."
docker rmi xp-protocol 2>/dev/null || true

echo "[*] Removing files..."
rm -rf $XP_DIR
rm -f /usr/local/bin/xp

echo "[*] Removing firewall rules..."
ufw delete allow 443/tcp 2>/dev/null || true

echo ""
echo -e "${GREEN}[✓] XP Protocol با موفقیت حذف شد${NC}"
