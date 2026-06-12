package logic

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"internalwallet/proto/pb"

	"github.com/stretchr/testify/assert"
)

func TestSignTransactionLogic_SignTransaction(t *testing.T) {
	if os.Getenv("SIGNER_E2E") != "1" {
		t.Skip("skip signer e2e test (set SIGNER_E2E=1 to enable)")
	}
	ctx := requireSignerSvcCtx(t)

	// 生成正确的测试交易数据
	mockEthRawTxBytes, err := CreateMockETHTransaction()
	if err != nil {
		t.Fatalf("Failed to create mock ETH transaction: %v", err)
	}
	mockEthRawTx := "0x" + fmt.Sprintf("%x", mockEthRawTxBytes)
	t.Logf("ETH Transaction hex: %s", mockEthRawTx)

	mockBscRawTxBytes, err := CreateMockBSCTransaction()
	if err != nil {
		t.Fatalf("Failed to create mock BSC transaction: %v", err)
	}
	mockBscRawTx := "0x" + fmt.Sprintf("%x", mockBscRawTxBytes)
	t.Logf("BSC Transaction hex: %s", mockBscRawTx)

	tests := []struct {
		name    string
		req     *pb.SignTransactionRequest
		wantErr bool
	}{
		{
			name: "签名ETH提币交易 - 使用默认热钱包",
			req: &pb.SignTransactionRequest{
				RequestId:      fmt.Sprintf("tx-test-eth-default-%d", time.Now().UnixNano()),
				Chain:          "ETH",
				RawTransaction: mockEthRawTx,
				OperationType:  2, // 提币
				ToAddress:      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
				Amount:         "0.1",
				AssetSymbol:    "ETH",
				TokenContract:  "",
				Requester:      "signer_service",
				// 注意: 不指定 FromAddress，使用默认热钱包
			},
			wantErr: false,
		},
		{
			name: "签名BSC提币交易 - 使用默认热钱包",
			req: &pb.SignTransactionRequest{
				RequestId:      fmt.Sprintf("tx-test-bsc-default-%d", time.Now().UnixNano()),
				Chain:          "BSC",
				RawTransaction: mockBscRawTx,
				OperationType:  2, // 提币
				ToAddress:      "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
				Amount:         "0.1",
				AssetSymbol:    "BNB",
				TokenContract:  "",
				Requester:      "signer_service",
				// 注意: 不指定 FromAddress，使用默认热钱包
			},
			wantErr: false,
		},
		{
			name: "签名ETH提币交易 - 指定已存在的公司钱包地址",
			req: &pb.SignTransactionRequest{
				RequestId:      fmt.Sprintf("tx-test-eth-company-%d", time.Now().UnixNano()),
				Chain:          "ETH",
				FromAddress:    "0x095003Fdc16cc8C724e3D412761a30AE1448A3f3", // 使用默认热钱包地址（在数据库中存在）
				RawTransaction: mockEthRawTx,
				OperationType:  2, // 提币
				ToAddress:      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
				Amount:         "0.1",
				AssetSymbol:    "ETH",
				TokenContract:  "",
				Requester:      "signer_service",
			},
			wantErr: false, // 地址存在且有效，应该成功
		},
		{
			name: "签名ETH提币交易 - 指定不存在的地址",
			req: &pb.SignTransactionRequest{
				RequestId:      fmt.Sprintf("tx-test-eth-notfound-%d", time.Now().UnixNano()),
				Chain:          "ETH",
				FromAddress:    "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", // 不存在的地址
				RawTransaction: mockEthRawTx,
				OperationType:  2, // 提币
				ToAddress:      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
				Amount:         "0.1",
				AssetSymbol:    "ETH",
				TokenContract:  "",
				Requester:      "signer_service",
			},
			wantErr: true, // 地址不存在，应该失败
		},
		{
			name: "参数验证 - 缺少必填字段",
			req: &pb.SignTransactionRequest{
				RequestId:      "",
				Chain:          "ETH",
				RawTransaction: mockEthRawTx,
				OperationType:  2,
			},
			wantErr: true, // request_id 为空，应该失败
		},
		{
			name: "参数验证 - 不支持的链",
			req: &pb.SignTransactionRequest{
				RequestId:      fmt.Sprintf("tx-test-invalid-%d", time.Now().UnixNano()),
				Chain:          "INVALID_CHAIN",
				RawTransaction: mockEthRawTx,
				OperationType:  2,
			},
			wantErr: true, // 不支持的链类型，应该失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logic := NewSignTransactionLogic(context.Background(), ctx)
			resp, err := logic.SignTransaction(tt.req)

			// SignTransaction 不会返回 error，而是在 Response 中返回错误码
			assert.NoError(t, err)
			assert.NotNil(t, resp)

			if tt.wantErr {
				// 期望失败：检查响应码不为 0
				assert.NotEqual(t, int32(0), resp.Code, "Expected error response, but got success")
				t.Logf("Expected error occurred: code=%d, message=%s", resp.Code, resp.Message)
			} else {
				// 期望成功：检查响应码为 0
				assert.Equal(t, int32(0), resp.Code, "Expected success, but got error: code=%d, message=%s", resp.Code, resp.Message)
				assert.NotEmpty(t, resp.Signature, "Signature should not be empty")
				assert.NotEmpty(t, resp.TxHash, "TxHash should not be empty")
				assert.NotEmpty(t, resp.SignedAt, "SignedAt should not be empty")
				assert.Equal(t, tt.req.RequestId, resp.RequestId, "RequestId should match")
				assert.Equal(t, tt.req.Chain, resp.Chain, "Chain should match")

				// 安全的切片操作：检查长度
				if len(resp.Signature) > 20 {
					t.Logf("✓ Signature successful - RequestId: %s, TxHash: %s, Signature: %s...",
						resp.RequestId, resp.TxHash, resp.Signature[:20])
				} else {
					t.Logf("✓ Signature successful - RequestId: %s, TxHash: %s, Signature: %s",
						resp.RequestId, resp.TxHash, resp.Signature)
				}
			}
		})
	}
}
