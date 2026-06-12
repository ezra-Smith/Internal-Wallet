package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestUpdateSeedStatusLogic_UpdateSeedStatus(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	// 先创建一个测试种子
	createSeedForTest(t, ctx, "test-status-seed", "状态测试种子", []string{"ETH"}, 1)

	tests := []struct {
		name    string
		req     *pb.UpdateSeedStatusRequest
		wantErr bool
	}{
		{
			name: "停用种子",
			req: &pb.UpdateSeedStatusRequest{
				SeedId: "test-status-seed",
				Status: 2, // 停用
				Reason: "测试停用功能",
			},
			wantErr: false,
		},
		{
			name: "重新激活种子",
			req: &pb.UpdateSeedStatusRequest{
				SeedId: "test-status-seed",
				Status: 1, // 激活
				Reason: "恢复使用",
			},
			wantErr: false,
		},
		{
			name: "废弃种子",
			req: &pb.UpdateSeedStatusRequest{
				SeedId: "test-status-seed",
				Status: 3, // 废弃
				Reason: "不再使用",
			},
			wantErr: false,
		},
		{
			name: "种子不存在",
			req: &pb.UpdateSeedStatusRequest{
				SeedId: "non-existent-seed",
				Status: 2,
				Reason: "测试",
			},
			wantErr: true,
		},
		{
			name: "无效的状态值",
			req: &pb.UpdateSeedStatusRequest{
				SeedId: "test-status-seed",
				Status: 99, // 无效状态
				Reason: "测试",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewUpdateSeedStatusLogic(context.Background(), ctx)
			resp, err := logic.UpdateSeedStatus(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.Equal(t, tt.req.SeedId, resp.SeedId)
				assert.Equal(t, tt.req.Status, resp.Status)
				assert.NotEmpty(t, resp.UpdatedAt)
			}
		})
	}
}
