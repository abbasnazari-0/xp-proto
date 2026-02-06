# XP Protocol - eXtreme Privacy 🛡️

پروتوکل VPN قدرتمند و مقاوم در برابر DPI (Deep Packet Inspection)

## ویژگی‌ها

### حالت TLS (پیش‌فرض)

- 🔪 **TLS ClientHello Fragmentation** - شکستن پکت‌ها برای دور زدن تشخیص SNI
- 🎭 **Chrome TLS Fingerprint** - تقلید از fingerprint مرورگر Chrome
- 🔐 **ChaCha20-Poly1305** - رمزنگاری سریع و امن
- 📦 **Random Padding** - اضافه کردن noise برای پنهان کردن الگوی ترافیک
- ⏱️ **Timing Obfuscation** - شبیه‌سازی رفتار HTTP عادی
- 🎯 **Anti-Probe** - وقتی کسی سرور رو probe می‌کنه، یه سایت واقعی می‌بینه!

### حالت Raw (Ultimate Stealth! 🥷)

- 🔥 **Raw TCP Packets** - دور زدن کامل TCP stack سیستم‌عامل
- 👻 **Invisible** - هیچ socket در netstat/ss دیده نمیشه
- 🔀 **TCP Flag Rotation** - چرخش بین flagهای مختلف TCP
- 📡 **KCP over Raw** - ترنسپورت reliable روی پکت‌های خام
- 🎲 **FEC (Forward Error Correction)** - تحمل packet loss

---

## 🚀 نصب سریع

### برای سرور خارج (اصلی):

```bash
bash <(curl -sSL https://raw.githubusercontent.com/abbasnazari-0/xp-proto/main/install-online.sh)
```

### برای کلاینت (کاربر عادی):

```bash
bash <(curl -sSL https://raw.githubusercontent.com/abbasnazari-0/xp-proto/main/install-client.sh)
```

### برای سرور ایران (تونل/Relay):

```bash
bash <(curl -sSL https://raw.githubusercontent.com/abbasnazari-0/xp-proto/main/install-relay.sh)
```

---

## 📱 استفاده با لینک کانفیگ

بعد از نصب سرور، یه لینک مثل این دریافت میکنی:

```
xp://SECRET_KEY@SERVER:443?transport=tls&sni=microsoft.com&fragment=true#XP-Server
```

**استفاده:**

```bash
./xp-client -uri "xp://..."
```

---

## 🔀 تونل از سرور ایران (Relay)

اگه IP سرور خارجت بلاک شده، میتونی از سرور ایرانی تونل بزنی:

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  کاربر (ایران)  →  Relay (ایران)  →  XP Server (خارج)       │
│       📱              🔀                   🌍                │
│                       │                                      │
│                  IP اینجا دیده میشه                          │
│                  نه IP سرور خارج!                            │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**نصب Relay روی سرور ایران:**

```bash
bash <(curl -sSL https://raw.githubusercontent.com/abbasnazari-0/xp-proto/main/install-relay.sh)
```

**بعد در کانفیگ کلاینت:**

```
قدیم: server_addr: "IP_KHAREJ:443"
جدید: server_addr: "IP_IRAN:443"
```

---

## 🚀 نصب دستی روی سرور Ubuntu

### چی نصب میشه؟

| مورد            | توضیح                    |
| --------------- | ------------------------ |
| 🐳 Docker       | اجرا در container ایزوله |
| 🔥 UFW Firewall | فقط پورت‌های لازم باز    |
| 🛡️ Fail2ban     | محافظت از SSH            |
| 🚀 BBR          | بهبود سرعت TCP           |
| 🔄 Auto Updates | آپدیت‌های امنیتی خودکار  |

### دستورات مدیریت

```bash
xp start      # شروع سرور
xp stop       # توقف سرور
xp restart    # ریستارت
xp status     # وضعیت
xp logs       # لاگ‌ها
xp key        # نمایش کلید
xp uninstall  # حذف
```

---

## نصب دستی (برای توسعه‌دهندگان)

```bash
# Clone
git clone https://github.com/abbasnazari-0/xp-proto
cd xp

# Build
go build -o bin/xp-server ./cmd/xp-server
go build -o bin/xp-client ./cmd/xp-client
```

## استفاده

### ۱. تولید کلید

```bash
./bin/xp-server -genkey
# Output: 🔑 New key: ABC123...
```

### ۲. تنظیم سرور

فایل `server.yaml`:

```yaml
mode: server

server:
  listen: "0.0.0.0:443"
  key: "کلید_تولید_شده"
  fake_site: "www.microsoft.com"
  probe_resist: true
  fallback_site: "www.microsoft.com"
  fragment: true
  padding: true
  timing_jitter: true
```

اجرا:

```bash
./bin/xp-server -c server.yaml
```

### ۳. تنظیم کلاینت

فایل `client.yaml`:

```yaml
mode: client

client:
  server_addr: "آدرس_سرور:443"
  key: "همان_کلید_سرور"
  fake_sni: "www.microsoft.com"
  socks_addr: "127.0.0.1:1080"
  fragment: true
  padding: true
  timing_jitter: true
  fingerprint: "chrome"
```

