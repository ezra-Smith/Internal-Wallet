package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"internalwallet/proto/pb"
)

type fakeQRCodeCache struct {
	data   map[string]string
	getErr error
	setErr error
}

func (c *fakeQRCodeCache) Get(ctx context.Context, key string) (string, bool, error) {
	_ = ctx
	if c.getErr != nil {
		return "", false, c.getErr
	}
	v, ok := c.data[key]
	return v, ok, nil
}

func (c *fakeQRCodeCache) Set(ctx context.Context, key string, value string) error {
	_ = ctx
	if c.setErr != nil {
		return c.setErr
	}
	if c.data == nil {
		c.data = make(map[string]string)
	}
	c.data[key] = value
	return nil
}

type fakeQRCodeUploader struct {
	url       string
	uploadErr error

	uploadCalls   int
	lastObjectKey string
	lastType      string
	lastData      []byte
}

func (u *fakeQRCodeUploader) DepositQRCodeObjectKey(assetCode string, chainCode string, addressHash string, ext string) string {
	_ = ext
	return "currencies/icons/qrcode/deposit/" + assetCode + "/" + chainCode + "/" + addressHash + ".png"
}

func (u *fakeQRCodeUploader) Upload(ctx context.Context, objectKey string, contentType string, data []byte) (string, error) {
	_ = ctx
	u.uploadCalls++
	u.lastObjectKey = objectKey
	u.lastType = contentType
	u.lastData = data
	if u.uploadErr != nil {
		return "", u.uploadErr
	}
	return u.url, nil
}

func TestEnhanceCreateDepositQRCode_CacheHit(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	hash := sha256Hex("T-ADDR")
	url := "https://cdn.example.com/qrcode.png"
	meta := depositQRCodeCacheMeta{
		DataHash:  hash,
		ObjectKey: "currencies/icons/qrcode/deposit/USDT/TRON/" + hash + ".png",
		URL:       url,
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}

	cache := &fakeQRCodeCache{
		data: map[string]string{
			depositQRCodeURLKey("123", "USDT", "TRON"):  url,
			depositQRCodeMetaKey("123", "USDT", "TRON"): string(metaJSON),
		},
	}
	uploader := &fakeQRCodeUploader{url: "https://should-not-upload.example.com/qrcode.png"}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl != url {
		t.Fatalf("expected qrcode_url=%q, got %q", url, resp.QrcodeUrl)
	}
	if uploader.uploadCalls != 0 {
		t.Fatalf("expected no upload, got %d upload calls", uploader.uploadCalls)
	}
}

func TestEnhanceCreateDepositQRCode_CacheMiss_UploadsAndCaches(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	hash := sha256Hex("T-ADDR")
	url := "https://cdn.example.com/qrcode.png"
	cache := &fakeQRCodeCache{data: map[string]string{}}
	uploader := &fakeQRCodeUploader{url: url}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl != url {
		t.Fatalf("expected qrcode_url=%q, got %q", url, resp.QrcodeUrl)
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("expected 1 upload call, got %d", uploader.uploadCalls)
	}
	if uploader.lastType != "image/png" {
		t.Fatalf("expected content_type=image/png, got %q", uploader.lastType)
	}
	if len(uploader.lastData) < 8 || string(uploader.lastData[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG data")
	}

	cachedURL, ok := cache.data[depositQRCodeURLKey("123", "USDT", "TRON")]
	if !ok || cachedURL != url {
		t.Fatalf("expected cached url to be set, got ok=%v url=%q", ok, cachedURL)
	}
	metaStr, ok := cache.data[depositQRCodeMetaKey("123", "USDT", "TRON")]
	if !ok || metaStr == "" {
		t.Fatalf("expected cached meta to be set")
	}
	var meta depositQRCodeCacheMeta
	if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.DataHash != hash {
		t.Fatalf("expected meta.data_hash=%q, got %q", hash, meta.DataHash)
	}
	if meta.URL != url {
		t.Fatalf("expected meta.url=%q, got %q", url, meta.URL)
	}
}

func TestEnhanceCreateDepositQRCode_CacheMetaMismatch_Reuploads(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	hash := sha256Hex("T-ADDR")
	url := "https://cdn.example.com/qrcode.png"
	meta := depositQRCodeCacheMeta{
		DataHash:  "different",
		ObjectKey: "currencies/icons/qrcode/deposit/USDT/TRON/different.png",
		URL:       "https://cdn.example.com/old.png",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}

	cache := &fakeQRCodeCache{
		data: map[string]string{
			depositQRCodeURLKey("123", "USDT", "TRON"):  "https://cdn.example.com/old.png",
			depositQRCodeMetaKey("123", "USDT", "TRON"): string(metaJSON),
		},
	}
	uploader := &fakeQRCodeUploader{url: url}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl != url {
		t.Fatalf("expected qrcode_url=%q, got %q", url, resp.QrcodeUrl)
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("expected 1 upload call, got %d", uploader.uploadCalls)
	}

	metaStr, ok := cache.data[depositQRCodeMetaKey("123", "USDT", "TRON")]
	if !ok || metaStr == "" {
		t.Fatalf("expected cached meta to be set")
	}
	var updated depositQRCodeCacheMeta
	if err := json.Unmarshal([]byte(metaStr), &updated); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if updated.DataHash != hash {
		t.Fatalf("expected updated meta.data_hash=%q, got %q", hash, updated.DataHash)
	}
}

func TestEnhanceCreateDepositQRCode_CacheObjectKeyMismatch_Reuploads(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	hash := sha256Hex("T-ADDR")
	url := "https://cdn.example.com/qrcode.png"
	meta := depositQRCodeCacheMeta{
		DataHash:  hash,
		ObjectKey: "qrcode/deposit/USDT/TRON/" + hash + ".png", // legacy path
		URL:       "https://cdn.example.com/old.png",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}

	cache := &fakeQRCodeCache{
		data: map[string]string{
			depositQRCodeURLKey("123", "USDT", "TRON"):  "https://cdn.example.com/old.png",
			depositQRCodeMetaKey("123", "USDT", "TRON"): string(metaJSON),
		},
	}
	uploader := &fakeQRCodeUploader{url: url}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl != url {
		t.Fatalf("expected qrcode_url=%q, got %q", url, resp.QrcodeUrl)
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("expected 1 upload call, got %d", uploader.uploadCalls)
	}
}

func TestEnhanceCreateDepositQRCode_UploadFailure_LeavesEmptyURL(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	cache := &fakeQRCodeCache{data: map[string]string{}}
	uploader := &fakeQRCodeUploader{uploadErr: errors.New("upload failed")}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl != "" {
		t.Fatalf("expected qrcode_url to remain empty, got %q", resp.QrcodeUrl)
	}
}

func TestEnhanceCreateDepositQRCode_RedisGetError_FallsBackToUpload(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "123")
	resp := &pb.CreateDepositResp{
		Asset:      "USDT",
		Chain:      "TRON",
		Address:    "T-ADDR",
		QrcodeData: "T-ADDR",
		QrcodeUrl:  "",
	}

	cache := &fakeQRCodeCache{
		data:   map[string]string{},
		getErr: errors.New("redis down"),
	}
	uploader := &fakeQRCodeUploader{url: "https://cdn.example.com/qrcode.png"}

	enhanceCreateDepositQRCode(ctx, uploader, cache, resp)

	if resp.QrcodeUrl == "" {
		t.Fatalf("expected qrcode_url to be set")
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("expected 1 upload call, got %d", uploader.uploadCalls)
	}
}
