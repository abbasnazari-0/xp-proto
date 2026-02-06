#!/bin/bash

#═══════════════════════════════════════════════════════════════════════════════
#  XP Protocol - Auto Installer for Ubuntu Server (Docker Edition)
#  نصب‌کننده خودکار برای سرور اوبونتو
#═══════════════════════════════════════════════════════════════════════════════

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Config
XP_DIR="/opt/xp-protocol"
XP_USER="xp-proto"
DOCKER_IMAGE="xp-protocol"

#───────────────────────────────────────────────────────────────────────────────
# Functions
#───────────────────────────────────────────────────────────────────────────────

print_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║                                                                   ║"
    echo "║     ██╗  ██╗██████╗     ██████╗ ██████╗  ██████╗ ████████╗ ██████╗║"
    echo "║     ╚██╗██╔╝██╔══██╗    ██╔══██╗██╔══██╗██╔═══██╗╚══██╔══╝██╔═══██╝"
    echo "║      ╚███╔╝ ██████╔╝    ██████╔╝██████╔╝██║   ██║   ██║   ██║   ██║"
    echo "║      ██╔██╗ ██╔═══╝     ██╔═══╝ ██╔══██╗██║   ██║   ██║   ██║   ██║"
    echo "║     ██╔╝ ██╗██║         ██║     ██║  ██║╚██████╔╝   ██║   ╚██████╔╝"
    echo "║     ╚═╝  ╚═╝╚═╝         ╚═╝     ╚═╝  ╚═╝ ╚═════╝    ╚═╝    ╚═════╝ "
    echo "║                                                                   ║"
    echo "║              🛡️  Anti-DPI VPN Protocol Installer                  ║"
    echo "║                                                                   ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "این اسکریپت باید با دسترسی root اجرا بشه!"
        log_info "دستور: sudo bash install.sh"
        exit 1
    fi
}

check_os() {
    if [[ ! -f /etc/os-release ]]; then
        log_error "سیستم‌عامل شناسایی نشد!"
        exit 1
    fi
    
    source /etc/os-release
    if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
        log_warn "این اسکریپت برای Ubuntu/Debian طراحی شده. ادامه می‌دید؟ (y/n)"
        read -r confirm < /dev/tty
        if [[ "$confirm" != "y" ]]; then
            exit 1
        fi
    fi
    log_success "سیستم‌عامل: $PRETTY_NAME"
}

get_user_input() {
    echo ""
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${PURPLE}                    تنظیمات نصب                               ${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    
    # Port
    echo -e "${CYAN}پورت سرور (پیش‌فرض: 443):${NC}"
    read -r input_port < /dev/tty
    SERVER_PORT=${input_port:-443}
    
    # Fake site
    echo -e "${CYAN}سایت جعلی برای Anti-Probe (پیش‌فرض: www.microsoft.com):${NC}"
    read -r input_fake_site < /dev/tty
    FAKE_SITE=${input_fake_site:-www.microsoft.com}
    
    # Transport mode
    echo -e "${CYAN}حالت Transport:${NC}"
    echo "  1) TLS (پیش‌فرض - پیشنهادی)"
    echo "  2) KCP (سریع‌تر، مناسب شبکه‌های با packet loss)"
    echo "  3) Raw (نیاز به تنظیمات بیشتر)"
    read -r input_mode < /dev/tty
    case $input_mode in
        2) TRANSPORT_MODE="kcp" ;;
        3) TRANSPORT_MODE="raw" ;;
        *) TRANSPORT_MODE="tls" ;;
    esac
    
    # Enable BBR
    echo -e "${CYAN}فعال‌سازی BBR برای بهبود سرعت؟ (y/n, پیش‌فرض: y):${NC}"
    read -r input_bbr < /dev/tty
    ENABLE_BBR=${input_bbr:-y}
    
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}خلاصه تنظیمات:${NC}"
    echo -e "  پورت: ${YELLOW}$SERVER_PORT${NC}"
    echo -e "  سایت جعلی: ${YELLOW}$FAKE_SITE${NC}"
    echo -e "  Transport: ${YELLOW}$TRANSPORT_MODE${NC}"
    echo -e "  BBR: ${YELLOW}$ENABLE_BBR${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    
    echo -e "${CYAN}ادامه می‌دید؟ (y/n):${NC}"
    read -r confirm < /dev/tty
    if [[ "$confirm" != "y" ]]; then
        log_info "لغو شد."
        exit 0
    fi
}

