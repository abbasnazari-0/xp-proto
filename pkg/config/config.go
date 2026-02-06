package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode      string          `yaml:"mode"`
	Transport TransportConfig `yaml:"transport"`
	Server    ServerConfig    `yaml:"server"`
	Client    ClientConfig    `yaml:"client"`
}

// TransportConfig configures the transport layer
type TransportConfig struct {
	Mode string    `yaml:"mode"` // "tls", "kcp", "raw" (raw = ultimate stealth!)
	TLS  TLSConfig `yaml:"tls"`
	KCP  KCPConfig `yaml:"kcp"`
	Raw  RawConfig `yaml:"raw"`
}

// TLSConfig for TLS-based transport (default)
type TLSConfig struct {
	Fragment     bool `yaml:"fragment"`
	Padding      bool `yaml:"padding"`
	TimingJitter bool `yaml:"timing_jitter"`
}

// KCPConfig for KCP-based transport
type KCPConfig struct {
	Key          string `yaml:"key"`
	Salt         string `yaml:"salt"`
	Mode         string `yaml:"mode"` // normal, fast, fast2, fast3
	DataShards   int    `yaml:"data_shards"`
	ParityShards int    `yaml:"parity_shards"`
}

// RawConfig for raw packet transport (bypasses OS TCP stack!)
type RawConfig struct {
	Interface string   `yaml:"interface"`  // eth0, en0, etc.
	LocalIP   string   `yaml:"local_ip"`   // Your IP
	RouterMAC string   `yaml:"router_mac"` // Gateway MAC
	LocalMAC  string   `yaml:"local_mac"`  // Optional, auto-detected
	TCPFlags  []string `yaml:"tcp_flags"`  // ["PA", "A"] for flag cycling
	UseKCP    bool     `yaml:"use_kcp"`    // Use KCP over raw packets
}

type ServerConfig struct {
	Listen       string `yaml:"listen"`
	Key          string `yaml:"key"`
	FakeSite     string `yaml:"fake_site"`
	ProbeResist  bool   `yaml:"probe_resist"`
	FallbackSite string `yaml:"fallback_site"`
	Fragment     bool   `yaml:"fragment"`
	Padding      bool   `yaml:"padding"`
	TimingJitter bool   `yaml:"timing_jitter"`
}

type ClientConfig struct {
	ServerAddr   string `yaml:"server_addr"`
	Key          string `yaml:"key"`
	FakeSNI      string `yaml:"fake_sni"`
	SOCKSAddr    string `yaml:"socks_addr"`
	HTTPAddr     string `yaml:"http_addr"`
	Fragment     bool   `yaml:"fragment"`
	Padding      bool   `yaml:"padding"`
	TimingJitter bool   `yaml:"timing_jitter"`
	Fingerprint  string `yaml:"fingerprint"`
}

func DefaultServerConfig() Config {
	return Config{
		Mode: "server",
		Transport: TransportConfig{
			Mode: "tls",
			TLS: TLSConfig{
				Fragment:     true,
				Padding:      true,
				TimingJitter: true,
			},
			KCP: KCPConfig{
				Mode:         "fast2",
				DataShards:   10,
				ParityShards: 3,
			},
			Raw: RawConfig{
				TCPFlags: []string{"PA", "A"},
				UseKCP:   true,
			},
		},
		Server: ServerConfig{
			Listen:       "0.0.0.0:443",
			FakeSite:     "www.microsoft.com",
			ProbeResist:  true,
			FallbackSite: "www.microsoft.com",
			Fragment:     true,
			Padding:      true,
			TimingJitter: true,
		},
	}
}

func DefaultClientConfig() Config {
	return Config{
		Mode: "client",
		Transport: TransportConfig{
			Mode: "tls",
			TLS: TLSConfig{
				Fragment:     true,
				Padding:      true,
				TimingJitter: true,
			},
			KCP: KCPConfig{
				Mode:         "fast2",
				DataShards:   10,
				ParityShards: 3,
			},
			Raw: RawConfig{
				TCPFlags: []string{"PA", "A"},
				UseKCP:   true,
			},
		},
		Client: ClientConfig{
			FakeSNI:      "www.microsoft.com",
			SOCKSAddr:    "127.0.0.1:1080",
			Fragment:     true,
			Padding:      true,
			TimingJitter: true,
			Fingerprint:  "chrome",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}

func SaveConfig(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func (c *ServerConfig) GetKey() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.Key)
}

func (c *ClientConfig) GetKey() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.Key)
}

func GenerateKeyString() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func GenerateExampleConfig(mode string) string {
	if mode == "server" {
		return `# XP Protocol Server Configuration
mode: server

server:
  listen: "0.0.0.0:443"
  key: "YOUR_BASE64_KEY_HERE"
  fake_site: "www.microsoft.com"
  probe_resist: true
  fallback_site: "www.microsoft.com"
  fragment: true
  padding: true
  timing_jitter: true
`
	}
	return `# XP Protocol Client Configuration
mode: client

client:
  server_addr: "your-server.com:443"
  key: "YOUR_BASE64_KEY_HERE"
  fake_sni: "www.microsoft.com"
  socks_addr: "127.0.0.1:1080"
  http_addr: "127.0.0.1:8080"
  fragment: true
  padding: true
  timing_jitter: true
  fingerprint: "chrome"
`
}
