package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
)

// MD5Hash MD5哈希
func MD5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SHA256Hash SHA256哈希
func SHA256Hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