install_dependencies() {
    log_info "نصب پیش‌نیازها..."
    
    apt-get update -qq
    apt-get install -y -qq \
        curl \
        wget \
        ca-certificates \
        gnupg \
        lsb-release \
        ufw \
        fail2ban \
        unattended-upgrades
    
    log_success "پیش‌نیازها نصب شدند"
}

install_docker() {
    if command -v docker &> /dev/null; then
        log_success "Docker قبلاً نصب شده"
        return
    fi
    
    log_info "نصب Docker..."
    
    # Add Docker's official GPG key
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    
    # Add repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      tee /etc/apt/sources.list.d/docker.list > /dev/null
    
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    
    systemctl enable docker
    systemctl start docker
    
    log_success "Docker نصب شد"
}

generate_key() {
    log_info "تولید کلید امن..."
    SECRET_KEY=$(openssl rand -base64 32)
    log_success "کلید تولید شد"
}

setup_directories() {
    log_info "ایجاد دایرکتوری‌ها..."
    
    mkdir -p $XP_DIR/{config,logs,data}
    chmod 700 $XP_DIR
    
    log_success "دایرکتوری‌ها ایجاد شدند"
}

create_dockerfile() {
    log_info "ایجاد Dockerfile..."
    
    cat > $XP_DIR/Dockerfile << 'DOCKERFILE'
# XP Protocol Server - Docker Image
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache git gcc musl-dev libpcap-dev

# Set working directory
WORKDIR /build

# Copy source code
COPY . .

# Download dependencies
RUN go mod download

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o xp-server ./cmd/xp-server

# Final image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates libpcap tzdata

# Create non-root user
RUN addgroup -g 1000 xp && adduser -u 1000 -G xp -s /bin/sh -D xp

# Copy binary
COPY --from=builder /build/xp-server /usr/local/bin/

# Create directories
RUN mkdir -p /etc/xp-protocol /var/log/xp-protocol && \
    chown -R xp:xp /etc/xp-protocol /var/log/xp-protocol

# Switch to non-root user
USER xp

# Expose port
EXPOSE 443

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:443 || exit 1

# Run
ENTRYPOINT ["xp-server"]
CMD ["-c", "/etc/xp-protocol/server.yaml"]
DOCKERFILE

    log_success "Dockerfile ایجاد شد"
}

create_docker_compose() {
    log_info "ایجاد docker-compose.yml..."
    
    cat > $XP_DIR/docker-compose.yml << EOF
services:
  xp-server:
    build: .
    container_name: xp-protocol
    restart: unless-stopped
    ports:
      - "${SERVER_PORT}:443"
    volumes:
      - ./config:/etc/xp-protocol:ro
      - ./logs:/var/log/xp-protocol
      - ./data:/var/lib/xp-protocol
    environment:
      - TZ=UTC
    networks:
      - xp-network
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

networks:
  xp-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.28.0.0/16
EOF

    log_success "docker-compose.yml ایجاد شد"
}

create_server_config() {
    log_info "ایجاد فایل کانفیگ سرور..."
    
    cat > $XP_DIR/config/server.yaml << EOF
# XP Protocol Server Configuration
# Generated by installer on $(date)

mode: server

transport:
  mode: ${TRANSPORT_MODE}
  tls:
    fragment: true
    padding: true
    timing_jitter: true
  kcp:
    mode: fast2
    data_shards: 10
    parity_shards: 3

server:
  listen: "0.0.0.0:443"
  key: "${SECRET_KEY}"
  fake_site: "${FAKE_SITE}"
  probe_resist: true
  fallback_site: "${FAKE_SITE}"
  fragment: true
  padding: true
  timing_jitter: true
EOF

    chmod 600 $XP_DIR/config/server.yaml
    log_success "فایل کانفیگ ایجاد شد"
}

copy_source_code() {
    log_info "کپی سورس کد..."
    
    # Copy Go source files
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    cp -r "$SCRIPT_DIR/cmd" $XP_DIR/ 2>/dev/null || true
    cp -r "$SCRIPT_DIR/pkg" $XP_DIR/ 2>/dev/null || true
    cp "$SCRIPT_DIR/go.mod" $XP_DIR/ 2>/dev/null || true
    cp "$SCRIPT_DIR/go.sum" $XP_DIR/ 2>/dev/null || true
    
    log_success "سورس کد کپی شد"
}

