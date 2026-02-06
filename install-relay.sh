#!/bin/bash

#═══════════════════════════════════════════════════════════════════════════════
#  XP Protocol - Relay Server Installer (سرور ایران)
#  نصب سرور واسط برای تونل زدن
#
#  معماری:
#  کاربر (ایران) → Relay Server (ایران) → XP Server (خارج)
#
#  مزیت:
#  - IP سرور خارج مخفی میمونه
#  - فقط IP سرور ایران دیده میشه
#  - اگه سرور خارج بلاک شد، فقط Relay رو عوض میکنی
#═══════════════════════════════════════════════════════════════════════════════

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

XP_DIR="/opt/xp-relay"

print_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║          XP Protocol - Relay Server Installer                     ║"
    echo "║               🔀 Bridge • Tunnel • Stealth                        ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo ""
    echo -e "${YELLOW}این سرور رو روی سرور ایرانی نصب کن${NC}"
    echo -e "${YELLOW}ترافیک از اینجا به سرور خارجی تونل میشه${NC}"
    echo ""
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        echo -e "${RED}[✗]${NC} نیاز به دسترسی root داره!"
        echo "sudo bash install-relay.sh"
        exit 1
    fi
}

get_config() {
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${PURPLE}                      تنظیمات Relay                            ${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    
    echo -e "${CYAN}پورت Listen (پیش‌فرض: 443):${NC}"
    read -r input_port < /dev/tty
    LISTEN_PORT=${input_port:-443}
    
    echo -e "${CYAN}آدرس سرور خارجی (مثال: 1.2.3.4:443):${NC}"
    read -r TARGET_ADDR < /dev/tty
    
    if [[ -z "$TARGET_ADDR" ]]; then
        echo -e "${RED}[✗]${NC} آدرس سرور خارجی الزامی است!"
        exit 1
    fi
    
    echo ""
    echo -e "${GREEN}تنظیمات:${NC}"
    echo -e "  Listen: ${YELLOW}0.0.0.0:$LISTEN_PORT${NC}"
    echo -e "  Target: ${YELLOW}$TARGET_ADDR${NC}"
    echo ""
    
    echo -e "${CYAN}ادامه؟ (y/n):${NC}"
    read -r confirm < /dev/tty
    if [[ "$confirm" != "y" ]]; then
        exit 0
    fi
}

install_relay() {
    echo -e "${BLUE}[*]${NC} نصب Relay..."
    
    mkdir -p $XP_DIR
    
    # Check if Go is installed
    if command -v go &> /dev/null; then
        echo -e "${BLUE}[*]${NC} بیلد از سورس..."
        
        TEMP_DIR=$(mktemp -d)
        git clone --depth 1 https://github.com/abbasnazari-0/xp-proto.git "$TEMP_DIR"
        cd "$TEMP_DIR"
        go build -o $XP_DIR/xp-relay ./cmd/xp-relay
        cd -
        rm -rf "$TEMP_DIR"
    else
        echo -e "${BLUE}[*]${NC} دانلود باینری..."
        
        # Download pre-built binary
        ARCH=$(uname -m)
        case $ARCH in
            x86_64) ARCH="amd64";;
            aarch64) ARCH="arm64";;
        esac
        
        curl -sSL "https://github.com/abbasnazari-0/xp-proto/releases/latest/download/xp-relay-linux-${ARCH}" -o $XP_DIR/xp-relay || {
            echo -e "${RED}[✗]${NC} دانلود نشد. Go رو نصب کن:"
            echo "apt install golang-go"
            exit 1
        }
    fi
    
    chmod +x $XP_DIR/xp-relay
    echo -e "${GREEN}[✓]${NC} Relay نصب شد"
}

create_service() {
    echo -e "${BLUE}[*]${NC} ایجاد سرویس systemd..."
    
    cat > /etc/systemd/system/xp-relay.service << EOF
[Unit]
Description=XP Protocol Relay Server
After=network.target

[Service]
Type=simple
ExecStart=$XP_DIR/xp-relay -l 0.0.0.0:${LISTEN_PORT} -t ${TARGET_ADDR}
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable xp-relay
    systemctl start xp-relay
    
    echo -e "${GREEN}[✓]${NC} سرویس شروع شد"
}

setup_firewall() {
    echo -e "${BLUE}[*]${NC} تنظیم فایروال..."
    
    if command -v ufw &> /dev/null; then
        ufw allow $LISTEN_PORT/tcp
        echo -e "${GREEN}[✓]${NC} پورت $LISTEN_PORT باز شد"
    fi
}

create_management() {
    cat > /usr/local/bin/xp-relay << 'EOF'
#!/bin/bash
case "$1" in
    status) systemctl status xp-relay;;
    start) systemctl start xp-relay;;
    stop) systemctl stop xp-relay;;
    restart) systemctl restart xp-relay;;
    logs) journalctl -u xp-relay -f;;
    *) echo "Usage: xp-relay {start|stop|restart|status|logs}";;
esac
EOF
    chmod +x /usr/local/bin/xp-relay
}

print_summary() {
    RELAY_IP=$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_RELAY_IP")
    
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                    ✅ نصب Relay کامل شد!                          ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                      معماری تونل                              ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  کاربر (ایران) → ${YELLOW}$RELAY_IP:$LISTEN_PORT${NC} → ${GREEN}$TARGET_ADDR${NC}"
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  ⚠️  مهم: در کانفیگ کلاینت، آدرس سرور رو عوض کن:            ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${YELLOW}قدیم:${NC} server_addr: \"$TARGET_ADDR\""
    echo -e "  ${GREEN}جدید:${NC} server_addr: \"$RELAY_IP:$LISTEN_PORT\""
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                      دستورات مدیریت                           ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}xp-relay status${NC}   - وضعیت"
    echo -e "  ${GREEN}xp-relay restart${NC}  - ریستارت"
    echo -e "  ${GREEN}xp-relay logs${NC}     - لاگ‌ها"
    echo ""
}

# Main
main() {
    print_banner
    check_root
    get_config
    install_relay
    create_service
    setup_firewall
    create_management
    print_summary
}

main "$@"
