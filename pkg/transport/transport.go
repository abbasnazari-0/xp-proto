package transport

import (
	"fmt"
	"io"
	"net"
)

// ResolveIPv4 resolves a hostname to IPv4 address only
// IPv6 doesn't work in Iran, so we force IPv4 everywhere
func ResolveIPv4(host string) (string, error) {
	// Check if already an IP
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
		return "", fmt.Errorf("IPv6 address not supported: %s", host)
	}

	// Resolve hostname to IPv4 only
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s: %w", host, err)
	}

	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found for %s", host)
}

// ResolveAddressIPv4 resolves host:port to IPv4 address
func ResolveAddressIPv4(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	ipv4, err := ResolveIPv4(host)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(ipv4, port), nil
}

// Transport defines the interface for all transport modes
type Transport interface {
	// Dial connects to the remote address
	Dial(address string) (Connection, error)
	// Listen starts listening on the address
	Listen(address string) (Listener, error)
	// Close shuts down the transport
	Close() error
}

// Connection represents a connection over the transport
type Connection interface {
	io.ReadWriteCloser
	LocalAddr() string
	RemoteAddr() string
}

// Listener listens for incoming connections
type Listener interface {
	Accept() (Connection, error)
	Close() error
	Addr() string
}

// Mode represents the transport mode
type Mode string

const (
	ModeTLS Mode = "tls"
	ModeKCP Mode = "kcp"
	ModeRaw Mode = "raw"
)

// Config holds transport configuration
type Config struct {
	Mode         Mode
	Interface    string   // For raw mode
	LocalIP      string   // For raw mode
	RouterMAC    string   // For raw mode
	TCPFlags     []string // For raw mode
	UseKCP       bool     // Use KCP over raw
	KCPMode      string   // KCP mode: normal, fast, fast2, fast3
	DataShards   int      // Reed-Solomon data shards
	ParityShards int      // Reed-Solomon parity shards
}

// NetConnWrapper wraps net.Conn to implement Connection interface
type NetConnWrapper struct {
	net.Conn
}

func (w *NetConnWrapper) LocalAddr() string {
	return w.Conn.LocalAddr().String()
}

func (w *NetConnWrapper) RemoteAddr() string {
	return w.Conn.RemoteAddr().String()
}

// NetListenerWrapper wraps net.Listener to implement Listener interface
type NetListenerWrapper struct {
	net.Listener
}

func (w *NetListenerWrapper) Accept() (Connection, error) {
	conn, err := w.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &NetConnWrapper{conn}, nil
}

func (w *NetListenerWrapper) Addr() string {
	return w.Listener.Addr().String()
}

// TCPTransport implements standard TCP transport (for testing/fallback)
type TCPTransport struct{}

func NewTCPTransport() *TCPTransport {
	return &TCPTransport{}
}

func (t *TCPTransport) Dial(address string) (Connection, error) {
	// Force IPv4 - IPv6 doesn't work in Iran
	conn, err := net.Dial("tcp4", address)
	if err != nil {
		return nil, err
	}
	return &NetConnWrapper{conn}, nil
}

func (t *TCPTransport) Listen(address string) (Listener, error) {
	// Force IPv4 - IPv6 doesn't work in Iran
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, err
	}
	return &NetListenerWrapper{listener}, nil
}

func (t *TCPTransport) Close() error {
	return nil
}

// NewTransport creates a transport based on mode
func NewTransport(cfg *Config) (Transport, error) {
	switch cfg.Mode {
	case ModeRaw:
		if cfg.UseKCP {
			return NewRawKCPTransport(cfg.Interface, cfg.LocalIP, cfg.RouterMAC)
		}
		return NewRawTransport(cfg.Interface, cfg.LocalIP, cfg.RouterMAC)
	case ModeKCP:
		return NewKCPTransport("", cfg.KCPMode, cfg.DataShards, cfg.ParityShards)
	default:
		return NewTCPTransport(), nil
	}
}
