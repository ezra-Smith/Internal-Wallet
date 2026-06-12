package binance

import (
	"math/rand"
	"time"

	"internalwallet/services/market/rpc/internal/config"
)

type backoff struct {
	initial    time.Duration
	max        time.Duration
	multiplier float64
	jitter     float64

	current time.Duration
	rng     *rand.Rand
}

func newBackoff(cfg config.ReconnectConfig) *backoff {
	return &backoff{
		initial:    time.Duration(cfg.InitialBackoffMillis) * time.Millisecond,
		max:        time.Duration(cfg.MaxBackoffMillis) * time.Millisecond,
		multiplier: cfg.Multiplier,
		jitter:     cfg.Jitter,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *backoff) Reset() {
	b.current = 0
}

func (b *backoff) NextDelay() time.Duration {
	if b.current <= 0 {
		b.current = b.initial
	} else {
		next := time.Duration(float64(b.current) * b.multiplier)
		if next < b.initial {
			next = b.initial
		}
		if next > b.max {
			next = b.max
		}
		b.current = next
	}

	delay := b.current
	if b.jitter <= 0 {
		return delay
	}

	// Apply +/- jitter percentage.
	j := b.jitter
	if j > 1 {
		j = 1
	}
	if j < 0 {
		j = 0
	}
	delta := float64(delay) * j
	if delta <= 0 {
		return delay
	}

	offset := (b.rng.Float64()*2 - 1) * delta
	jittered := time.Duration(float64(delay) + offset)
	if jittered < 0 {
		return 0
	}
	return jittered
}
