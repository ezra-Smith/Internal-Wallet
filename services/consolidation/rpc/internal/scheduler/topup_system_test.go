package scheduler

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"internalwallet/common/model"
	"internalwallet/pkg/chainnode"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"
	"internalwallet/services/signer/rpc/signerservice"
)

type memoryTopUpRepo struct {
	repository.TopUpRecordRepository

	mu      sync.Mutex
	nextID  int64
	records map[int64]*models.ConsolidationTopUpRecord
	byTask  map[string][]int64
}

func newMemoryTopUpRepo() *memoryTopUpRepo {
	return &memoryTopUpRepo{
		records: make(map[int64]*models.ConsolidationTopUpRecord),
		byTask:  make(map[string][]int64),
	}
}

func (r *memoryTopUpRepo) CreateIgnoreDuplicate(ctx context.Context, rec *models.ConsolidationTopUpRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range r.byTask[rec.TaskID] {
		existing := r.records[id]
		if existing == nil {
			continue
		}
		if existing.Chain == rec.Chain && existing.ToAddress == rec.ToAddress && existing.Purpose == rec.Purpose {
			if existing.Status != models.ConsolidationTopUpStatusFailed && existing.Status != models.ConsolidationTopUpStatusConfirmed {
				return false, nil
			}
		}
	}

	r.nextID++
	rec.ID = r.nextID
	cp := *rec
	r.records[cp.ID] = &cp
	r.byTask[cp.TaskID] = append(r.byTask[cp.TaskID], cp.ID)
	return true, nil
}