اجرا:

```bash
./bin/xp-client -c client.yaml
```

### ۴. استفاده

مرورگر یا برنامه خودتو به SOCKS5 proxy وصل کن:

- **Address:** `127.0.0.1`
- **Port:** `1080`

---

## 🥷 حالت Raw (Ultimate Stealth)

این حالت از پکت‌های خام TCP استفاده می‌کنه و **کاملاً از TCP stack سیستم‌عامل عبور می‌کنه!**

### چرا Raw mode؟

```
┌─────────────────────────────────────────────────────────────┐
│                    حالت عادی (TLS)                          │
├─────────────────────────────────────────────────────────────┤
│  App → Socket → OS TCP Stack → Network                       │
│                     ↑                                        │
│              DPI می‌تونه اینجا                               │
│              connection رو ببینه                            │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    حالت Raw 🥷                              │
├─────────────────────────────────────────────────────────────┤
│  App → XP Protocol → Raw Packet (pcap) → Network             │
│                                                              │
│  ✅ هیچ socket سیستم‌عامل وجود نداره!                         │
│  ✅ netstat/ss هیچی نشون نمیده!                              │
│  ✅ DPI فقط پکت‌های random می‌بینه!                          │
└─────────────────────────────────────────────────────────────┘
```

### نیازمندی‌ها

```bash
# macOS
brew install libpcap

# Ubuntu/Debian
sudo apt-get install libpcap-dev

# RHEL/CentOS
sudo yum install libpcap-devel
```

### پیدا کردن اطلاعات شبکه

```bash
# پیدا کردن interface
ip addr  # Linux
ifconfig # macOS

# پیدا کردن MAC روتر
arp -a | grep gateway
# یا
ip neigh show | grep gateway  # Linux
```

### تنظیم کلاینت Raw

فایل `client-raw.yaml`:

```yaml
mode: client

transport:
  mode: raw # 🥷 Ultimate stealth!

  raw:
    interface: "en0" # eth0 در Linux
    local_ip: "192.168.1.100" # IP خودت
    router_mac: "aa:bb:cc:dd:ee:ff" # MAC روتر
    tcp_flags: ["PA", "A"] # چرخش flag
    use_kcp: true # Reliable transport

client:
  server_addr: "your-server.com:443"
  key: "YOUR_SECRET_KEY"
  socks_addr: "127.0.0.1:1080"
```

### اجرا (نیاز به sudo)

```bash
# Raw mode نیاز به root داره!
sudo ./bin/xp-client -c client-raw.yaml
```

---

## چطور کار می‌کنه؟

```
┌─────────────────────────────────────────────────────────────┐
│                    XP Protocol Flow                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Client                   DPI                    Server      │
│    │                       │                        │        │
│    │──[Fragment1]─────────▶│                        │        │
│    │       └──"www.mic"    │ 🤷 نمی‌فهمه           │        │
│    │──[Fragment2]─────────▶│                        │        │
│    │       └──"rosoft"     │ 🤷 نمی‌فهمه           │        │
│    │──[Fragment3]─────────▶│                        │        │
│    │       └──".com"       │ 🤷 نمی‌فهمه           │        │
│    │                       │                        │        │
│    │◀═══════ TLS Encrypted Tunnel ═════════════════▶│       │
│    │        (به نظر ترافیک عادی Microsoft میاد)      │       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## تکنیک‌های Anti-DPI

### 1. TLS ClientHello Fragmentation

SNI در پکت TLS ClientHello قرار داره. ما این پکت رو به تیکه‌های کوچیک می‌شکنیم:

```
قبل: [ClientHello + SNI = blocked.com] → DPI می‌بلاکه ❌

بعد: [Cli] → [entH] → [ello] → [+SNI] → [=blo] → [cked] → [.com]
                                  ↑
                           DPI گیج میشه! ✅
```

### 2. Chrome TLS Fingerprint

پکت‌های ما دقیقاً شبیه Chrome به نظر میان - cipher suites، extensions، و ترتیب همه چیز.

### 3. Anti-Probe Protection

اگه کسی (مثل censorship system) سرور رو probe کنه، به جای VPN، یه سایت واقعی (مثل Microsoft) می‌بینه!

## مقایسه با بقیه

| ویژگی               | OpenVPN | WireGuard | Xray | **XP Protocol** |
| ------------------- | ------- | --------- | ---- | --------------- |
| Anti-DPI            | ❌      | ❌        | ✅   | ✅✅            |
| Fragmentation       | ❌      | ❌        | ❌   | ✅              |
| Browser Fingerprint | ❌      | ❌        | ⚠️   | ✅              |
| Anti-Probe          | ❌      | ❌        | ⚠️   | ✅              |
| سادگی استفاده       | ❌      | ✅        | ⚠️   | ✅              |

## License

MIT

## هشدار

این ابزار فقط برای اهداف آموزشی و حفظ حریم خصوصی است. استفاده از آن باید مطابق با قوانین محلی شما باشد.
