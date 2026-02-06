package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/abbasnazari-0/xp-proto/pkg/config"
	"github.com/abbasnazari-0/xp-proto/pkg/crypto"
	"github.com/abbasnazari-0/xp-proto/pkg/obfs"
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
		fmt.Println(config.GenerateExampleConfig("server"))
		return
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		fmt.Println("💡 Run with -genconfig to generate example config")
		os.Exit(1)
	}

	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║       XP Protocol Server v1.0             ║")
	fmt.Println("║   🛡️  Anti-DPI • Anti-Probe • Stealth     ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	server := NewXPServer(cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n👋 Shutting down...")
		server.Stop()
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		fmt.Printf("❌ Server error: %v\n", err)
		os.Exit(1)
	}
}

type XPServer struct {
	config     *config.Config
	listener   net.Listener
	key        []byte
	fragmenter *obfs.Fragmenter
	padder     *obfs.Padder
}

func NewXPServer(cfg *config.Config) *XPServer {
	key, err := cfg.Server.GetKey()
	if err != nil {
		fmt.Printf("⚠️  Invalid key, generating new one\n")
		key, _ = crypto.GenerateKey()
	}

	fragConfig := obfs.DefaultFragmentConfig()
	fragConfig.Enabled = cfg.Server.Fragment

	padConfig := obfs.DefaultPaddingConfig()
	padConfig.Enabled = cfg.Server.Padding

	return &XPServer{
		config:     cfg,
		key:        key,
		fragmenter: obfs.NewFragmenter(fragConfig),
		padder:     obfs.NewPadder(padConfig),
	}
}

func (s *XPServer) Start() error {
	tlsConfig, err := s.createTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	listener, err := tls.Listen("tcp", s.config.Server.Listen, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	s.listener = listener

	fmt.Printf("🚀 Server listening on %s\n", s.config.Server.Listen)
	fmt.Printf("🎭 Fake site: %s\n", s.config.Server.FakeSite)
	fmt.Printf("🔧 Fragmentation: %v | Padding: %v | Timing: %v\n",
		s.config.Server.Fragment, s.config.Server.Padding, s.config.Server.TimingJitter)
	fmt.Println()
	fmt.Println("📡 Waiting for connections...")
	fmt.Println()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *XPServer) createTLSConfig() (*tls.Config, error) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

func (s *XPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("📥 New connection from %s\n", remoteAddr)

	tun, err := tunnel.NewTunnel(conn, s.key)
	if err != nil {
		fmt.Printf("❌ [%s] Failed to create tunnel: %v\n", remoteAddr, err)
		if s.config.Server.ProbeResist {
			s.proxyToFakeSite(conn)
		}
		return
	}

	for {
		buf := make([]byte, 65536)
		n, err := tun.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("⚠️  [%s] Read error: %v\n", remoteAddr, err)
			}
			return
		}

		if n < 1 {
			continue
		}

		cmd := buf[0]
		switch cmd {
		case 0x01:
			target := string(buf[1:n])
			fmt.Printf("🔗 [%s] Connecting to %s\n", remoteAddr, target)
			s.handleConnect(tun, target, remoteAddr)
			return
		default:
			fmt.Printf("⚠️  [%s] Unknown command: %d\n", remoteAddr, cmd)
		}
	}
}

func (s *XPServer) handleConnect(tun *tunnel.Tunnel, target string, clientAddr string) {
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		fmt.Printf("❌ [%s] Failed to connect to %s: %v\n", clientAddr, target, err)
		tun.Write([]byte{0x01})
		return
	}
	defer targetConn.Close()

	tun.Write([]byte{0x00})
	fmt.Printf("✅ [%s] Connected to %s\n", clientAddr, target)

	done := make(chan bool, 2)

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := targetConn.Read(buf)
			if err != nil {
				done <- true
				return
			}
			if _, err := tun.Write(buf[:n]); err != nil {
				done <- true
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := tun.Read(buf)
			if err != nil {
				done <- true
				return
			}
			if _, err := targetConn.Write(buf[:n]); err != nil {
				done <- true
				return
			}
		}
	}()

	<-done
	fmt.Printf("🔌 [%s] Disconnected from %s\n", clientAddr, target)
}

func (s *XPServer) proxyToFakeSite(clientConn net.Conn) {
	fakeSite := s.config.Server.FallbackSite
	if fakeSite == "" {
		fakeSite = s.config.Server.FakeSite
	}
	fmt.Printf("🎭 Proxying probe to fake site: %s\n", fakeSite)

	targetConn, err := tls.Dial("tcp", fakeSite+":443", &tls.Config{ServerName: fakeSite})
	if err != nil {
		return
	}
	defer targetConn.Close()

	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

func (s *XPServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

func generateSelfSignedCert() (tls.Certificate, error) {
	certPEM := []byte(`-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpeg/GTVMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnhw
LXZwbjAeFw0yNDAxMDEwMDAwMDBaFw0zNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnhwLXZwbjBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96HtiNYpMPHvrG7NXSE
knYJWRJvLk7sN/XhLmSJLMzPQvE7kNRrLwVvB3QbQCddVLyiB5aXHFLiFpVOYvbD
AgMBAAGjUzBRMB0GA1UdDgQWBBQBT9MfGpRtVc3JD1V/qVVFuYrXLTAfBgNVHSME
GDAWgBQBT9MfGpRtVc3JD1V/qVVFuYrXLTAPBgNVHRMBAf8EBTADAQH/MA0GCSqG
SIb3DQEBCwUAA0EAH6LmB8GWjPFsrKhGPy/vs0kNLPwLG4vqXqCmLX2rnRfVMqc3
k1XkEJZJpFEWQMwTVFQOsXiR6K0vN0FHwJOIJQ==
-----END CERTIFICATE-----`)

	keyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MIIBVQIBADANBgkqhkiG9w0BAQEFAASCAT8wggE7AgEAAkEAu6Peh7YjWKTDx76x
uzV0hJJ2CVkSby5O7Df14S5kiSzMz0LxO5DUay8Fbwd0G0AnXVS8ogeWlxxS4haV
TmL2wwIDAQABAkEAhB4xS9amSNUz0rigmT7TjVkP1vdF5S2B0caCi+PJXPA6A1xP
I+WCBPjVuO9ZRbFh5B6hkLvRvkH1bVQnUv9PIQIhAOfTNNj7lXz+wlp9eNmn2EJJ
JqvUjCNfDaQlJBdQ1f6TAiEA0A0FTXL1qBjb9qZKbJENFkQFLhXzlGzlccBR5yR5
aRECIDVe1S/fbNBNLMcN7X/gGkvhvtB0r8QROPB2WwwpJT/TAiEAy90BWgpATb2N
Lmn8GKLACvqmdCxbNeOQEOxpkNYRSzECIDhfKpaJFFKU/5YUGv3Ax3H+PK8XoRnf
MdGsqJ0YIAFF
-----END PRIVATE KEY-----`)

	return tls.X509KeyPair(certPEM, keyPEM)
}