setup_firewall() {
    log_info "تنظیم فایروال..."
    
    # Enable UFW
    ufw --force reset
    ufw default deny incoming
    ufw default allow outgoing
    
    # Allow SSH
    ufw allow ssh
    
    # Allow XP Protocol port
    ufw allow ${SERVER_PORT}/tcp
    
    # Enable
    ufw --force enable
    
    log_success "فایروال تنظیم شد"
}

setup_fail2ban() {
    log_info "تنظیم Fail2ban..."
    
    cat > /etc/fail2ban/jail.local << EOF
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 86400
EOF

    systemctl enable fail2ban
    systemctl restart fail2ban
    
    log_success "Fail2ban تنظیم شد"
}

enable_bbr() {
    if [[ "$ENABLE_BBR" != "y" ]]; then
        return
    fi
    
    log_info "فعال‌سازی BBR..."
    
    # Check if BBR is available
    if ! grep -q "tcp_bbr" /proc/modules 2>/dev/null; then
        modprobe tcp_bbr 2>/dev/null || true
    fi
    
    # Enable BBR
    cat >> /etc/sysctl.conf << EOF

# BBR - Enabled by XP Protocol installer
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF

    sysctl -p
    
    log_success "BBR فعال شد"
}

setup_auto_updates() {
    log_info "تنظیم آپدیت خودکار امنیتی..."
    
    cat > /etc/apt/apt.conf.d/50unattended-upgrades << EOF
Unattended-Upgrade::Allowed-Origins {
    "\${distro_id}:\${distro_codename}-security";
};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
EOF

    systemctl enable unattended-upgrades
    
    log_success "آپدیت خودکار تنظیم شد"
}

build_and_start() {
    log_info "ساخت و اجرای Docker container..."
    
    cd $XP_DIR
    
    # Build
    docker compose build --no-cache
    
    # Start
    docker compose up -d
    
    log_success "سرور اجرا شد"
}

create_management_script() {
    log_info "ایجاد اسکریپت مدیریت..."
    
    cat > /usr/local/bin/xp << 'SCRIPT'
#!/bin/bash

XP_DIR="/opt/xp-protocol"

case "$1" in
    start)
        cd $XP_DIR && docker compose up -d
        echo "✓ سرور شروع شد"
        ;;
    stop)
        cd $XP_DIR && docker compose down
        echo "✓ سرور متوقف شد"
        ;;
    restart)
        cd $XP_DIR && docker compose restart
        echo "✓ سرور ریستارت شد"
        ;;
    status)
        docker ps -a | grep xp-protocol
        ;;
    logs)
        cd $XP_DIR && docker compose logs -f
        ;;
    key)
        grep "key:" $XP_DIR/config/server.yaml | awk '{print $2}'
        ;;
    config)
        cat $XP_DIR/config/server.yaml
        ;;
    link)
        echo ""
        echo "🔗 لینک کانفیگ XP Protocol:"
        echo ""
        cat $XP_DIR/config-link.txt
        echo ""
        echo "📋 این لینک رو کپی کن و توی کلاینت import کن"
        echo ""
        ;;
    update)
        cd $XP_DIR && docker compose pull && docker compose up -d
        echo "✓ آپدیت شد"
        ;;
    uninstall)
        echo "آیا مطمئنید؟ (y/n)"
        read confirm < /dev/tty
        if [ "$confirm" = "y" ]; then
            cd $XP_DIR && docker compose down -v
            rm -rf $XP_DIR
            rm /usr/local/bin/xp
            echo "✓ حذف شد"
        fi
        ;;
    *)
        echo "XP Protocol Management"
        echo ""
        echo "Usage: xp <command>"
        echo ""
        echo "Commands:"
        echo "  start      شروع سرور"
        echo "  stop       توقف سرور"
        echo "  restart    ریستارت سرور"
        echo "  status     وضعیت سرور"
        echo "  logs       نمایش لاگ‌ها"
        echo "  key        نمایش کلید"
        echo "  link       نمایش لینک کانفیگ"
        echo "  config     نمایش تنظیمات"
        echo "  update     آپدیت"
        echo "  uninstall  حذف"
        ;;
esac
SCRIPT

    chmod +x /usr/local/bin/xp
    log_success "اسکریپت مدیریت در /usr/local/bin/xp ایجاد شد"
}

