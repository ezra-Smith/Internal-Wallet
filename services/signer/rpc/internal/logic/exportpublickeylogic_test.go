package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestExportPublicKeyLogic_ExportPublicKey(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	// 先创建一个测试种子
	createSeedForTest(t, ctx, "test-export-seed", "导出公钥测试种子", []string{"ETH", "BTC"}, 1)
	unlockSeedForTest(t, ctx, "test-export-seed")

	tests := []struct {
		name    string
		req     *pb.ExportPublicKeyRequest
		wantErr bool
	}{
		{
			name: "导出ETH地址",
			req: &pb.ExportPublicKeyRequest{
				SeedId:         "test-export-seed",
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
			},
			wantErr: false,
		},
		{
			name: "导出BTC地址",
			req: &pb.ExportPublicKeyRequest{
				SeedId:         "test-export-seed",
				Chain:          "BTC",
				DerivationPath: "m/44'/0'/0'/0/0",
			},
			wantErr: false,
		},
		{
			name: "种子不存在",
			req: &pb.ExportPublicKeyRequest{
				SeedId:         "non-existent-seed",
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
			},
			wantErr: true,
		},
		{
			name: "无效的派生路径",
			req: &pb.ExportPublicKeyRequest{
				SeedId:         "test-export-seed",
				Chain:          "ETH",
				DerivationPath: "invalid/path",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewExportPublicKeyLogic(context.Background(), ctx)
			resp, err := logic.ExportPublicKey(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.NotEmpty(t, resp.PublicKey)
				assert.NotEmpty(t, resp.CompressedPubkey)
				t.Logf("Chain: %s, Pubkey: %s, Compressed: %s", resp.Chain, resp.PublicKey[:20]+"...", resp.CompressedPubkey[:20]+"...")
			}
		})
	}
}
