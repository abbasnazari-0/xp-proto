package tunnel

import (
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	SOCKS5Version  = 0x05
	AuthNone       = 0x00
	CmdConnect     = 0x01
	AddrIPv4       = 0x01
	AddrDomain     = 0x03
	AddrIPv6       = 0x04
	RepSuccess     = 0x00
	RepServerFail  = 0x01
	RepCmdNotSupp  = 0x07
	RepAddrNotSupp = 0x08
)

type SOCKS5Server struct {
	listenAddr string
	tunnel     *Tunnel
	listener   net.Listener
	mu         sync.Mutex
}

func NewSOCKS5Server(listenAddr string) *SOCKS5Server {
	return &SOCKS5Server{listenAddr: listenAddr}
}

func (s *SOCKS5Server) SetTunnel(tunnel *Tunnel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunnel = tunnel
}

func (s *SOCKS5Server) Start() error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start SOCKS5 server: %w", err)
	}
	s.listener = listener
	fmt.Printf("🧦 SOCKS5 server listening on %s\n", s.listenAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *SOCKS5Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	if err := s.handshake(conn); err != nil {
		return
	}
	target, err := s.readRequest(conn)
	if err != nil {
		return
	}
	s.mu.Lock()
	tun := s.tunnel
	s.mu.Unlock()
	if tun == nil {
		s.sendReply(conn, RepServerFail)
		return
	}
	connectReq := append([]byte{CmdConnect}, []byte(target)...)
	if _, err := tun.Write(connectReq); err != nil {
		s.sendReply(conn, RepServerFail)
		return
	}
	respBuf := make([]byte, 1024)
	n, err := tun.Read(respBuf)
	if err != nil || n < 1 {
		s.sendReply(conn, RepServerFail)
		return
	}
	if respBuf[0] != RepSuccess {
		s.sendReply(conn, respBuf[0])
		return
	}
	s.sendReply(conn, RepSuccess)
	s.proxy(conn, tun)
}

func (s *SOCKS5Server) handshake(conn net.Conn) error {
	buf := make([]byte, 258)
	n, err := conn.Read(buf)
	if err != nil || n < 2 {
		return fmt.Errorf("handshake read failed")
	}
	if buf[0] != SOCKS5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}
	_, err = conn.Write([]byte{SOCKS5Version, AuthNone})
	return err
}

func (s *SOCKS5Server) readRequest(conn net.Conn) (string, error) {
	buf := make([]byte, 262)
	n, err := conn.Read(buf)
	if err != nil || n < 4 {
		return "", fmt.Errorf("request read failed")
	}
	if buf[0] != SOCKS5Version {
		return "", fmt.Errorf("unsupported version")
	}
	if buf[1] != CmdConnect {
		s.sendReply(conn, RepCmdNotSupp)
		return "", fmt.Errorf("unsupported command: %d", buf[1])
	}
	var host string
	var port uint16
	switch buf[3] {
	case AddrIPv4:
		if n < 10 {
			return "", fmt.Errorf("invalid IPv4")
		}
		host = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		port = uint16(buf[8])<<8 | uint16(buf[9])
	case AddrDomain:
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return "", fmt.Errorf("invalid domain")
		}
		host = string(buf[5 : 5+domainLen])
		port = uint16(buf[5+domainLen])<<8 | uint16(buf[6+domainLen])
	case AddrIPv6:
		if n < 22 {
			return "", fmt.Errorf("invalid IPv6")
		}
		ip := net.IP(buf[4:20])
		host = ip.String()
		port = uint16(buf[20])<<8 | uint16(buf[21])
	default:
		s.sendReply(conn, RepAddrNotSupp)
		return "", fmt.Errorf("unsupported address type: %d", buf[3])
	}
	return fmt.Sprintf("%s:%d", host, port), nil
}

func (s *SOCKS5Server) sendReply(conn net.Conn, rep byte) {
	reply := []byte{SOCKS5Version, rep, 0x00, AddrIPv4, 0, 0, 0, 0, 0, 0}
	conn.Write(reply)
}

func (s *SOCKS5Server) proxy(client net.Conn, tun *Tunnel) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := client.Read(buf)
			if err != nil {
				return
			}
			if _, err := tun.Write(buf[:n]); err != nil {
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := tun.Read(buf)
			if err != nil {
				return
			}
			if _, err := client.Write(buf[:n]); err != nil {
				return
			}
		}
	}()
	wg.Wait()
}

func (s *SOCKS5Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func ProxyConn(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	copyFunc := func(dst, src net.Conn) {
		defer wg.Done()
		io.Copy(dst, src)
		dst.Close()
	}
	go copyFunc(c1, c2)
	go copyFunc(c2, c1)
	wg.Wait()
}
