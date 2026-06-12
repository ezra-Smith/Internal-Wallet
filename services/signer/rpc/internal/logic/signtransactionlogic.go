package logic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"internalwallet/services/signer/rpc/internal/repository"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSignTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignTransactionLogic {
	return &SignTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SignTransactionLogic) SignTransaction(in *pb.SignTransactionRequest) (*pb.SignTransactionResponse, error) {
	// 1. 参数验证
	if err := l.validateParams(in); err != nil {
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: err.Error(),
		}, nil
	}

	rawTxHash := calcRawTxHash(in.RawTransaction)

	// 2. 幂等性检查
	existingLog, err := l.svcCtx.SignatureLogRepo.FindByRequestID(l.ctx, in.RequestId)
	if err == nil && existingLog != nil {
		// 请求已处理
		if existingLog.Status == constants.SignStatusSuccess {
			if existingLog.RawTxHash != nil && strings.TrimSpace(*existingLog.RawTxHash) != "" && *existingLog.RawTxHash != rawTxHash {
				return &pb.SignTransactionResponse{
					Code:    int32(errcode.SignerDuplicateRequest),
					Message: "duplicate request_id with different raw_transaction",
				}, nil
			}
			if existingLog.SignedTx == nil || strings.TrimSpace(*existingLog.SignedTx) == "" || existingLog.TxHash == nil || strings.TrimSpace(*existingLog.TxHash) == "" {
				return &pb.SignTransactionResponse{
					Code:    int32(errcode.SignerInternalError),
					Message: "cached signature log missing signed_tx/tx_hash; use a new request_id",
				}, nil
			}
			return &pb.SignTransactionResponse{
				Code:      0,
				Message:   "success (cached)",
				RequestId: existingLog.RequestID,
				Chain:     existingLog.Chain,
				Signature: *existingLog.SignedTx,
				TxHash:    *existingLog.TxHash,
				SignedAt:  existingLog.UpdatedAt.Format(time.RFC3339),
			}, nil
		}
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerDuplicateRequest),
			Message: "duplicate request",
		}, nil
	}

	// 3. 根据 from_address 查询 seed_id 和 derivation_path
	var seedID string
	var derivationPath string

	if in.FromAddress == "" {
		// 未指定 from_address，使用该链的默认热钱包
		l.Logger.Infof("No from_address specified, using default hot wallet for chain %s", in.Chain)
		companyWallet, err := l.svcCtx.CompanyWalletRepo.GetDefaultHotWallet(l.ctx, in.Chain)
		if err != nil {
			l.Logger.Errorf("Failed to get default hot wallet: %v", err)
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerInternalError),
				Message: fmt.Sprintf("no default hot wallet found for chain %s", in.Chain),
			}, nil
		}
		seedID = companyWallet.SeedID
		derivationPath = companyWallet.DerivationPath
		l.Logger.Infof("✓ Using default hot wallet: address=%s, seed=%s, path=%s",
			companyWallet.Address, seedID, derivationPath)
	} else {
		// 指定了 from_address，先从公司钱包查找，再从用户地址生成日志查找
		l.Logger.Infof("Looking up seed for from_address=%s on chain=%s", in.FromAddress, in.Chain)

		// 尝试从公司钱包表查找
		companyWallet, err := l.svcCtx.CompanyWalletRepo.FindByChainAndAddress(l.ctx, in.Chain, in.FromAddress)
		if err == nil && companyWallet != nil {
			seedID = companyWallet.SeedID
			derivationPath = companyWallet.DerivationPath
			l.Logger.Infof("✓ Found in company_wallets: seed=%s, path=%s", seedID, derivationPath)
		} else {
			// 从用户地址生成日志表查找
			addrLog, err := l.svcCtx.AddressGenerationLogRepo.FindByAddressAndChain(l.ctx, in.FromAddress, in.Chain)
			if err != nil {
				l.Logger.Errorf("Address not found in company_wallets or address_generation_logs: %v", err)
				return &pb.SignTransactionResponse{
					Code:    int32(errcode.SignerInvalidParams),
					Message: fmt.Sprintf("unknown address: %s", in.FromAddress),
				}, nil
			}

			// 从 MasterSeedID 获取 SeedID
			if addrLog.MasterSeedID == nil {
				l.Logger.Errorf("Address %s has no master_seed_id", in.FromAddress)
				return &pb.SignTransactionResponse{
					Code:    int32(errcode.SignerInternalError),
					Message: "address has no associated seed",
				}, nil
			}

			masterSeed, err := l.svcCtx.MasterSeedRepo.FindByID(l.ctx, *addrLog.MasterSeedID)
			if err != nil {
				l.Logger.Errorf("Failed to find master seed by ID %d: %v", *addrLog.MasterSeedID, err)
				return &pb.SignTransactionResponse{
					Code:    int32(errcode.SignerSeedNotFound),
					Message: "seed not found for address",
				}, nil
			}

			seedID = masterSeed.SeedID
			derivationPath = addrLog.DerivationPath
			l.Logger.Infof("✓ Found in address_generation_logs: seed=%s, path=%s", seedID, derivationPath)
		}
	}

	// 4. 查询Seed详情
	seed, err := l.svcCtx.MasterSeedRepo.FindBySeedID(l.ctx, seedID)
	if err != nil {
		if errors.Is(err, repository.ErrMasterSeedNotFound) {
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerSeedNotFound),
				Message: "seed not found",
			}, nil
		}
		l.Logger.Errorf("Failed to query seed: %v", err)
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "database error",
		}, nil
	}

	// 5. 创建日志记录
	signLog := &models.SignatureLog{
		RequestID:      in.RequestId,
		MasterSeedID:   seed.ID, // 使用查询到的seed ID
		Chain:          in.Chain,
		DerivationPath: derivationPath, // 使用查询到的派生路径
		OperationType:  int(in.OperationType),
		RawTxHash:      &rawTxHash,
		Status:         constants.SignStatusPending,
	}
	if in.Amount != "" {
		signLog.Amount = &in.Amount
	}
	if in.ToAddress != "" {
		signLog.ToAddress = &in.ToAddress
	}
	if in.FromAddress != "" {
		signLog.FromAddress = &in.FromAddress
	}
	if in.AssetSymbol != "" {
		signLog.AssetSymbol = &in.AssetSymbol
	}
	if in.TokenContract != "" {
		signLog.TokenContract = &in.TokenContract
	}
	if in.Requester != "" {
		signLog.Requester = &in.Requester
	}

	if err := l.svcCtx.SignatureLogRepo.Create(l.ctx, signLog); err != nil {
		l.Logger.Errorf("Failed to create signature log: %v", err)
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "failed to create log",
		}, nil
	}

	// 5. 检查Seed状态
	if seed.Status != constants.SeedStatusActive {
		l.updateLogStatus(signLog.ID, constants.SignStatusRejected, "seed is not active")
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerSeedInactive),
			Message: "seed is not active",
		}, nil
	}

	// 6. signer 必须先解锁（seed 仅驻留内存）
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seed.SeedID)
	if !ok {
		l.updateLogStatus(signLog.ID, constants.SignStatusRejected, "wallet is locked")
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 7. 验证Seed哈希（防止运行态被污染）
	if !utils.VerifySeedHash(seedBytes, seed.SeedHash) {
		l.updateLogStatus(signLog.ID, constants.SignStatusFailed, "seed integrity check failed")
		l.Logger.Errorf("Seed hash verification failed (seed_id=%s)", seed.SeedID)
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerSeedHashMismatch),
			Message: "seed integrity check failed",
		}, nil
	}

	// 8. 创建HD钱包
	wallet, err := NewHDWalletFromSeed(seedBytes, in.Chain)
	if err != nil {
		l.updateLogStatus(signLog.ID, constants.SignStatusFailed, "failed to create wallet")
		l.Logger.Errorf("Failed to create HD wallet: %v", err)
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to create wallet",
		}, nil
	}

	// 9. 【安全检查】验证 from 地址是否与派生路径匹配
	if in.FromAddress != "" {
		// 从派生路径获取对应的地址
		uncompressed, _, err := wallet.GetPublicKey(derivationPath)
		if err != nil {
			l.updateLogStatus(signLog.ID, constants.SignStatusFailed, "failed to derive address")
			l.Logger.Errorf("Failed to derive address from path: %v", err)
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerInternalError),
				Message: "failed to derive address",
			}, nil
		}

		derivedAddress, err := PublicKeyToAddress(in.Chain, uncompressed)
		if err != nil {
			l.updateLogStatus(signLog.ID, constants.SignStatusFailed, "failed to generate address")
			l.Logger.Errorf("Failed to generate address from pubkey: %v", err)
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerInternalError),
				Message: "failed to generate address",
			}, nil
		}

		// 验证地址是否匹配（忽略大小写）
		if !strings.EqualFold(derivedAddress, in.FromAddress) {
			errMsg := fmt.Sprintf("address mismatch: derivation path %s corresponds to %s, but from_address is %s",
				derivationPath, derivedAddress, in.FromAddress)
			l.updateLogStatus(signLog.ID, constants.SignStatusFailed, errMsg)
			l.Logger.Errorf("Security check failed: %s", errMsg)
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerInvalidParams),
				Message: errMsg,
			}, nil
		}

		l.Logger.Infof("✓ Address verification passed: %s matches %s", in.FromAddress, derivationPath)
	}

	// 10. 解码交易数据
	txData, err := hexutil.Decode(in.RawTransaction)
	if err != nil {
		l.updateLogStatus(signLog.ID, constants.SignStatusFailed, "invalid tx data format")
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerInvalidTxData),
			Message: "invalid tx data hex format",
		}, nil
	}

	// 10.1 【安全检查】归集交易必须校验 raw_transaction 内容与请求字段一致（不信任上游构建服务/节点）
	if in.OperationType == 3 {
		if err := validateConsolidationRawTx(in, txData); err != nil {
			l.updateLogStatus(signLog.ID, constants.SignStatusRejected, err.Error())
			return &pb.SignTransactionResponse{
				Code:    int32(errcode.SignerInvalidTxData),
				Message: err.Error(),
			}, nil
		}
	}

	// 11. 根据链类型签名交易
	signature, txHashStr, err := wallet.SignTransactionForChain(in.Chain, derivationPath, txData)
	if err != nil {
		l.updateLogStatus(signLog.ID, constants.SignStatusFailed, err.Error())
		l.Logger.Errorf("Failed to sign %s transaction: %v", in.Chain, err)
		return &pb.SignTransactionResponse{
			Code:    int32(errcode.SignerSignatureFailed),
			Message: fmt.Sprintf("failed to sign %s transaction: %v", in.Chain, err),
		}, nil
	}

	// 12. 更新日志为成功
	completedAt := time.Now()
	if err := l.svcCtx.SignatureLogRepo.MarkSuccess(l.ctx, signLog.ID, txHashStr, signature, rawTxHash, completedAt); err != nil {
		l.Logger.Errorf("Failed to update signature log: %v", err)
	}

	// 13. 更新Seed统计
	if err := l.svcCtx.MasterSeedRepo.IncrementSignatureCount(l.ctx, seed.SeedID); err != nil {
		l.Logger.Errorf("Failed to increment seed signature count: %v", err)
	}

	l.Logger.Infof("✓ %s transaction signed successfully: request_id=%s, tx_hash=%s", in.Chain, in.RequestId, txHashStr)

	return &pb.SignTransactionResponse{
		Code:      0,
		Message:   "success",
		RequestId: in.RequestId,
		Chain:     in.Chain,
		Signature: signature,
		TxHash:    txHashStr,
		SignedAt:  completedAt.Format(time.RFC3339),
	}, nil
}

func (l *SignTransactionLogic) validateParams(in *pb.SignTransactionRequest) error {
	if in.RequestId == "" {
		return errors.New("request_id is required")
	}
	if in.Chain == "" {
		return errors.New("chain is required")
	}
	if in.RawTransaction == "" {
		return errors.New("raw_transaction is required")
	}
	if !constants.IsChainSupported(in.Chain) {
		return errors.New("unsupported chain")
	}
	// 注意：from_address 是可选的，不指定则使用默认热钱包
	// seed_id 和 derivation_path 已移除，由系统内部根据 from_address 查询
	return nil
}

func (l *SignTransactionLogic) updateLogStatus(logID int64, status int, errorMsg string) {
	if err := l.svcCtx.SignatureLogRepo.UpdateStatus(l.ctx, logID, status, errorMsg); err != nil {
		l.Logger.Errorf("Failed to update signature log status (id=%d): %v", logID, err)
	}
}

func calcRawTxHash(rawTransaction string) string {
	canonical := strings.ToLower(strings.TrimSpace(rawTransaction))
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum[:])
}
