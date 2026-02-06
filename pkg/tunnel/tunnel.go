package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xp-proto/xp/pkg/crypto"
	"github.com/xp-proto/xp/pkg/obfs"
)

type Tunnel struct {
	conn       net.Conn
	crypto     *crypto.XPCrypto
	fragmenter *obfs.Fragmenter
	padder     *obfs.Padder
	timing     *obfs.TimingObfuscator
	readMu     sync.Mutex
	writeMu    sync.Mutex
	closed     bool
}

func NewTunnel(conn net.Conn, key []byte) (*Tunnel, error) {
	xpCrypto, err := crypto.NewXPCrypto(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create crypto: %w", err)
	}
	return &Tunnel{
		conn:       conn,
		crypto:     xpCrypto,
		fragmenter: obfs.NewFragmenter(obfs.DefaultFragmentConfig()),
		padder:     obfs.NewPadder(obfs.DefaultPaddingConfig()),
		timing:     obfs.NewTimingObfuscator(obfs.DefaultTimingConfig()),
	}, nil
}

func (t *Tunnel) Write(data []byte) (int, error) {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if t.closed {
		return 0, io.ErrClosedPipe
	}
	padded := t.padder.Pad(data)
	encrypted, err := t.crypto.Encrypt(padded)
	if err != nil {
		return 0, fmt.Errorf("encryption failed: %w", err)
	}
	packet := make([]byte, 4+len(encrypted))
	binary.BigEndian.PutUint32(packet[:4], uint32(len(encrypted)))
	copy(packet[4:], encrypted)
	t.timing.SimulateHTTPTiming()
	_, err = t.conn.Write(packet)
	if err != nil {
		return 0, fmt.Errorf("write failed: %w", err)
	}
	return len(data), nil
}

func (t *Tunnel) Read(buf []byte) (int, error) {
	t.readMu.Lock()
	defer t.readMu.Unlock()
	if t.closed {
		return 0, io.ErrClosedPipe
	}
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(t.conn, lenBuf); err != nil {
		return 0, err
	}
	packetLen := binary.BigEndian.Uint32(lenBuf)
	if packetLen > 1024*1024 {
		return 0, fmt.Errorf("packet too large: %d", packetLen)
	}
	encrypted := make([]byte, packetLen)
	if _, err := io.ReadFull(t.conn, encrypted); err != nil {
		return 0, err
	}
	padded, err := t.crypto.Decrypt(encrypted)
	if err != nil {
		return 0, fmt.Errorf("decryption failed: %w", err)
	}
	data, err := t.padder.Unpad(padded)
	if err != nil {
		return 0, fmt.Errorf("unpad failed: %w", err)
	}
	n := copy(buf, data)
	return n, nil
}

func (t *Tunnel) Close() error {
	t.closed = true
	return t.conn.Close()
}

func (t *Tunnel) LocalAddr() net.Addr  { return t.conn.LocalAddr() }
func (t *Tunnel) RemoteAddr() net.Addr { return t.conn.RemoteAddr() }

func (t *Tunnel) SetDeadline(d time.Time) error      { return t.conn.SetDeadline(d) }
func (t *Tunnel) SetReadDeadline(d time.Time) error  { return t.conn.SetReadDeadline(d) }
func (t *Tunnel) SetWriteDeadline(d time.Time) error { return t.conn.SetWriteDeadline(d) }
