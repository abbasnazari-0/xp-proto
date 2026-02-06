#!/bin/bash

#═══════════════════════════════════════════════════════════════════════════════
#  XP Protocol - Easy Client Installer (برای کاربر عادی)
#  نصب و اجرای کلاینت با یک دستور
#═══════════════════════════════════════════════════════════════════════════════

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

XP_DIR="$HOME/.xp-protocol"
BINARY="$XP_DIR/xp-client"

print_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║          XP Protocol Client - Easy Setup                          ║"
    echo "║               🛡️ Anti-DPI • Stealth • Fast                        ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

detect_os() {
    OS="unknown"
    ARCH="unknown"
    
    case "$(uname -s)" in
        Linux*)  OS="linux";;
        Darwin*) OS="darwin";;
        MINGW*|CYGWIN*|MSYS*) OS="windows";;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64) ARCH="amd64";;
        arm64|aarch64) ARCH="arm64";;
        armv7l) ARCH="arm";;
    esac
    
    echo -e "${GREEN}[✓]${NC} سیستم‌عامل: $OS ($ARCH)"
}

download_binary() {
    echo -e "${BLUE}[*]${NC} دانلود کلاینت..."
    
    mkdir -p "$XP_DIR"
    
    # Download URL (replace with actual release URL)
    DOWNLOAD_URL="https://github.com/abbasnazari-0/xp-proto/releases/latest/download/xp-client-${OS}-${ARCH}"
    
    if command -v curl &> /dev/null; then
        curl -sSL "$DOWNLOAD_URL" -o "$BINARY" 2>/dev/null || {
            echo -e "${YELLOW}[!]${NC} دانلود نشد. بیلد محلی..."
            build_from_source
            return
        }
    elif command -v wget &> /dev/null; then
        wget -q "$DOWNLOAD_URL" -O "$BINARY" 2>/dev/null || {
            echo -e "${YELLOW}[!]${NC} دانلود نشد. بیلد محلی..."
            build_from_source
            return
        }
    fi
    
    chmod +x "$BINARY"
    echo -e "${GREEN}[✓]${NC} کلاینت دانلود شد"
}

build_from_source() {
    echo -e "${BLUE}[*]${NC} بیلد از سورس..."
    
    # Check Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}[✗]${NC} Go نصب نیست!"
        echo -e "${YELLOW}نصب Go: https://go.dev/dl/${NC}"
        exit 1
    fi
    
    # Clone and build
    TEMP_DIR=$(mktemp -d)
    git clone --depth 1 https://github.com/abbasnazari-0/xp-proto.git "$TEMP_DIR" 2>/dev/null
    cd "$TEMP_DIR"
    go build -o "$BINARY" ./cmd/xp-client
    cd - > /dev/null
    rm -rf "$TEMP_DIR"
    
    chmod +x "$BINARY"
    echo -e "${GREEN}[✓]${NC} کلاینت بیلد شد"
}

get_config() {
    echo ""
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${PURPLE}                      تنظیمات اتصال                            ${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    
    echo -e "${CYAN}لینک کانفیگ (xp://...) رو وارد کن:${NC}"
    read -r XP_URI < /dev/tty
    
    if [[ ! "$XP_URI" =~ ^xp:// ]]; then
        echo -e "${RED}[✗]${NC} لینک نامعتبر! باید با xp:// شروع بشه"
        exit 1
    fi
    
    # Save URI
    echo "$XP_URI" > "$XP_DIR/config-uri.txt"
    echo -e "${GREEN}[✓]${NC} کانفیگ ذخیره شد"
}

create_launcher() {
    # Create simple launcher script
    cat > "$XP_DIR/start.sh" << 'EOF'
#!/bin/bash
XP_DIR="$HOME/.xp-protocol"
URI=$(cat "$XP_DIR/config-uri.txt")
"$XP_DIR/xp-client" -uri "$URI"
EOF
    chmod +x "$XP_DIR/start.sh"
    
    # Create desktop shortcut for Linux
    if [[ "$OS" == "linux" ]] && [[ -d "$HOME/.local/share/applications" ]]; then
        cat > "$HOME/.local/share/applications/xp-protocol.desktop" << EOF
[Desktop Entry]
Name=XP Protocol
Comment=Anti-DPI VPN Client
Exec=$XP_DIR/start.sh
Icon=network-vpn
Terminal=true
Type=Application
Categories=Network;
EOF
    fi
    
    # macOS - create app alias
    if [[ "$OS" == "darwin" ]]; then
        echo "alias xp-connect='$XP_DIR/start.sh'" >> "$HOME/.zshrc" 2>/dev/null || true
        echo "alias xp-connect='$XP_DIR/start.sh'" >> "$HOME/.bashrc" 2>/dev/null || true
    fi
}

print_usage() {
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}                    ✅ نصب کامل شد!                            ${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${CYAN}برای اتصال:${NC}"
    echo ""
    echo -e "  ${YELLOW}$XP_DIR/start.sh${NC}"
    echo ""
    echo -e "  یا:"
    echo ""
    echo -e "  ${YELLOW}$BINARY -uri 'xp://...'${NC}"
    echo ""
    echo -e "${CYAN}تنظیمات پراکسی در مرورگر/سیستم:${NC}"
    echo ""
    echo -e "  SOCKS5: ${YELLOW}127.0.0.1:1080${NC}"
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Main
main() {
    print_banner
    detect_os
    download_binary
    get_config
    create_launcher
    print_usage
}

main "$@"