func (r *memoryTopUpRepo) GetByID(ctx context.Context, id int64) (*models.ConsolidationTopUpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.records[id]
	if rec == nil {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

func (r *memoryTopUpRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.ConsolidationTopUpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.byTask[taskID]
	out := make([]models.ConsolidationTopUpRecord, 0, len(ids))
	for _, id := range ids {
		if rec := r.records[id]; rec != nil {
			out = append(out, *rec)
		}
	}
	return out, nil
}

func (r *memoryTopUpRepo) SoftDeleteByTaskID(ctx context.Context, taskID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.byTask[taskID]
	var affected int64
	for _, id := range ids {
		if rec := r.records[id]; rec != nil {
			delete(r.records, id)
			affected++
		}
	}
	delete(r.byTask, taskID)
	return affected, nil
}

func (r *memoryTopUpRepo) GetPendingRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.ConsolidationTopUpRecord
	for _, rec := range r.records {
		if rec.Status == models.ConsolidationTopUpStatusPending {
			out = append(out, *rec)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *memoryTopUpRepo) GetSentRecords(ctx context.Context, limit int) ([]models.ConsolidationTopUpRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.ConsolidationTopUpRecord
	for _, rec := range r.records {
		if rec.Status == models.ConsolidationTopUpStatusSent {
			out = append(out, *rec)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *memoryTopUpRepo) MarkSent(ctx context.Context, id int64, txHash string, signedTx string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.records[id]
	if rec == nil {
		return nil
	}
	rec.Status = models.ConsolidationTopUpStatusSent
	tx := txHash
	rec.TxHash = &tx
	st := signedTx
	rec.SignedTx = &st
	return nil
}

func (r *memoryTopUpRepo) MarkConfirmed(ctx context.Context, id int64, confirmedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.records[id]
	if rec == nil {
		return nil
	}
	rec.Status = models.ConsolidationTopUpStatusConfirmed
	rec.ConfirmedAt = &confirmedAt
	return nil
}

func (r *memoryTopUpRepo) MarkFailed(ctx context.Context, id int64, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.records[id]
	if rec == nil {
		return nil
	}
	rec.Status = models.ConsolidationTopUpStatusFailed
	msg := errorMsg
	rec.ErrorMessage = &msg
	return nil
}

func (r *memoryTopUpRepo) IncrementRetry(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.records[id]
	if rec == nil {
		return nil
	}
	rec.RetryCount++
	return nil
}

type stubSigner struct {
	signerservice.SignerService

	getCompanyWalletFn func(ctx context.Context, in *pb.GetCompanyWalletRequest, opts ...grpc.CallOption) (*pb.GetCompanyWalletResponse, error)
	signTxFn           func(ctx context.Context, in *pb.SignTransactionRequest, opts ...grpc.CallOption) (*pb.SignTransactionResponse, error)
}

func (s *stubSigner) GetCompanyWallet(ctx context.Context, in *pb.GetCompanyWalletRequest, opts ...grpc.CallOption) (*pb.GetCompanyWalletResponse, error) {
	if s.getCompanyWalletFn == nil {
		panic("GetCompanyWallet not implemented")
	}
	return s.getCompanyWalletFn(ctx, in, opts...)
}

func (s *stubSigner) SignTransaction(ctx context.Context, in *pb.SignTransactionRequest, opts ...grpc.CallOption) (*pb.SignTransactionResponse, error) {
	if s.signTxFn == nil {
		panic("SignTransaction not implemented")
	}
	return s.signTxFn(ctx, in, opts...)
}

type stubChain struct {
	chainnode.Client

	buildTxFn     func(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error)
	broadcastTxFn func(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error)
	getReceiptFn  func(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error)
	getBlockFn    func(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error)
	estimateGasFn func(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error)
	getBalanceFn  func(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error)
}

func (s *stubChain) BuildTransaction(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error) {
	if s.buildTxFn == nil {
		panic("BuildTransaction not implemented")
	}
	return s.buildTxFn(ctx, in)
}

func (s *stubChain) BroadcastTransaction(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	if s.broadcastTxFn == nil {
		panic("BroadcastTransaction not implemented")
	}
	return s.broadcastTxFn(ctx, in)
}

func (s *stubChain) GetTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	if s.getReceiptFn == nil {
		panic("GetTransactionReceipt not implemented")
	}
	return s.getReceiptFn(ctx, in)
}

func (s *stubChain) GetBlockHeight(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	if s.getBlockFn == nil {
		panic("GetBlockHeight not implemented")
	}
	return s.getBlockFn(ctx, in)
}

func (s *stubChain) EstimateGas(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	if s.estimateGasFn == nil {
		panic("EstimateGas not implemented")
	}
	return s.estimateGasFn(ctx, in)
}

func (s *stubChain) GetBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	if s.getBalanceFn == nil {
		panic("GetBalance not implemented")
	}
	return s.getBalanceFn(ctx, in)
}

func TestTopUpWorker_SendTopUp_Evm_SuccessAndIdempotent(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	to := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	unsigned := types.NewTransaction(0, to, big.NewInt(1), 21_000, big.NewInt(1), nil)
	signed, err := types.SignTx(unsigned, types.LatestSignerForChainID(big.NewInt(1)), key)
	require.NoError(t, err)
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)
	signedHex := hexutil.Encode(raw)
	expectedHash := signed.Hash().Hex()

	topRepo := newMemoryTopUpRepo()
	chain := &stubChain{
		buildTxFn: func(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error) {
			return &pb.BuildTransactionResp{Success: true, RawTransaction: "0xraw"}, nil
		},
		broadcastTxFn: func(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
			return &pb.BroadcastTransactionResp{Success: true, TxHash: expectedHash}, nil
		},
	}
	signer := &stubSigner{
		getCompanyWalletFn: func(ctx context.Context, in *pb.GetCompanyWalletRequest, opts ...grpc.CallOption) (*pb.GetCompanyWalletResponse, error) {
			return &pb.GetCompanyWalletResponse{
				Code:    0,
				Message: "ok",
				Wallet:  &pb.CompanyWalletInfo{Address: "0xHot"},
			}, nil
		},
		signTxFn: func(ctx context.Context, in *pb.SignTransactionRequest, opts ...grpc.CallOption) (*pb.SignTransactionResponse, error) {
			return &pb.SignTransactionResponse{Code: 0, Signature: signedHex}, nil
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled: true,
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{
						"ETH": "1000000000000000000",
					},
				},
				TopUp: config.TopUpConfig{
					Enabled:                 true,
					HotWalletAddressType:    "hot_primary",
					HotWalletTemperature:    1,
					MaxTopUpAttemptsPerTask: 3,
				},
			},
		},
		Chain:           chain,
		SignerRpc:       signer,
		TopUpRecordRepo: topRepo,
		InstanceID:      "test",
	}

	worker := NewTopUpWorker(svcCtx)
	contract := "0xToken"
	task := &models.ConsolidationTask{
		TaskID:        "task-1",
		Chain:         "ETH",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
		FromAddress:   "0xDeposit",
		ToAddress:     "0xTarget",
		Amount:        "1",
		Status:        models.ConsolidationTaskStatusNeedGas,
	}

	txHash, err := worker.sendTopUp(context.Background(), task, "gas", "ETH", "123", 0)
	require.NoError(t, err)
	require.Equal(t, expectedHash, txHash)

	history, err := topRepo.ListByTaskID(context.Background(), "task-1")
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, models.ConsolidationTopUpStatusSent, history[0].Status)
	require.NotNil(t, history[0].TxHash)
	require.Equal(t, expectedHash, *history[0].TxHash)

	// Second attempt should be idempotent while the first record is still active (Sent).
	txHash2, err := worker.sendTopUp(context.Background(), task, "gas", "ETH", "123", 0)
	require.NoError(t, err)
	require.Equal(t, "", txHash2)
}

