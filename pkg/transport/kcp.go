package transport

import (
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
)

// KCPTransport implements KCP-based transport with smux multiplexing
type KCPTransport struct {
	key          []byte
	mode         string
	dataShards   int
	parityShards int
}

// KCPConnection wraps a smux stream
type KCPConnection struct {
	stream  *smux.Stream
	session *smux.Session
}

// KCPListener wraps KCP listener with smux
type KCPListener struct {
	listener *kcp.Listener
	sessions map[string]*smux.Session
}

// NewKCPTransport creates a new KCP transport
func NewKCPTransport(key string, mode string, dataShards, parityShards int) (*KCPTransport, error) {
	// Generate encryption key from passphrase
	salt := []byte("xp-protocol-kcp-salt")
	derivedKey := pbkdf2.Key([]byte(key), salt, 4096, 32, sha256.New)

	if mode == "" {
		mode = "fast2"
	}
	if dataShards == 0 {
		dataShards = 10
	}
	if parityShards == 0 {
		parityShards = 3
	}

	return &KCPTransport{
		key:          derivedKey,
		mode:         mode,
		dataShards:   dataShards,
		parityShards: parityShards,
	}, nil
}

// createBlockCrypt creates AES encryption for KCP
func (t *KCPTransport) createBlockCrypt() (kcp.BlockCrypt, error) {
	return kcp.NewAESBlockCrypt(t.key)
}

// Dial connects to a KCP server
func (t *KCPTransport) Dial(address string) (Connection, error) {
	block, err := t.createBlockCrypt()
	if err != nil {
		return nil, fmt.Errorf("failed to create block crypt: %w", err)
	}

	// Connect with FEC
	conn, err := kcp.DialWithOptions(address, block, t.dataShards, t.parityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to dial KCP: %w", err)
	}

	// Apply KCP tuning
	t.tuneKCP(conn)

	// Create smux session
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = 10 * time.Second
	smuxConfig.KeepAliveTimeout = 30 * time.Second

	session, err := smux.Client(conn, smuxConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create smux session: %w", err)
	}

	// Open stream
	stream, err := session.OpenStream()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	return &KCPConnection{
		stream:  stream,
		session: session,
	}, nil
}

// Listen creates a KCP listener
func (t *KCPTransport) Listen(address string) (Listener, error) {
	block, err := t.createBlockCrypt()
	if err != nil {
		return nil, fmt.Errorf("failed to create block crypt: %w", err)
	}

	listener, err := kcp.ListenWithOptions(address, block, t.dataShards, t.parityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	return &KCPListener{
		listener: listener,
		sessions: make(map[string]*smux.Session),
	}, nil
}

// Close closes the transport
func (t *KCPTransport) Close() error {
	return nil
}

// tuneKCP applies performance tuning based on mode
func (t *KCPTransport) tuneKCP(conn *kcp.UDPSession) {
	// Set buffer sizes
	conn.SetReadBuffer(4 * 1024 * 1024)
	conn.SetWriteBuffer(4 * 1024 * 1024)

	// Configure based on mode
	switch t.mode {
	case "normal":
		conn.SetNoDelay(0, 40, 0, 0)
	case "fast":
		conn.SetNoDelay(0, 30, 2, 1)
	case "fast2":
		conn.SetNoDelay(1, 20, 2, 1)
	case "fast3":
		conn.SetNoDelay(1, 10, 2, 1)
	default:
		conn.SetNoDelay(1, 20, 2, 1)
	}

	conn.SetWindowSize(1024, 1024)
	conn.SetMtu(1350)
	conn.SetACKNoDelay(true)
	conn.SetStreamMode(true)
}

// Accept accepts a connection
func (l *KCPListener) Accept() (Connection, error) {
	conn, err := l.listener.AcceptKCP()
	if err != nil {
		return nil, err
	}

	// Tune connection
	conn.SetReadBuffer(4 * 1024 * 1024)
	conn.SetWriteBuffer(4 * 1024 * 1024)
	conn.SetNoDelay(1, 20, 2, 1)
	conn.SetWindowSize(1024, 1024)
	conn.SetMtu(1350)
	conn.SetACKNoDelay(true)
	conn.SetStreamMode(true)

	// Create smux session
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = 10 * time.Second
	smuxConfig.KeepAliveTimeout = 30 * time.Second

	session, err := smux.Server(conn, smuxConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create smux session: %w", err)
	}

	// Accept stream
	stream, err := session.AcceptStream()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	return &KCPConnection{
		stream:  stream,
		session: session,
	}, nil
}

// Close closes the listener
func (l *KCPListener) Close() error {
	return l.listener.Close()
}

// Addr returns listener address
func (l *KCPListener) Addr() string {
	return l.listener.Addr().String()
}

// Read reads from stream
func (c *KCPConnection) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

// Write writes to stream
func (c *KCPConnection) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

// Close closes connection
func (c *KCPConnection) Close() error {
	c.stream.Close()
	return c.session.Close()
}

// LocalAddr returns local address
func (c *KCPConnection) LocalAddr() string {
	return c.stream.LocalAddr().String()
}

// RemoteAddr returns remote address
func (c *KCPConnection) RemoteAddr() string {
	return c.stream.RemoteAddr().String()
}

// UDPObfuscator adds obfuscation to UDP packets
type UDPObfuscator struct {
	conn net.PacketConn
}

// NewUDPObfuscator creates an obfuscating UDP wrapper
func NewUDPObfuscator(conn net.PacketConn) *UDPObfuscator {
	return &UDPObfuscator{conn: conn}
}

// ReadFrom reads and deobfuscates
func (o *UDPObfuscator) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = o.conn.ReadFrom(p)
	if err == nil && n > 0 {
		// XOR with simple key
		for i := 0; i < n; i++ {
			p[i] ^= byte(0x42 + i%256)
		}
	}
	return
}

// WriteTo obfuscates and writes
func (o *UDPObfuscator) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	obfuscated := make([]byte, len(p))
	for i := 0; i < len(p); i++ {
		obfuscated[i] = p[i] ^ byte(0x42+i%256)
	}
	return o.conn.WriteTo(obfuscated, addr)
}

// Close closes the connection
func (o *UDPObfuscator) Close() error {
	return o.conn.Close()
}

// LocalAddr returns local address
func (o *UDPObfuscator) LocalAddr() net.Addr {
	return o.conn.LocalAddr()
}

// SetDeadline sets deadline
func (o *UDPObfuscator) SetDeadline(t time.Time) error {
	return o.conn.SetDeadline(t)
}

// SetReadDeadline sets read deadline
func (o *UDPObfuscator) SetReadDeadline(t time.Time) error {
	return o.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets write deadline
func (o *UDPObfuscator) SetWriteDeadline(t time.Time) error {
	return o.conn.SetWriteDeadline(t)
}
