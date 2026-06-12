package logic

import (
	"context"
	"fmt"
	"testing"
	"time"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestExportPublicKeyBatchLogic_ExportPublicKeyBatch(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	// 先创建测试种子
	seedId := fmt.Sprintf("test-batch-seed-%d", time.Now().UnixNano())
	createSeedForTest(t, ctx, seedId, "批量导出测试种子", []string{"ETH", "BTC"}, 1)
	unlockSeedForTest(t, ctx, seedId)

	tests := []struct {
		name      string
		req       *pb.ExportPublicKeyBatchRequest
		wantErr   bool
		wantCount int
	}{
		{
			name: "批量导出10个ETH地址",
			req: &pb.ExportPublicKeyBatchRequest{
				SeedId:     seedId,
				Chain:      "ETH",
				StartIndex: 0,
				Count:      10,
			},
			wantErr:   false,
			wantCount: 10,
		},
		{
			name: "批量导出5个BTC地址",
			req: &pb.ExportPublicKeyBatchRequest{
				SeedId:     seedId,
				Chain:      "BTC",
				StartIndex: 0,
				Count:      5,
			},
			wantErr:   false,
			wantCount: 5,
		},
		{
			name: "超出最大数量限制",
			req: &pb.ExportPublicKeyBatchRequest{
				SeedId:     seedId,
				Chain:      "ETH",
				StartIndex: 0,
				Count:      1001, // 超过1000
			},
			wantErr: true,
		},
		{
			name: "无效的数量",
			req: &pb.ExportPublicKeyBatchRequest{
				SeedId:     seedId,
				Chain:      "ETH",
				StartIndex: 0,
				Count:      0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewExportPublicKeyBatchLogic(context.Background(), ctx)
			resp, err := logic.ExportPublicKeyBatch(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.Equal(t, tt.wantCount, len(resp.PublicKeys))

				// 验证每个地址的连续性
				for i, pk := range resp.PublicKeys {
					assert.Equal(t, int32(i), pk.Index)
					assert.NotEmpty(t, pk.PublicKey)
					assert.NotEmpty(t, pk.CompressedPubkey)
				}

				t.Logf("成功导出 %d 个 %s 地址", len(resp.PublicKeys), resp.Chain)
			}
		})
	}
}
