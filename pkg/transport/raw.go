package transport

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// RawTransport implements raw TCP packet transport
// This bypasses the OS TCP stack for maximum stealth
type RawTransport struct {
	handle    *pcap.Handle
	iface     string
	localIP   net.IP
	localMAC  net.HardwareAddr
	routerMAC net.HardwareAddr
	mu        sync.Mutex
	conns     map[string]*RawConnection
	listener  *RawListener
}

// RawConnection represents a raw TCP connection
type RawConnection struct {
	transport  *RawTransport
	localPort  uint16
	remoteIP   net.IP
	remotePort uint16
	seqNum     uint32
	ackNum     uint32
	recvChan   chan []byte
	closed     bool
	mu         sync.Mutex
}

// RawListener listens for raw TCP connections
type RawListener struct {
	transport *RawTransport
	localPort uint16
	acceptCh  chan *RawConnection
	closed    bool
}

// NewRawTransport creates a new raw packet transport
func NewRawTransport(iface, localIP, routerMAC string) (*RawTransport, error) {
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

	t := &RawTransport{
		handle:    handle,
		iface:     iface,
		localIP:   lip.To4(),
		localMAC:  ifi.HardwareAddr,
		routerMAC: rmac,
		conns:     make(map[string]*RawConnection),
	}

	// Start packet receiver
	go t.receivePackets()

	return t, nil
}

// Dial connects to a remote address using raw packets
func (t *RawTransport) Dial(address string) (Connection, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// Resolve remote IP
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs found for %s", host)
	}
	remoteIP := ips[0].To4()

	// Parse port
	var port uint16
	fmt.Sscanf(portStr, "%d", &port)

	// Create connection
	conn := &RawConnection{
		transport:  t,
		localPort:  uint16(rand.Intn(16383) + 49152), // Random ephemeral port
		remoteIP:   remoteIP,
		remotePort: port,
		seqNum:     rand.Uint32(),
		ackNum:     0,
		recvChan:   make(chan []byte, 256),
	}

	// Register connection
	key := fmt.Sprintf("%s:%d", remoteIP.String(), port)
	t.mu.Lock()
	t.conns[key] = conn
	t.mu.Unlock()

	// Send SYN
	if err := conn.sendTCP(nil, true, false, false); err != nil {
		return nil, fmt.Errorf("failed to send SYN: %w", err)
	}

	// Wait for SYN-ACK (with timeout)
	select {
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("connection timeout")
	case <-conn.recvChan:
		// Got SYN-ACK, send ACK
		conn.sendTCP(nil, false, true, false)
	}

	return conn, nil
}

// Listen creates a raw TCP listener
func (t *RawTransport) Listen(address string) (Listener, error) {
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	var port uint16
	fmt.Sscanf(portStr, "%d", &port)

	listener := &RawListener{
		transport: t,
		localPort: port,
		acceptCh:  make(chan *RawConnection, 16),
	}

	t.mu.Lock()
	t.listener = listener
	t.mu.Unlock()

	return listener, nil
}

// Close closes the transport
func (t *RawTransport) Close() error {
	if t.handle != nil {
		t.handle.Close()
	}
	return nil
}

// receivePackets processes incoming packets
func (t *RawTransport) receivePackets() {
	packetSource := gopacket.NewPacketSource(t.handle, t.handle.LinkType())

	for packet := range packetSource.Packets() {
		t.processPacket(packet)
	}
}

func (t *RawTransport) processPacket(packet gopacket.Packet) {
	// Get IP layer
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	ip := ipLayer.(*layers.IPv4)

	// Get TCP layer
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp := tcpLayer.(*layers.TCP)

	// Check if this is for us
	if !ip.DstIP.Equal(t.localIP) {
		return
	}

	// Handle listener
	if t.listener != nil && tcp.DstPort == layers.TCPPort(t.listener.localPort) {
		if tcp.SYN && !tcp.ACK {
			// New connection request
			conn := &RawConnection{
				transport:  t,
				localPort:  t.listener.localPort,
				remoteIP:   ip.SrcIP,
				remotePort: uint16(tcp.SrcPort),
				seqNum:     rand.Uint32(),
				ackNum:     tcp.Seq + 1,
				recvChan:   make(chan []byte, 256),
			}

			key := fmt.Sprintf("%s:%d", ip.SrcIP.String(), tcp.SrcPort)
			t.mu.Lock()
			t.conns[key] = conn
			t.mu.Unlock()

			// Send SYN-ACK
			conn.sendTCP(nil, true, true, false)

			// Accept connection
			select {
			case t.listener.acceptCh <- conn:
			default:
			}
		}
	}

	// Handle existing connections
	key := fmt.Sprintf("%s:%d", ip.SrcIP.String(), tcp.SrcPort)
	t.mu.Lock()
	conn := t.conns[key]
	t.mu.Unlock()

	if conn != nil {
		if tcp.FIN {
			conn.closed = true
			close(conn.recvChan)
			return
		}

		conn.mu.Lock()
		conn.ackNum = tcp.Seq + uint32(len(tcp.Payload))
		if len(tcp.Payload) == 0 && tcp.SYN {
			conn.ackNum = tcp.Seq + 1
		}
		conn.mu.Unlock()

		if len(tcp.Payload) > 0 {
			select {
			case conn.recvChan <- tcp.Payload:
			default:
			}
		}
	}
}

