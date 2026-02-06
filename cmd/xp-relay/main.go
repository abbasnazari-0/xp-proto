package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
)

//═══════════════════════════════════════════════════════════════════════════════
//  XP Protocol - Relay/Bridge Server
//  برای تونل زدن از سرور ایران به سرور خارج
//
//  کاربرد:
//  Client (Iran) → Relay Server (Iran) → XP Server (Abroad)
//
//  مزیت: IP سرور خارج مخفی میمونه و فقط IP سرور ایران دیده میشه
//═══════════════════════════════════════════════════════════════════════════════

var (
	listenAddr = flag.String("l", "0.0.0.0:443", "Listen address")
	targetAddr = flag.String("t", "", "Target XP server address (required)")
	mode       = flag.String("m", "tcp", "Mode: tcp, ws, or sni")
)

func main() {
	flag.Parse()

	if *targetAddr == "" {
		fmt.Println("❌ Target address required!")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  xp-relay -l 0.0.0.0:443 -t YOUR_FOREIGN_SERVER:443")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  xp-relay -l 0.0.0.0:443 -t 1.2.3.4:443")
		fmt.Println("")
		os.Exit(1)
	}

	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║       XP Protocol Relay Server            ║")
	fmt.Println("║   🔀 Bridge • Tunnel • Stealth            ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("📡 Listen: %s\n", *listenAddr)
	fmt.Printf("🎯 Target: %s\n", *targetAddr)
	fmt.Printf("🔧 Mode: %s\n", *mode)
	fmt.Println()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n👋 Shutting down...")
		os.Exit(0)
	}()

	// Start relay
	switch *mode {
	case "tcp":
		startTCPRelay()
	case "sni":
		startSNIRelay()
	default:
		startTCPRelay()
	}
}

func startTCPRelay() {
	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Printf("❌ Failed to listen: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("✅ TCP Relay started")
	fmt.Println("📡 Waiting for connections...")
	fmt.Println()

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleTCPRelay(clientConn)
	}
}

func handleTCPRelay(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	fmt.Printf("📥 [%s] New connection\n", clientAddr)

	// Connect to target
	targetConn, err := net.Dial("tcp", *targetAddr)
	if err != nil {
		fmt.Printf("❌ [%s] Failed to connect to target: %v\n", clientAddr, err)
		return
	}
	defer targetConn.Close()

	fmt.Printf("🔗 [%s] Connected to target\n", clientAddr)

	// Bidirectional copy
	done := make(chan bool, 2)

	go func() {
		io.Copy(targetConn, clientConn)
		done <- true
	}()

	go func() {
		io.Copy(clientConn, targetConn)
		done <- true
	}()

	<-done
	fmt.Printf("🔌 [%s] Disconnected\n", clientAddr)
}

// SNI-based relay - forwards based on SNI in TLS ClientHello
func startSNIRelay() {
	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		fmt.Printf("❌ Failed to listen: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("✅ SNI Relay started")
	fmt.Println("📡 Waiting for connections...")
	fmt.Println()

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleSNIRelay(clientConn)
	}
}

func handleSNIRelay(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()

	// Read first bytes to peek at TLS ClientHello
	buf := make([]byte, 1024)
	n, err := clientConn.Read(buf)
	if err != nil {
		return
	}

	// Extract SNI (simplified - just forward everything)
	// In production, parse TLS ClientHello properly
	
	// Connect to target
	targetConn, err := net.Dial("tcp", *targetAddr)
	if err != nil {
		fmt.Printf("❌ [%s] Failed to connect to target: %v\n", clientAddr, err)
		return
	}
	defer targetConn.Close()

	// Send buffered data
	targetConn.Write(buf[:n])

	fmt.Printf("🔗 [%s] Relaying...\n", clientAddr)

	// Bidirectional copy
	done := make(chan bool, 2)

	go func() {
		io.Copy(targetConn, clientConn)
		done <- true
	}()

	go func() {
		io.Copy(clientConn, targetConn)
		done <- true
	}()

	<-done
}
