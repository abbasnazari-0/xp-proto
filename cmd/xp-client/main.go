package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abbasnazari-0/xp-proto/pkg/config"
	"github.com/abbasnazari-0/xp-proto/pkg/crypto"
	"github.com/abbasnazari-0/xp-proto/pkg/obfs"
	xtls "github.com/abbasnazari-0/xp-proto/pkg/tls"
	"github.com/abbasnazari-0/xp-proto/pkg/tunnel"
)

var (
	configPath = flag.String("c", "config.yaml", "Path to config file")
	genKey     = flag.Bool("genkey", false, "Generate a new key")
	genConfig  = flag.Bool("genconfig", false, "Generate example config")
)

func main() {
	flag.Parse()

	if *genKey {
		key, err := crypto.GenerateKey()
		if err != nil {
			fmt.Printf("❌ Failed to generate key: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🔑 New key: %s\n", base64.StdEncoding.EncodeToString(key))
		return
	}

	if *genConfig {
		fmt.Println(config.GenerateExampleConfig("client"))
		return
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		fmt.Println("💡 Run with -genconfig to generate example config")
		os.Exit(1)
	}

	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║       XP Protocol Client v1.0             ║")
	fmt.Println("║   🛡️  Anti-DPI • Stealth • Fast           ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	client := NewXPClient(cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n👋 Shutting down...")
		client.Stop()
		os.Exit(0)
	}()

	if err := client.Start(); err != nil {
		fmt.Printf("❌ Client error: %v\n", err)
		os.Exit(1)
	}
}

type XPClient struct {
	config     *config.Config
	key        []byte
	socks5     *tunnel.SOCKS5Server
	fragmenter *obfs.Fragmenter
}

func NewXPClient(cfg *config.Config) *XPClient {
	key, err := cfg.Client.GetKey()
	if err != nil {
		fmt.Printf("⚠️  Invalid key\n")
		os.Exit(1)
	}

	fragConfig := obfs.DefaultFragmentConfig()
	fragConfig.Enabled = cfg.Client.Fragment

	return &XPClient{
		config:     cfg,
		key:        key,
		socks5:     tunnel.NewSOCKS5Server(cfg.Client.SOCKSAddr),
		fragmenter: obfs.NewFragmenter(fragConfig),
	}
}

func (c *XPClient) Start() error {
	fmt.Printf("🔗 Connecting to %s\n", c.config.Client.ServerAddr)
	fmt.Printf("🎭 SNI: %s\n", c.config.Client.FakeSNI)
	fmt.Printf("🧦 SOCKS5 proxy: %s\n", c.config.Client.SOCKSAddr)
	fmt.Printf("🔧 Fragmentation: %v | Fingerprint: %s\n",
		c.config.Client.Fragment, c.config.Client.Fingerprint)
	fmt.Println()

	conn, err := c.connectToServer()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Println("✅ Connected to server!")
	fmt.Println()

	tun, err := tunnel.NewTunnel(conn, c.key)
	if err != nil {
		return fmt.Errorf("failed to create tunnel: %w", err)
	}

	c.socks5.SetTunnel(tun)

	fmt.Printf("🚀 SOCKS5 proxy ready on %s\n", c.config.Client.SOCKSAddr)
	fmt.Println()
	fmt.Println("💡 Configure your browser/apps to use:")
	fmt.Printf("   SOCKS5: %s\n", c.config.Client.SOCKSAddr)
	fmt.Println()

	return c.socks5.Start()
}

func (c *XPClient) connectToServer() (net.Conn, error) {
	_ = xtls.NewClientHelloBuilder(c.config.Client.FakeSNI)

	tlsConfig := &tls.Config{
		ServerName:         c.config.Client.FakeSNI,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
	}

	if c.config.Client.Fragment {
		return c.dialWithFragmentation(c.config.Client.ServerAddr, tlsConfig)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.config.Client.ServerAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS connection failed: %w", err)
	}
	return conn, nil
}

func (c *XPClient) dialWithFragmentation(addr string, tlsConfig *tls.Config) (*tls.Conn, error) {
	tcpConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	fragConn := &fragmentingConn{
		Conn:       tcpConn,
		fragmenter: c.fragmenter,
		firstWrite: true,
	}

	tlsConn := tls.Client(fragConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}

type fragmentingConn struct {
	net.Conn
	fragmenter *obfs.Fragmenter
	firstWrite bool
}

func (fc *fragmentingConn) Write(b []byte) (int, error) {
	if fc.firstWrite {
		fc.firstWrite = false
		fmt.Println("🔪 Fragmenting TLS ClientHello...")
		err := fc.fragmenter.FragmentTLSClientHello(fc.Conn, b)
		if err != nil {
			return 0, err
		}
		return len(b), nil
	}
	return fc.Conn.Write(b)
}

func (c *XPClient) Stop() {
	c.socks5.Stop()
}