type stubTaskRepo struct {
	repository.ConsolidationTaskRepository

	mu sync.Mutex

	task *models.ConsolidationTask

	markPendingCalled bool
}

func (r *stubTaskRepo) ClaimByTaskID(ctx context.Context, taskID string, now time.Time, lease time.Duration, instanceID string) (*models.ConsolidationTask, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.task.TaskID != taskID {
		return nil, false, nil
	}
	return r.task, true, nil
}

func (r *stubTaskRepo) ReleaseClaim(ctx context.Context, id int64, instanceID string) error {
	return nil
}

func (r *stubTaskRepo) MarkPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, nextAttemptAt *time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markPendingCalled = true
	if r.task != nil && r.task.ID == id {
		r.task.Status = models.ConsolidationTaskStatusPending
	}
	return true, nil
}

func (r *stubTaskRepo) MarkNeedGas(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	return true, nil
}

func (r *stubTaskRepo) MarkNeedEnergy(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	return true, nil
}

func TestTopUpVerifyWorker_ConfirmedTx_MovesTaskToPending(t *testing.T) {
	topRepo := newMemoryTopUpRepo()
	txHash := "0x1111111111111111111111111111111111111111111111111111111111111111"
	signed := "0x00"
	rec := &models.ConsolidationTopUpRecord{
		TaskID:      "task-2",
		Chain:       "ETH",
		AssetSymbol: "ETH",
		FromAddress: "0xHot",
		ToAddress:   "0xDeposit",
		Amount:      "123",
		Purpose:     "gas",
		Status:      models.ConsolidationTopUpStatusSent,
		TxHash:      &txHash,
		SignedTx:    &signed,
	}
	_, err := topRepo.CreateIgnoreDuplicate(context.Background(), rec)
	require.NoError(t, err)
	require.NoError(t, topRepo.MarkSent(context.Background(), rec.ID, txHash, signed))

	chain := &stubChain{
		getReceiptFn: func(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
			return &pb.GetTransactionReceiptResp{
				Success: true,
				Receipt: &pb.TransactionReceipt{
					TxHash:           txHash,
					BlockNumber:      "1",
					Status:           pb.TxStatus_TX_STATUS_CONFIRMED,
					FromAddress:      "0xHot",
					ToAddress:        "0xDeposit",
					GasFee:           "1",
					GasUsed:          1,
					BlockHash:        "0x0",
					TransactionIndex: 0,
				},
			}, nil
		},
		getBlockFn: func(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
			return &pb.GetBlockHeightResp{Success: true, BlockHeight: 10}, nil
		},
		estimateGasFn: func(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
			return &pb.EstimateGasResp{Success: true, GasLimit: 21_000, GasPrice: "1", EstimatedFee: "21000"}, nil
		},
		getBalanceFn: func(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
			return &pb.GetBalanceResp{Success: true, Balance: "1000000000000000000"}, nil
		},
	}

	taskRepo := &stubTaskRepo{
		task: &models.ConsolidationTask{
			BaseModel:     model.BaseModel{ID: 1},
			TaskID:        "task-2",
			Chain:         "ETH",
			AssetSymbol:   "USDT",
			FromAddress:   "0xDeposit",
			ToAddress:     "0xTarget",
			Amount:        "1",
			Status:        models.ConsolidationTaskStatusNeedGas,
			Version:       0,
			RetryCount:    0,
			TokenContract: func() *string { v := "0xToken"; return &v }(),
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: func() config.Config {
			c := config.Config{
				Consolidation: config.ConsolidationConfig{
					Enabled:             true,
					GasSafetyMultipliers: map[string]float64{"ETH": 1.0},
					NativeReserves:      map[string]map[string]string{},
					MaxGasPrice:         map[string]string{},
				},
			}
			c.Consolidation.TopUp.Enabled = true
			c.Consolidation.TopUp.VerificationConfirmations.ETH = 1
			c.Consolidation.TopUp.TaskRetryDelaySeconds = 1
			return c
		}(),
		Chain:                 chain,
		TopUpRecordRepo:       topRepo,
		ConsolidationTaskRepo: taskRepo,
		InstanceID:            "test",
	}

	worker := NewTopUpVerifyWorker(svcCtx)
	worker.processRecord(context.Background(), rec, 10*time.Second)

	history, err := topRepo.ListByTaskID(context.Background(), "task-2")
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, models.ConsolidationTopUpStatusConfirmed, history[0].Status)
	require.True(t, taskRepo.markPendingCalled)
	require.Equal(t, models.ConsolidationTaskStatusPending, taskRepo.task.Status)
}
