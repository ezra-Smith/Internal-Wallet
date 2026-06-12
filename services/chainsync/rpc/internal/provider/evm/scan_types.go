package evm

import "time"

// BatchConfig controls batched block scanning.
type BatchConfig struct {
	Enabled          bool
	MaxBatchSize     int
	MaxConcurrency   int
	BatchTimeout     time.Duration
	RetryAttempts    int
	ProgressInterval time.Duration
}

type BatchRange struct {
	Start uint64
	End   uint64
}

