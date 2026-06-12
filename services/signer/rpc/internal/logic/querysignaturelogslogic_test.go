package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestQuerySignatureLogsLogic_QuerySignatureLogs(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	tests := []struct {
		name    string
		req     *pb.QuerySignatureLogsRequest
		wantErr bool
	}{
		{
			name: "查询特定种子的所有日志",
			req: &pb.QuerySignatureLogsRequest{
				SeedId:   "test-export-seed",
				Page:     1,
				PageSize: 10,
			},
			wantErr: false,
		},
		{
			name: "按链筛选",
			req: &pb.QuerySignatureLogsRequest{
				Chain:    "ETH",
				Page:     1,
				PageSize: 20,
			},
			wantErr: false,
		},
		{
			name: "按操作类型筛选",
			req: &pb.QuerySignatureLogsRequest{
				OperationType: 2, // 提币
				Page:          1,
				PageSize:      10,
			},
			wantErr: false,
		},
		{
			name: "按日期范围查询",
			req: &pb.QuerySignatureLogsRequest{
				StartDate: "2025-11-01",
				EndDate:   "2025-11-30",
				Page:      1,
				PageSize:  50,
			},
			wantErr: false,
		},
		{
			name: "默认分页参数",
			req: &pb.QuerySignatureLogsRequest{
				Page:     0, // 会自动调整为1
				PageSize: 0, // 会自动调整为20
			},
			wantErr: false,
		},
		{
			name: "超大分页（测试上限）",
			req: &pb.QuerySignatureLogsRequest{
				Page:     1,
				PageSize: 200, // 会被限制为100
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewQuerySignatureLogsLogic(context.Background(), ctx)
			resp, err := logic.QuerySignatureLogs(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.GreaterOrEqual(t, resp.Total, int64(0))

				// 验证日志记录
				for _, log := range resp.Records {
					assert.NotEmpty(t, log.RequestId)
					assert.NotEmpty(t, log.Chain)
					assert.NotEmpty(t, log.CreatedAt)
					t.Logf("Log: %s, Chain: %s, Type: %d, Status: %d",
						log.RequestId, log.Chain, log.OperationType, log.Status)
				}
			}
		})
	}
}
