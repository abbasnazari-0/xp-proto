package transport

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
)

// RawKCPTransport combines raw packets with KCP for ultimate stealth
// This bypasses the OS TCP/UDP stack entirely while providing reliable transport
type RawKCPTransport struct {
	handle       *pcap.Handle
	iface        string
	localIP      net.IP
	localMAC     net.HardwareAddr
	routerMAC    net.HardwareAddr
	key          []byte
	dataShards   int
	parityShards int
	mu           sync.Mutex
	fakeConn     *FakeUDPConn
}

// FakeUDPConn implements net.PacketConn over raw packets
type FakeUDPConn struct {
	transport  *RawKCPTransport
	localPort  uint16
	remoteAddr *net.UDPAddr
	recvChan   chan *UDPPacket
	closed     bool
	mu         sync.Mutex
}

// UDPPacket represents a received UDP packet
type UDPPacket struct {
	data []byte
	addr *net.UDPAddr
}

// RawKCPConnection wraps smux stream over raw KCP
type RawKCPConnection struct {
	stream    *smux.Stream
	session   *smux.Session
	transport *RawKCPTransport
}

// RawKCPListener listens for raw KCP connections
type RawKCPListener struct {
	transport *RawKCPTransport
	listener  *kcp.Listener
	sessions  map[string]*smux.Session
	mu        sync.Mutex
}

// NewRawKCPTransport creates a raw packet transport with KCP
func NewRawKCPTransport(iface, localIP, routerMAC string) (*RawKCPTransport, error) {
	// Open pcap handle
	handle, err := pcap.OpenLive(iface, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open interface %s: %w", iface, err)
	}

	// Parse local IP
	lip := net.ParseIP(localIP)
	if lip == nil {
		handle.Close()
		return nil, fmt.Errorf("invalid local IP: %s", localIP)
	}

	// Parse router MAC
	rmac, err := net.ParseMAC(routerMAC)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("invalid router MAC: %s", routerMAC)
	}

	// Get local MAC
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to get interface: %w", err)
	}

	// Generate encryption key
	salt := []byte("xp-protocol-raw-kcp")
	key := pbkdf2.Key([]byte("xp-proto"), salt, 4096, 32, sha256.New)

	t := &RawKCPTransport{
		handle:       handle,
		iface:        iface,
		localIP:      lip.To4(),
		localMAC:     ifi.HardwareAddr,
		routerMAC:    rmac,
		key:          key,
		dataShards:   10,
		parityShards: 3,
	}

	return t, nil
}

// Dial connects using raw packets + KCP
func (t *RawKCPTransport) Dial(address string) (Connection, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// Resolve remote IP
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
	}
	remoteIP := ips[0].To4()

	var port int
	fmt.Sscanf(portStr, "%d", &port)

	// Create fake UDP connection over raw packets
	fakeConn := &FakeUDPConn{
		transport: t,
		localPort: uint16(rand.Intn(16383) + 49152),
		remoteAddr: &net.UDPAddr{
			IP:   remoteIP,
			Port: port,
		},
		recvChan: make(chan *UDPPacket, 256),
	}

	t.mu.Lock()
	t.fakeConn = fakeConn
	t.mu.Unlock()

	// Start packet receiver
	go t.receivePackets()

	// Create KCP connection over fake UDP
	block, err := kcp.NewAESBlockCrypt(t.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create block crypt: %w", err)
	}

	kcpConn, err := kcp.NewConn2(fakeConn.remoteAddr, block, t.dataShards, t.parityShards, fakeConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP connection: %w", err)
	}

	// Tune KCP
	kcpConn.SetReadBuffer(4 * 1024 * 1024)
	kcpConn.SetWriteBuffer(4 * 1024 * 1024)
	kcpConn.SetNoDelay(1, 20, 2, 1)
	kcpConn.SetWindowSize(1024, 1024)
	kcpConn.SetMtu(1350)
	kcpConn.SetACKNoDelay(true)
	kcpConn.SetStreamMode(true)

	// Create smux session
	smuxConfig := smux.DefaultConfig()
	smuxConfig.Version = 2
	smuxConfig.KeepAliveInterval = 10 * time.Second

	session, err := smux.Client(kcpConn, smuxConfig)
	if err != nil {
		kcpConn.Close()
		return nil, fmt.Errorf("failed to create smux session: %w", err)
	}

	stream, err := session.OpenStream()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	return &RawKCPConnection{
		stream:    stream,
		session:   session,
		transport: t,
	}, nil
}

