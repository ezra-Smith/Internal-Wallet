package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type CreateDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDepositLogic {
	return &CreateDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateDepositLogic) CreateDeposit(in *pb.CreateDepositRequest) (*pb.CreateDepositResponse, error) {
	if in == nil || in.UserId <= 0 || strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.ChainCode) == "" || strings.TrimSpace(in.DepositAddress) == "" || strings.TrimSpace(in.Amount) == "" || strings.TrimSpace(in.TransactionHash) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"user_id":          "required",
			"asset_code":       "required",
			"chain_code":       "required",
			"deposit_address":  "required",
			"amount":           "required",
			"transaction_hash": "required",
		})
	}
	if in.Confirmations < 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CONFIRMATIONS", "invalid confirmations", map[string]string{"confirmations": "invalid"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.AccountingRpc == nil || l.svcCtx.ChainRepo == nil || l.svcCtx.CurrencyChainSettingsRepo == nil || l.svcCtx.WalletDepositAddressRepo == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Validate user exists.
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	assetCode := normalizeCode(in.AssetCode)
	accAssetResp, callErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if callErr != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accAssetResp == nil || !accAssetResp.Success || accAssetResp.Item == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "ASSET_NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
	}
	asset := accAssetResp.Item
	chainCode := normalizeCode(in.ChainCode)
	chains, err := l.svcCtx.ChainRepo.ListEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("list enabled chains failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	chainEnabled := false
	for _, c := range chains {
		if c == nil {
			continue
		}
		if normalizeCode(c.Name) == chainCode {
			chainEnabled = true
			break
		}
	}
	if !chainEnabled {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "CHAIN_NOT_FOUND", "chain not found", map[string]string{"chain_code": "not found"})
	}
	chainSettings, err := l.svcCtx.CurrencyChainSettingsRepo.ListByAssetCode(l.ctx, assetCode)
	if err != nil {
		l.Logger.Errorf("list currency chain settings failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	mappingOK := false
	for _, cs := range chainSettings {
		if cs == nil || cs.IsDeleted() {
			continue
		}
		if normalizeCode(cs.ChainCode) == chainCode {
			mappingOK = cs.Status == 1 && cs.DepositEnabled
			break
		}
	}
	if !mappingOK {
		return nil, errx.New(codes.FailedPrecondition, 422, errx.CodeInvalidParam, "CHAIN_NOT_CONFIGURED", "asset deposit not enabled on this chain", map[string]string{
			"asset_code": assetCode,
			"chain_code": chainCode,
		})
	}

	depositAddress := strings.TrimSpace(in.DepositAddress)
	addrRec, err := l.svcCtx.WalletDepositAddressRepo.FindByAddress(l.ctx, depositAddress)
	if err != nil || addrRec == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "ADDRESS_NOT_FOUND", "deposit address not found", nil)
	}
	if addrRec.UserID != in.UserId || normalizeCode(addrRec.ChainCode) != chainCode {
		return nil, errx.New(codes.FailedPrecondition, 422, errx.CodeInvalidParam, "ADDRESS_MISMATCH", "deposit address does not match user/asset/chain", map[string]string{
			"deposit_address": "mismatch",
		})
	}

	amountStr, err := parseAmountToDecimalString(in.Amount, asset.Precision, false)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT", "invalid amount", map[string]string{"amount": "invalid"})
	}

	txHash := strings.TrimSpace(in.TransactionHash)
	if _, err := l.svcCtx.WalletDepositRepo.FindByTxHash(l.ctx, txHash); err == nil {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "TX_HASH_EXISTS", "transaction_hash already exists", map[string]string{"transaction_hash": "exists"})
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "pending"
	}
	if !validateDepositStatus(status) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
	}

	// If a deposit is created directly as "completed", credit balances via Accounting before persisting the record.
	if strings.TrimSpace(status) == "completed" {
		if l.svcCtx.AccountingRpc == nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
		}
		idemKey, bizRef := depositConfirmIdemKeyAndBizRef(txHash)
		confirmResp, confirmErr := l.svcCtx.AccountingRpc.ConfirmDeposit(l.ctx, &pb.ConfirmDepositRequest{
			IdempotencyKey: idemKey,
			BizRef:         bizRef,
			UserId:         in.UserId,
			AssetCode:      assetCode,
			ChainCode:      chainCode,
			AmountDecimal:  amountStr,
			TxHash:         txHash,
			ToAddress:      depositAddress,
			Memo:           strings.TrimSpace(in.Memo),
		})
		if err := errFromAccountingTx("CONFIRM_DEPOSIT", confirmResp, confirmErr); err != nil {
			return nil, err
		}
	}

	var blockNumber *int64
	if in.BlockNumber > 0 {
		bn := in.BlockNumber
		blockNumber = &bn
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var created *model.WalletDepositModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		depRepo := l.svcCtx.WalletDepositRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m := &model.WalletDepositModel{
			UserID:          in.UserId,
			AssetCode:       assetCode,
			ChainCode:       chainCode,
			DepositAddress:  depositAddress,
			Amount:          amountStr,
			TransactionHash: txHash,
			BlockNumber:     blockNumber,
			Confirmations:   in.Confirmations,
			Status:          status,
			Memo:            strPtrOrNilTrim(in.Memo),
			CreatedAt:       &now,
			UpdatedAt:       &now,
		}
		if err := depRepo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"user_id":          in.UserId,
			"asset_code":       assetCode,
			"chain_code":       chainCode,
			"deposit_address":  maskAddress(depositAddress),
			"transaction_hash": maskAddress(txHash),
			"amount":           amountStr,
			"status":           status,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit.create",
			TargetType:  "wallet_deposit",
			TargetID:    maskAddress(txHash),
			Description: "创建充值记录: " + maskAddress(txHash),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "TX_HASH_EXISTS", "transaction_hash already exists", map[string]string{"transaction_hash": "exists"})
		}
		l.Logger.Errorf("create deposit failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create deposit failed", nil)
	}

	return &pb.CreateDepositResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateDepositData{
			Deposit: toPBDepositItem(created),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
