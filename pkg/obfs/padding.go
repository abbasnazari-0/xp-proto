package obfs

import (
	"crypto/rand"
	"encoding/binary"
)

type PaddingConfig struct {
	Enabled bool
	MinPad  int
	MaxPad  int
}

func DefaultPaddingConfig() PaddingConfig {
	return PaddingConfig{Enabled: true, MinPad: 16, MaxPad: 256}
}

type Padder struct {
	config PaddingConfig
}

func NewPadder(config PaddingConfig) *Padder {
	return &Padder{config: config}
}

func (p *Padder) Pad(data []byte) []byte {
	if !p.config.Enabled {
		result := make([]byte, 2+len(data))
		binary.BigEndian.PutUint16(result[:2], uint16(len(data)))
		copy(result[2:], data)
		return result
	}
	padLen := randomInt(p.config.MinPad, p.config.MaxPad)
	result := make([]byte, 2+len(data)+padLen)
	binary.BigEndian.PutUint16(result[:2], uint16(len(data)))
	copy(result[2:], data)
	rand.Read(result[2+len(data):])
	return result
}

func (p *Padder) Unpad(data []byte) ([]byte, error) {
	if len(data) < 2 {
		return data, nil
	}
	originalLen := binary.BigEndian.Uint16(data[:2])
	if int(originalLen) > len(data)-2 {
		return data[2:], nil
	}
	return data[2 : 2+originalLen], nil
}

func (p *Padder) PadToSize(data []byte, targetSize int) []byte {
	if len(data)+2 >= targetSize {
		return p.Pad(data)
	}
	result := make([]byte, targetSize)
	binary.BigEndian.PutUint16(result[:2], uint16(len(data)))
	copy(result[2:], data)
	rand.Read(result[2+len(data):])
	return result
}

func RandomizeMTU() int {
	mtus := []int{1400, 1420, 1440, 1460, 1480, 1500}
	idx := randomInt(0, len(mtus))
	if idx >= len(mtus) {
		idx = len(mtus) - 1
	}
	return mtus[idx]
}