// Listen creates a raw KCP listener
func (t *RawKCPTransport) Listen(address string) (Listener, error) {
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)

	// Create fake UDP connection for listening
	fakeConn := &FakeUDPConn{
		transport: t,
		localPort: uint16(port),
		recvChan:  make(chan *UDPPacket, 256),
	}

	t.mu.Lock()
	t.fakeConn = fakeConn
	t.mu.Unlock()

	// Start packet receiver
	go t.receivePackets()

	// Create KCP listener over fake UDP
	block, err := kcp.NewAESBlockCrypt(t.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create block crypt: %w", err)
	}

	listener, err := kcp.ServeConn(block, t.dataShards, t.parityShards, fakeConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP listener: %w", err)
	}

	return &RawKCPListener{
		transport: t,
		listener:  listener,
		sessions:  make(map[string]*smux.Session),
	}, nil
}

// Close closes the transport
func (t *RawKCPTransport) Close() error {
	if t.handle != nil {
		t.handle.Close()
	}
	return nil
}

// receivePackets processes incoming raw packets
func (t *RawKCPTransport) receivePackets() {
	packetSource := gopacket.NewPacketSource(t.handle, t.handle.LinkType())

	for packet := range packetSource.Packets() {
		t.processPacket(packet)
	}
}

func (t *RawKCPTransport) processPacket(packet gopacket.Packet) {
	// Get IP layer
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	ip := ipLayer.(*layers.IPv4)

	// Check if this is for us
	if !ip.DstIP.Equal(t.localIP) {
		return
	}

	// Get UDP layer
	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return
	}
	udp := udpLayer.(*layers.UDP)

	t.mu.Lock()
	fakeConn := t.fakeConn
	t.mu.Unlock()

	if fakeConn == nil {
		return
	}

	// Check port
	if uint16(udp.DstPort) != fakeConn.localPort {
		return
	}

	// Forward to fake connection
	pkt := &UDPPacket{
		data: udp.Payload,
		addr: &net.UDPAddr{
			IP:   ip.SrcIP,
			Port: int(udp.SrcPort),
		},
	}

	select {
	case fakeConn.recvChan <- pkt:
	default:
	}
}

// FakeUDPConn implements net.PacketConn

func (c *FakeUDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	pkt, ok := <-c.recvChan
	if !ok {
		return 0, nil, fmt.Errorf("connection closed")
	}
	n = copy(p, pkt.data)
	return n, pkt.addr, nil
}

func (c *FakeUDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("invalid address type")
	}

	// Build and send raw UDP packet
	eth := &layers.Ethernet{
		SrcMAC:       c.transport.localMAC,
		DstMAC:       c.transport.routerMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    c.transport.localIP,
		DstIP:    udpAddr.IP.To4(),
		Id:       uint16(rand.Intn(65535)),
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(c.localPort),
		DstPort: layers.UDPPort(udpAddr.Port),
	}
	udp.SetNetworkLayerForChecksum(ip)

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	if err := gopacket.SerializeLayers(buffer, opts, eth, ip, udp, gopacket.Payload(p)); err != nil {
		return 0, fmt.Errorf("failed to serialize packet: %w", err)
	}

	if err := c.transport.handle.WritePacketData(buffer.Bytes()); err != nil {
		return 0, fmt.Errorf("failed to send packet: %w", err)
	}

	return len(p), nil
}

func (c *FakeUDPConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.recvChan)
	}
	return nil
}

func (c *FakeUDPConn) LocalAddr() net.Addr {
	return &net.UDPAddr{
		IP:   c.transport.localIP,
		Port: int(c.localPort),
	}
}

func (c *FakeUDPConn) SetDeadline(t time.Time) error      { return nil }
func (c *FakeUDPConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *FakeUDPConn) SetWriteDeadline(t time.Time) error { return nil }

// RawKCPConnection methods

func (c *RawKCPConnection) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

func (c *RawKCPConnection) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

func (c *RawKCPConnection) Close() error {
	c.stream.Close()
	return c.session.Close()
}

func (c *RawKCPConnection) LocalAddr() string {
	return c.stream.LocalAddr().String()
}

func (c *RawKCPConnection) RemoteAddr() string {
	return c.stream.RemoteAddr().String()
}

// RawKCPListener methods

func (l *RawKCPListener) Accept() (Connection, error) {
	conn, err := l.listener.AcceptKCP()
	if err != nil {
		return nil, err
	}

	// Tune KCP
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

	session, err := smux.Server(conn, smuxConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create smux session: %w", err)
	}

	stream, err := session.AcceptStream()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	return &RawKCPConnection{
		stream:    stream,
		session:   session,
		transport: l.transport,
	}, nil
}

func (l *RawKCPListener) Close() error {
	return l.listener.Close()
}

func (l *RawKCPListener) Addr() string {
	return l.listener.Addr().String()
}
