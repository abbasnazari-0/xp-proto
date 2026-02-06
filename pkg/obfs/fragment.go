package obfs

import (
	"crypto/rand"
	"math/big"
	"net"
	"time"
)

type FragmentConfig struct {
	Enabled  bool
	MinSize  int
	MaxSize  int
	MinDelay time.Duration
	MaxDelay time.Duration
}

func DefaultFragmentConfig() FragmentConfig {
	return FragmentConfig{
		Enabled:  true,
		MinSize:  10,
		MaxSize:  50,
		MinDelay: 10 * time.Millisecond,
		MaxDelay: 50 * time.Millisecond,
	}
}

type Fragmenter struct {
	config FragmentConfig
}

func NewFragmenter(config FragmentConfig) *Fragmenter {
	return &Fragmenter{config: config}
}

func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	return int(n.Int64()) + min
}

func randomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	return time.Duration(n.Int64()) + min
}

func (f *Fragmenter) WriteFragmented(conn net.Conn, data []byte) error {
	if !f.config.Enabled || len(data) < f.config.MinSize*2 {
		_, err := conn.Write(data)
		return err
	}
	offset := 0
	for offset < len(data) {
		fragSize := randomInt(f.config.MinSize, f.config.MaxSize)
		if offset+fragSize > len(data) {
			fragSize = len(data) - offset
		}
		_, err := conn.Write(data[offset : offset+fragSize])
		if err != nil {
			return err
		}
		offset += fragSize
		if offset < len(data) {
			delay := randomDuration(f.config.MinDelay, f.config.MaxDelay)
			time.Sleep(delay)
		}
	}
	return nil
}

func (f *Fragmenter) FragmentTLSClientHello(conn net.Conn, clientHello []byte) error {
	if len(clientHello) < 100 {
		_, err := conn.Write(clientHello)
		return err
	}
	firstChunk := randomInt(15, 25)
	if firstChunk > len(clientHello) {
		firstChunk = len(clientHello)
	}
	_, err := conn.Write(clientHello[:firstChunk])
	if err != nil {
		return err
	}
	time.Sleep(randomDuration(20*time.Millisecond, 50*time.Millisecond))
	offset := firstChunk
	sniRegionEnd := min(len(clientHello), 200)
	for offset < sniRegionEnd {
		fragSize := randomInt(1, 5)
		if offset+fragSize > sniRegionEnd {
			fragSize = sniRegionEnd - offset
		}
		_, err := conn.Write(clientHello[offset : offset+fragSize])
		if err != nil {
			return err
		}
		offset += fragSize
		if offset < sniRegionEnd {
			time.Sleep(randomDuration(10*time.Millisecond, 30*time.Millisecond))
		}
	}
	if offset < len(clientHello) {
		time.Sleep(randomDuration(10*time.Millisecond, 20*time.Millisecond))
		_, err = conn.Write(clientHello[offset:])
		if err != nil {
			return err
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
