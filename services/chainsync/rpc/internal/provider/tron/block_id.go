package tron

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
)

// TRON Block ID:
// - first 8 bytes: block number (big-endian)
// - last 24 bytes: last 24 bytes of SHA256(BlockHeader.RawData)
func (t *TronProvider) calculateTronBlockIDFromHeader(header *core.BlockHeader) string {
	if header == nil || header.RawData == nil {
		return ""
	}

	blockNumber := header.RawData.Number

	rawDataBytes, err := proto.Marshal(header.RawData)
	if err != nil {
		logx.Errorf("Failed to marshal BlockHeader.RawData: %v", err)
		return t.fallbackBlockID(blockNumber)
	}

	hash := sha256.Sum256(rawDataBytes)

	blockID := make([]byte, 32)
	blockID[0] = byte(blockNumber >> 56)
	blockID[1] = byte(blockNumber >> 48)
	blockID[2] = byte(blockNumber >> 40)
	blockID[3] = byte(blockNumber >> 32)
	blockID[4] = byte(blockNumber >> 24)
	blockID[5] = byte(blockNumber >> 16)
	blockID[6] = byte(blockNumber >> 8)
	blockID[7] = byte(blockNumber)

	copy(blockID[8:], hash[8:])
	return hex.EncodeToString(blockID)
}

func (t *TronProvider) fallbackBlockID(blockNumber int64) string {
	blockID := make([]byte, 32)
	blockID[0] = byte(blockNumber >> 56)
	blockID[1] = byte(blockNumber >> 48)
	blockID[2] = byte(blockNumber >> 40)
	blockID[3] = byte(blockNumber >> 32)
	blockID[4] = byte(blockNumber >> 24)
	blockID[5] = byte(blockNumber >> 16)
	blockID[6] = byte(blockNumber >> 8)
	blockID[7] = byte(blockNumber)

	data := []byte(fmt.Sprintf("TRON_BLOCK_%d", blockNumber))
	hash := sha256.Sum256(data)
	copy(blockID[8:], hash[8:])

	return hex.EncodeToString(blockID)
}

