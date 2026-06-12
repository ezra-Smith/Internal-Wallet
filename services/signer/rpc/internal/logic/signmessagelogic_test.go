package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestSignMessageLogic_SignMessage(t *testing.T) {
	ctx := requireSignerSvcCtx(t)

	seedID := "test-signmsg-seed"
	createSeedForTest(t, ctx, seedID, "消息签名测试种子", []string{"ETH"}, 1)
	unlockSeedForTest(t, ctx, seedID)

	tests := []struct {
		name    string
		req     *pb.SignMessageRequest
		wantErr bool
	}{
		{
			name: "签名简单消息",
			req: &pb.SignMessageRequest{
				RequestId:      "msg-test-001",
				SeedId:         seedID,
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
				Message:        "Hello Crypto Exchange!",
			},
			wantErr: false,
		},
		{
			name: "签名中文消息",
			req: &pb.SignMessageRequest{
				RequestId:      "msg-test-002",
				SeedId:         seedID,
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
				Message:        "加密交易所测试消息",
			},
			wantErr: false,
		},
		{
			name: "缺少request_id",
			req: &pb.SignMessageRequest{
				RequestId:      "",
				SeedId:         seedID,
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
				Message:        "Test",
			},
			wantErr: true,
		},
		{
			name: "空消息",
			req: &pb.SignMessageRequest{
				RequestId:      "msg-test-003",
				SeedId:         seedID,
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
				Message:        "",
			},
			wantErr: true,
		},
		{
			name: "重复的request_id（幂等性测试）",
			req: &pb.SignMessageRequest{
				RequestId:      "msg-test-001",
				SeedId:         seedID,
				Chain:          "ETH",
				DerivationPath: "m/44'/60'/0'/0/0",
				Message:        "Hello Again",
			},
			wantErr: false, // 应返回缓存结果
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewSignMessageLogic(context.Background(), ctx)
			resp, err := logic.SignMessage(tt.req)

			assert.NoError(t, err)
			if tt.wantErr {
				assert.NotEqual(t, int32(0), resp.Code)
			} else {
				assert.Equal(t, int32(0), resp.Code)
				assert.NotEmpty(t, resp.Signature)
				assert.NotEmpty(t, resp.SignedAt)
				msgPreview := tt.req.Message
				if len(msgPreview) > 20 {
					msgPreview = msgPreview[:20] + "..."
				}
				sigPreview := resp.Signature
				if len(sigPreview) > 20 {
					sigPreview = sigPreview[:20] + "..."
				}
				t.Logf("Message: %s, Signature: %s", msgPreview, sigPreview)
			}
		})
	}
}
