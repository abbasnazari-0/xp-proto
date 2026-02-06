package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

type XPCrypto struct {
	aead  cipher.AEAD
	nonce uint64
	key   []byte
}

func DeriveKey(password string, salt []byte) []byte {
	hash := sha256.New
	hkdfReader := hkdf.New(hash, []byte(password), salt, []byte("xp-proto-v1"))
	key := make([]byte, chacha20poly1305.KeySize)
	io.ReadFull(hkdfReader, key)
	return key
}

func NewXPCrypto(key []byte) (*XPCrypto, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, errors.New("key must be 32 bytes")
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return &XPCrypto{aead: aead, nonce: 0, key: key}, nil
}

func (c *XPCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	binary.LittleEndian.PutUint64(nonce, c.nonce)
	c.nonce++
	if _, err := rand.Read(nonce[8:]); err != nil {
		return nil, err
	}
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (c *XPCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < chacha20poly1305.NonceSizeX {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:chacha20poly1305.NonceSizeX]
	ciphertext = ciphertext[chacha20poly1305.NonceSizeX:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}
