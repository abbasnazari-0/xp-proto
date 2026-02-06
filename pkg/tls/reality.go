package xtls

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"
)

type RealityClient struct {
	targetHost string
	realServer string
}

func NewRealityClient(targetHost, realServer string) *RealityClient {
	return &RealityClient{targetHost: targetHost, realServer: realServer}
}

func (r *RealityClient) Dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", r.realServer, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	tlsConfig := &tls.Config{
		ServerName:         r.targetHost,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
	}
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}
	return tlsConn, nil
}

type RealityServer struct {
	targetHost string
	listener   net.Listener
	authKey    []byte
}

func NewRealityServer(targetHost string, authKey []byte) *RealityServer {
	return &RealityServer{targetHost: targetHost, authKey: authKey}
}

func (r *RealityServer) Listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	r.listener = listener
	return nil
}

func (r *RealityServer) Accept() (net.Conn, bool, error) {
	conn, err := r.listener.Accept()
	if err != nil {
		return nil, false, err
	}
	return conn, true, nil
}

func (r *RealityServer) ProxyToRealSite(clientConn net.Conn) {
	defer clientConn.Close()
	targetConn, err := net.DialTimeout("tcp", r.targetHost+":443", 10*time.Second)
	if err != nil {
		return
	}
	defer targetConn.Close()
	go io.Copy(targetConn, clientConn)
	io.Copy(clientConn, targetConn)
}

type CertStealer struct{}

func (c *CertStealer) StealCert(host string) (*tls.Certificate, error) {
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", host, err)
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no certificates received from %s", host)
	}
	cert := state.PeerCertificates[0]
	fmt.Printf("Stole cert from %s: Subject=%s, Issuer=%s\n", host, cert.Subject.CommonName, cert.Issuer.CommonName)
	return nil, nil
}
