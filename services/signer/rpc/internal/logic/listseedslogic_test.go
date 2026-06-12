package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestListSeedsLogic_ListSeeds(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	tests := []struct {
		name     string
		req      *pb.ListSeedsRequest
		wantErr  bool
		minSeeds int // 期望的最小种子数量
	}{
		{
			name: "查询所有种子",
			req: &pb.ListSeedsRequest{
				Status: 0, // 0表示查询所有状态
			},
			wantErr:  false,
			minSeeds: 1,
		},
		{
			name: "只查询激活的种子",
			req: &pb.ListSeedsRequest{
				Status: 1, // 1=激活
			},
			wantErr:  false,
			minSeeds: 0,
		},
		{
			name: "查询停用的种子",
			req: &pb.ListSeedsRequest{
				Status: 2, // 2=停用
			},
			wantErr:  false,
			minSeeds: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewListSeedsLogic(context.Background(), ctx)
			resp, err := logic.ListSeeds(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.GreaterOrEqual(t, len(resp.Seeds), tt.minSeeds)

				// 验证每个种子的字段
				for _, seed := range resp.Seeds {
					assert.NotEmpty(t, seed.SeedId)
					assert.NotEmpty(t, seed.SeedName)
					assert.NotEmpty(t, seed.SupportedChains)
					assert.NotEmpty(t, seed.CreatedAt)
					t.Logf("Seed: %s, Name: %s, Chains: %v, Signatures: %d",
						seed.SeedId, seed.SeedName, seed.SupportedChains, seed.TotalSignatures)
				}
			}
		})
	}
}