generate_client_config() {
    SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s icanhazip.com 2>/dev/null || echo "YOUR_SERVER_IP")
    
    cat > $XP_DIR/client-config.yaml << EOF
# XP Protocol Client Configuration
# کانفیگ کلاینت - این فایل رو به سیستم خودت منتقل کن

mode: client

transport:
  mode: ${TRANSPORT_MODE}
  tls:
    fragment: true
    padding: true
    timing_jitter: true

client:
  server_addr: "${SERVER_IP}:${SERVER_PORT}"
  key: "${SECRET_KEY}"
  fake_sni: "${FAKE_SITE}"
  socks_addr: "127.0.0.1:1080"
  fragment: true
  padding: true
  timing_jitter: true
  fingerprint: "chrome"
EOF

    log_success "کانفیگ کلاینت در $XP_DIR/client-config.yaml ایجاد شد"
}

print_summary() {
    SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s icanhazip.com 2>/dev/null || echo "YOUR_SERVER_IP")
    
    # Generate XP URI (like vless://)
    # Format: xp://KEY@SERVER:PORT?transport=MODE&sni=SITE&fragment=true#NAME
    XP_URI="xp://${SECRET_KEY}@${SERVER_IP}:${SERVER_PORT}?transport=${TRANSPORT_MODE}&sni=${FAKE_SITE}&fragment=true&padding=true&fingerprint=chrome#XP-Server"
    
    # Also generate base64 version for QR code compatibility
    CONFIG_JSON="{\"server\":\"${SERVER_IP}\",\"port\":${SERVER_PORT},\"key\":\"${SECRET_KEY}\",\"transport\":\"${TRANSPORT_MODE}\",\"sni\":\"${FAKE_SITE}\",\"fragment\":true,\"padding\":true,\"fingerprint\":\"chrome\"}"
    XP_BASE64=$(echo -n "$CONFIG_JSON" | base64 | tr -d '\n')
    
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                    ✅ نصب با موفقیت انجام شد!                     ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${PURPLE}        🔗 لینک کانفیگ (کپی کن و توی کلاینت import کن)         ${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${YELLOW}${XP_URI}${NC}"
    echo ""
    
    # Save URI to file
    echo "$XP_URI" > $XP_DIR/config-link.txt
    
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                      اطلاعات اتصال                            ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${YELLOW}آدرس سرور:${NC}  $SERVER_IP"
    echo -e "  ${YELLOW}پورت:${NC}        $SERVER_PORT"
    echo -e "  ${YELLOW}کلید:${NC}        $SECRET_KEY"
    echo -e "  ${YELLOW}Transport:${NC}   $TRANSPORT_MODE"
    echo -e "  ${YELLOW}Fake Site:${NC}   $FAKE_SITE"
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                      دستورات مدیریت                           ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}xp start${NC}     - شروع سرور"
    echo -e "  ${GREEN}xp stop${NC}      - توقف سرور"
    echo -e "  ${GREEN}xp restart${NC}   - ریستارت سرور"
    echo -e "  ${GREEN}xp status${NC}    - وضعیت سرور"
    echo -e "  ${GREEN}xp logs${NC}      - نمایش لاگ‌ها"
    echo -e "  ${GREEN}xp key${NC}       - نمایش کلید"
    echo -e "  ${GREEN}xp link${NC}      - نمایش لینک کانفیگ"
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                      فایل‌های مهم                              ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  کانفیگ سرور:    ${YELLOW}$XP_DIR/config/server.yaml${NC}"
    echo -e "  لینک کانفیگ:    ${YELLOW}$XP_DIR/config-link.txt${NC}"
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  💡 برای دیدن لینک کانفیگ هر وقت: ${YELLOW}xp link${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
}

#───────────────────────────────────────────────────────────────────────────────
# Main
#───────────────────────────────────────────────────────────────────────────────

main() {
    print_banner
    check_root
    check_os
    get_user_input
    
    echo ""
    log_info "شروع نصب..."
    echo ""
    
    install_dependencies
    install_docker
    generate_key
    setup_directories
    copy_source_code
    create_dockerfile
    create_docker_compose
    create_server_config
    setup_firewall
    setup_fail2ban
    enable_bbr
    setup_auto_updates
    create_management_script
    generate_client_config
    build_and_start
    
    print_summary
}

# Run
main "$@"