// sendTCP sends a raw TCP packet
func (c *RawConnection) sendTCP(payload []byte, syn, ack, fin bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build Ethernet layer
	eth := &layers.Ethernet{
		SrcMAC:       c.transport.localMAC,
		DstMAC:       c.transport.routerMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	// Build IP layer
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    c.transport.localIP,
		DstIP:    c.remoteIP,
		Id:       uint16(rand.Intn(65535)),
	}

	// Build TCP layer
	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(c.localPort),
		DstPort: layers.TCPPort(c.remotePort),
		Seq:     c.seqNum,
		Ack:     c.ackNum,
		SYN:     syn,
		ACK:     ack,
		FIN:     fin,
		PSH:     len(payload) > 0,
		Window:  65535,
	}
	tcp.SetNetworkLayerForChecksum(ip)

	// Serialize packet
	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	if err := gopacket.SerializeLayers(buffer, opts, eth, ip, tcp, gopacket.Payload(payload)); err != nil {
		return fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Send packet
	if err := c.transport.handle.WritePacketData(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	// Update sequence number
	if syn {
		c.seqNum++
	}
	c.seqNum += uint32(len(payload))

	return nil
}

// Read reads data from the connection
func (c *RawConnection) Read(b []byte) (int, error) {
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}

	data, ok := <-c.recvChan
	if !ok {
		return 0, fmt.Errorf("connection closed")
	}

	n := copy(b, data)
	return n, nil
}

// Write writes data to the connection
func (c *RawConnection) Write(b []byte) (int, error) {
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}

	// Split into chunks
	const maxPayload = 1400
	total := 0

	for len(b) > 0 {
		chunk := b
		if len(chunk) > maxPayload {
			chunk = b[:maxPayload]
		}
		b = b[len(chunk):]

		if err := c.sendTCP(chunk, false, true, false); err != nil {
			return total, err
		}
		total += len(chunk)

		// Small delay between chunks
		time.Sleep(time.Millisecond)
	}

	return total, nil
}

// Close closes the connection
func (c *RawConnection) Close() error {
	if c.closed {
		return nil
	}

	c.sendTCP(nil, false, true, true) // FIN-ACK
	c.closed = true

	key := fmt.Sprintf("%s:%d", c.remoteIP.String(), c.remotePort)
	c.transport.mu.Lock()
	delete(c.transport.conns, key)
	c.transport.mu.Unlock()

	return nil
}

// LocalAddr returns local address
func (c *RawConnection) LocalAddr() string {
	return fmt.Sprintf("%s:%d", c.transport.localIP.String(), c.localPort)
}

// RemoteAddr returns remote address
func (c *RawConnection) RemoteAddr() string {
	return fmt.Sprintf("%s:%d", c.remoteIP.String(), c.remotePort)
}

// Accept accepts a connection
func (l *RawListener) Accept() (Connection, error) {
	if l.closed {
		return nil, fmt.Errorf("listener closed")
	}

	conn, ok := <-l.acceptCh
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}

	return conn, nil
}

// Close closes the listener
func (l *RawListener) Close() error {
	l.closed = true
	close(l.acceptCh)
	return nil
}

// Addr returns listener address
func (l *RawListener) Addr() string {
	return fmt.Sprintf(":%d", l.localPort)
}

// RawPacketBuilder helps build various packet types for obfuscation
type RawPacketBuilder struct {
	localMAC  net.HardwareAddr
	routerMAC net.HardwareAddr
	localIP   net.IP
}

// NewRawPacketBuilder creates a packet builder
func NewRawPacketBuilder(localMAC, routerMAC net.HardwareAddr, localIP net.IP) *RawPacketBuilder {
	return &RawPacketBuilder{
		localMAC:  localMAC,
		routerMAC: routerMAC,
		localIP:   localIP,
	}
}

// BuildHTTPLikePacket creates a packet that looks like HTTP traffic
func (b *RawPacketBuilder) BuildHTTPLikePacket(dstIP net.IP, dstPort uint16, payload []byte) []byte {
	// Prepend HTTP-like headers to payload
	httpPayload := append([]byte("GET / HTTP/1.1\r\nHost: www.google.com\r\n\r\n"), payload...)

	eth := &layers.Ethernet{
		SrcMAC:       b.localMAC,
		DstMAC:       b.routerMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    b.localIP,
		DstIP:    dstIP,
		Id:       uint16(rand.Intn(65535)),
	}

	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(rand.Intn(16383) + 49152),
		DstPort: layers.TCPPort(dstPort),
		Seq:     rand.Uint32(),
		PSH:     true,
		ACK:     true,
		Window:  65535,
	}
	tcp.SetNetworkLayerForChecksum(ip)

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	gopacket.SerializeLayers(buffer, opts, eth, ip, tcp, gopacket.Payload(httpPayload))
	return buffer.Bytes()
}

// BuildDNSLikePacket creates a packet that looks like DNS traffic
func (b *RawPacketBuilder) BuildDNSLikePacket(dstIP net.IP, payload []byte) []byte {
	// Encode payload as DNS query
	dnsPayload := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint16(dnsPayload[0:2], uint16(rand.Intn(65535))) // Transaction ID
	binary.BigEndian.PutUint16(dnsPayload[2:4], 0x0100)                   // Standard query
	binary.BigEndian.PutUint16(dnsPayload[4:6], 1)                        // Questions
	copy(dnsPayload[12:], payload)

	eth := &layers.Ethernet{
		SrcMAC:       b.localMAC,
		DstMAC:       b.routerMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    b.localIP,
		DstIP:    dstIP,
		Id:       uint16(rand.Intn(65535)),
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(rand.Intn(16383) + 49152),
		DstPort: 53,
	}
	udp.SetNetworkLayerForChecksum(ip)

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	gopacket.SerializeLayers(buffer, opts, eth, ip, udp, gopacket.Payload(dnsPayload))
	return buffer.Bytes()
}
