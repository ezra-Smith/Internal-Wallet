package logic

import "time"

type hotWalletInitSession struct {
	InitSessionID  string    `json:"init_session_id"`
	SeedID         string    `json:"seed_id"`
	UnlockPassword string    `json:"unlock_password"` // 仅会话期内持有；不会写日志
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type hotWalletChallengeSession struct {
	ChallengeSessionID string    `json:"challenge_session_id"`
	SeedID             string    `json:"seed_id"`
	ChallengeIndices   []int32   `json:"challenge_indices"`
	ExpectedWordHashes []string  `json:"expected_word_hashes"` // 小写单词的 sha256
	Attempts           int       `json:"attempts"`
	MaxAttempts        int       `json:"max_attempts"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

const (
	hotWalletInitSessionKeyPrefix      = "wallet_init:session:"
	hotWalletChallengeSessionKeyPrefix = "wallet_init:challenge:"
)
