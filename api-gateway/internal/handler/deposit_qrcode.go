package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"

	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/core/logx"
)

type QRCodeUploader interface {
	DepositQRCodeObjectKey(assetCode string, chainCode string, addressHash string, ext string) string
	Upload(ctx context.Context, objectKey string, contentType string, data []byte) (string, error)
}

type QRCodeCache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, value string) error
}

type redisQRCodeCache struct {
	client *redis.Client
}

func newRedisQRCodeCache(client *redis.Client) *redisQRCodeCache {
	if client == nil {
		return nil
	}
	return &redisQRCodeCache{client: client}
}

func (c *redisQRCodeCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *redisQRCodeCache) Set(ctx context.Context, key string, value string) error {
	// Permanent cache (no TTL): user requested Redis entries to never expire.
	return c.client.Set(ctx, key, value, 0).Err()
}

type depositQRCodeCacheMeta struct {
	DataHash  string `json:"data_hash"`
	ObjectKey string `json:"object_key"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updated_at"`
}

func depositQRCodeURLKey(userID string, asset string, chain string) string {
	return "qrcode:url:" + userID + ":" + asset + ":" + chain
}

func depositQRCodeMetaKey(userID string, asset string, chain string) string {
	return "qrcode:meta:" + userID + ":" + asset + ":" + chain
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func enhanceCreateDepositQRCode(ctx context.Context, uploader QRCodeUploader, cache QRCodeCache, resp *pb.CreateDepositResp) {
	if resp == nil {
		return
	}

	// If upstream already provides qrcode_url, keep it.
	if strings.TrimSpace(resp.QrcodeUrl) != "" {
		return
	}

	userID := strings.TrimSpace(middleware.GetUserID(ctx))
	asset := strings.ToUpper(strings.TrimSpace(resp.Asset))
	chain := strings.ToUpper(strings.TrimSpace(resp.Chain))
	if userID == "" || asset == "" || chain == "" {
		return
	}

	qrData := strings.TrimSpace(resp.QrcodeData)
	if qrData == "" {
		qrData = strings.TrimSpace(resp.Address)
	}
	if qrData == "" {
		return
	}

	dataHash := sha256Hex(qrData)
	urlKey := depositQRCodeURLKey(userID, asset, chain)
	metaKey := depositQRCodeMetaKey(userID, asset, chain)
	expectedObjectKey := ""
	if uploader != nil {
		expectedObjectKey = uploader.DepositQRCodeObjectKey(asset, chain, dataHash, "png")
	}

	// Fast path: cache hit.
	if cache != nil {
		metaStr, ok, err := cache.Get(ctx, metaKey)
		if err != nil {
			logx.WithContext(ctx).Debugf("deposit qrcode cache meta get failed: user_id=%s asset=%s chain=%s hash=%s err=%v", userID, asset, chain, dataHash, err)
		} else if ok && strings.TrimSpace(metaStr) != "" {
			var meta depositQRCodeCacheMeta
			if jsonErr := json.Unmarshal([]byte(metaStr), &meta); jsonErr == nil && meta.DataHash == dataHash {
				// If uploader exists, ensure cached object key matches current naming scheme (e.g., path/prefix changes).
				if expectedObjectKey != "" && strings.TrimSpace(meta.ObjectKey) != expectedObjectKey {
					// Treat as stale: fall through to regenerate/upload.
				} else {
					if url, ok, err := cache.Get(ctx, urlKey); err == nil && ok && strings.TrimSpace(url) != "" {
						resp.QrcodeUrl = url
						return
					}
					if strings.TrimSpace(meta.URL) != "" {
						resp.QrcodeUrl = strings.TrimSpace(meta.URL)
						return
					}
				}
			}
		}
	}

	// Best-effort: generate + upload.
	if uploader == nil {
		return
	}

	png, err := qrcode.Encode(qrData, qrcode.Medium, 256)
	if err != nil {
		logx.WithContext(ctx).Errorf("deposit qrcode encode failed: user_id=%s asset=%s chain=%s hash=%s err=%v", userID, asset, chain, dataHash, err)
		return
	}

	objectKey := uploader.DepositQRCodeObjectKey(asset, chain, dataHash, "png")
	url, err := uploader.Upload(ctx, objectKey, "image/png", png)
	if err != nil {
		logx.WithContext(ctx).Errorf("deposit qrcode upload failed: user_id=%s asset=%s chain=%s hash=%s err=%v", userID, asset, chain, dataHash, err)
		return
	}

	resp.QrcodeUrl = url

	// Best-effort: cache set (permanent).
	if cache != nil {
		if err := cache.Set(ctx, urlKey, url); err != nil {
			logx.WithContext(ctx).Debugf("deposit qrcode cache url set failed: user_id=%s asset=%s chain=%s hash=%s err=%v", userID, asset, chain, dataHash, err)
		}
		meta := depositQRCodeCacheMeta{
			DataHash:  dataHash,
			ObjectKey: objectKey,
			URL:       url,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if metaJSON, err := json.Marshal(meta); err == nil {
			if err := cache.Set(ctx, metaKey, string(metaJSON)); err != nil {
				logx.WithContext(ctx).Debugf("deposit qrcode cache meta set failed: user_id=%s asset=%s chain=%s hash=%s err=%v", userID, asset, chain, dataHash, err)
			}
		}
	}
}
