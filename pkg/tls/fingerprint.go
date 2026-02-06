package xtls

import (
	"crypto/rand"
	"encoding/binary"
)

var (
	chromeCipherSuites = []uint16{
		0x1301, 0x1302, 0x1303, 0xc02c, 0xc02b, 0xc030, 0xc02f,
		0xcca9, 0xcca8, 0xc013, 0xc014, 0x009c, 0x009d,
	}
	supportedGroups = []uint16{0x001d, 0x0017, 0x0018, 0x0019}
	signatureAlgorithms = []uint16{
		0x0403, 0x0503, 0x0603, 0x0807, 0x0808, 0x0804,
		0x0805, 0x0806, 0x0401, 0x0501, 0x0601,
	}
)

type ClientHelloBuilder struct {
	serverName string
	sessionID  []byte
}

func NewClientHelloBuilder(serverName string) *ClientHelloBuilder {
	sessionID := make([]byte, 32)
	rand.Read(sessionID)
	return &ClientHelloBuilder{serverName: serverName, sessionID: sessionID}
}

func (b *ClientHelloBuilder) Build() []byte {
	extensions := b.buildExtensions()
	handshakeLen := 2 + 32 + 1 + len(b.sessionID) + 2 + len(chromeCipherSuites)*2 + 2 + 2 + len(extensions)
	record := make([]byte, 5+4+handshakeLen)
	record[0] = 0x16
	binary.BigEndian.PutUint16(record[1:3], 0x0301)
	binary.BigEndian.PutUint16(record[3:5], uint16(handshakeLen+4))
	record[5] = 0x01
	record[6] = 0x00
	binary.BigEndian.PutUint16(record[7:9], uint16(handshakeLen))
	offset := 9
	binary.BigEndian.PutUint16(record[offset:], 0x0303)
	offset += 2
	clientRandom := make([]byte, 32)
	rand.Read(clientRandom)
	copy(record[offset:], clientRandom)
	offset += 32
	record[offset] = byte(len(b.sessionID))
	offset++
	copy(record[offset:], b.sessionID)
	offset += len(b.sessionID)
	binary.BigEndian.PutUint16(record[offset:], uint16(len(chromeCipherSuites)*2))
	offset += 2
	for _, suite := range chromeCipherSuites {
		binary.BigEndian.PutUint16(record[offset:], suite)
		offset += 2
	}
	record[offset] = 1
	offset++
	record[offset] = 0
	offset++
	binary.BigEndian.PutUint16(record[offset:], uint16(len(extensions)))
	offset += 2
	copy(record[offset:], extensions)
	return record
}

func (b *ClientHelloBuilder) buildExtensions() []byte {
	var ext []byte
	ext = append(ext, b.buildSNI()...)
	ext = append(ext, 0x00, 0x17, 0x00, 0x00)
	ext = append(ext, 0xff, 0x01, 0x00, 0x01, 0x00)
	ext = append(ext, b.buildSupportedGroups()...)
	ext = append(ext, 0x00, 0x0b, 0x00, 0x02, 0x01, 0x00)
	ext = append(ext, 0x00, 0x23, 0x00, 0x00)
	ext = append(ext, b.buildALPN()...)
	ext = append(ext, 0x00, 0x05, 0x00, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00)
	ext = append(ext, b.buildSignatureAlgorithms()...)
	ext = append(ext, 0x00, 0x12, 0x00, 0x00)
	ext = append(ext, 0x00, 0x2b, 0x00, 0x05, 0x04, 0x03, 0x04, 0x03, 0x03)
	ext = append(ext, 0x00, 0x2d, 0x00, 0x02, 0x01, 0x01)
	ext = append(ext, b.buildKeyShare()...)
	ext = append(ext, b.buildPadding(len(ext))...)
	return ext
}

func (b *ClientHelloBuilder) buildSNI() []byte {
	nameLen := len(b.serverName)
	listLen := nameLen + 3
	extLen := listLen + 2
	ext := make([]byte, 4+extLen)
	binary.BigEndian.PutUint16(ext[0:2], 0x0000)
	binary.BigEndian.PutUint16(ext[2:4], uint16(extLen))
	binary.BigEndian.PutUint16(ext[4:6], uint16(listLen))
	ext[6] = 0x00
	binary.BigEndian.PutUint16(ext[7:9], uint16(nameLen))
	copy(ext[9:], b.serverName)
	return ext
}

func (b *ClientHelloBuilder) buildSupportedGroups() []byte {
	groupsLen := len(supportedGroups) * 2
	ext := make([]byte, 6+groupsLen)
	binary.BigEndian.PutUint16(ext[0:2], 0x000a)
	binary.BigEndian.PutUint16(ext[2:4], uint16(groupsLen+2))
	binary.BigEndian.PutUint16(ext[4:6], uint16(groupsLen))
	for i, g := range supportedGroups {
		binary.BigEndian.PutUint16(ext[6+i*2:], g)
	}
	return ext
}

func (b *ClientHelloBuilder) buildSignatureAlgorithms() []byte {
	algLen := len(signatureAlgorithms) * 2
	ext := make([]byte, 6+algLen)
	binary.BigEndian.PutUint16(ext[0:2], 0x000d)
	binary.BigEndian.PutUint16(ext[2:4], uint16(algLen+2))
	binary.BigEndian.PutUint16(ext[4:6], uint16(algLen))
	for i, alg := range signatureAlgorithms {
		binary.BigEndian.PutUint16(ext[6+i*2:], alg)
	}
	return ext
}

func (b *ClientHelloBuilder) buildALPN() []byte {
	protocols := []string{"h2", "http/1.1"}
	var listData []byte
	for _, p := range protocols {
		listData = append(listData, byte(len(p)))
		listData = append(listData, []byte(p)...)
	}
	ext := make([]byte, 6+len(listData))
	binary.BigEndian.PutUint16(ext[0:2], 0x0010)
	binary.BigEndian.PutUint16(ext[2:4], uint16(len(listData)+2))
	binary.BigEndian.PutUint16(ext[4:6], uint16(len(listData)))
	copy(ext[6:], listData)
	return ext
}

func (b *ClientHelloBuilder) buildKeyShare() []byte {
	publicKey := make([]byte, 32)
	rand.Read(publicKey)
	ext := make([]byte, 40)
	binary.BigEndian.PutUint16(ext[0:2], 0x0033)
	binary.BigEndian.PutUint16(ext[2:4], 36)
	binary.BigEndian.PutUint16(ext[4:6], 34)
	binary.BigEndian.PutUint16(ext[6:8], 0x001d)
	binary.BigEndian.PutUint16(ext[8:10], 32)
	copy(ext[10:], publicKey)
	return ext
}

func (b *ClientHelloBuilder) buildPadding(currentLen int) []byte {
	targetLen := 517
	if currentLen >= targetLen-4 {
		return nil
	}
	padLen := targetLen - currentLen - 4
	ext := make([]byte, 4+padLen)
	binary.BigEndian.PutUint16(ext[0:2], 0x0015)
	binary.BigEndian.PutUint16(ext[2:4], uint16(padLen))
	return ext
}
