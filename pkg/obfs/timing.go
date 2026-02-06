package obfs

import (
	"crypto/rand"
	"math/big"
	"time"
)

type TimingConfig struct {
	Enabled   bool
	MinJitter time.Duration
	MaxJitter time.Duration
	BurstMode bool
}

func DefaultTimingConfig() TimingConfig {
	return TimingConfig{
		Enabled:   true,
		MinJitter: 5 * time.Millisecond,
		MaxJitter: 100 * time.Millisecond,
		BurstMode: true,
	}
}

type TimingObfuscator struct {
	config TimingConfig
}

func NewTimingObfuscator(config TimingConfig) *TimingObfuscator {
	return &TimingObfuscator{config: config}
}

func (t *TimingObfuscator) Jitter() {
	if !t.config.Enabled {
		return
	}
	delay := randomDuration(t.config.MinJitter, t.config.MaxJitter)
	time.Sleep(delay)
}

func (t *TimingObfuscator) JitterRange(min, max time.Duration) {
	delay := randomDuration(min, max)
	time.Sleep(delay)
}

func (t *TimingObfuscator) SimulateHTTPTiming() {
	if !t.config.BurstMode {
		return
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(100))
	switch {
	case n.Int64() < 70:
		time.Sleep(randomDuration(1*time.Millisecond, 20*time.Millisecond))
	case n.Int64() < 90:
		time.Sleep(randomDuration(50*time.Millisecond, 200*time.Millisecond))
	default:
		time.Sleep(randomDuration(200*time.Millisecond, 500*time.Millisecond))
	}
}

type BurstScheduler struct {
	burstSize  int
	burstCount int
	lastBurst  time.Time
}

func NewBurstScheduler() *BurstScheduler {
	return &BurstScheduler{burstSize: randomInt(3, 10), burstCount: 0, lastBurst: time.Now()}
}

func (b *BurstScheduler) ShouldDelay() (bool, time.Duration) {
	b.burstCount++
	if b.burstCount >= b.burstSize {
		b.burstCount = 0
		b.burstSize = randomInt(3, 10)
		b.lastBurst = time.Now()
		return true, randomDuration(100*time.Millisecond, 300*time.Millisecond)
	}
	return true, randomDuration(1*time.Millisecond, 10*time.Millisecond)
}
