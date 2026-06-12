package logic

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"
)

type ImportTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewImportTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportTransferBatchLogic {
	return &ImportTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type transferImportRow struct {
	RowNum   int32
	UID      string
	UserID   int64
	Currency string
	Address  string
	Amount   string
	Note     string
}

type transferImportError struct {
	RowNum int32
	Msg    string
}

const (
	transferImportMaxRows     = 1000
	transferImportMaxFileSize = 5 << 20 // 5MB
)

func (l *ImportTransferBatchLogic) ImportTransferBatch(in *pb.ImportTransferBatchRequest) (*pb.ImportTransferBatchResponse, error) {
	if in == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"file": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil || l.svcCtx.TransferBatchItemRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	filename := strings.TrimSpace(in.Filename)
	if filename == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FILE_FORMAT", "filename required", map[string]string{"filename": "required"})
	}
	if len(in.File) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "EMPTY_FILE", "file is empty", map[string]string{"file": "empty"})
	}
	if len(in.File) > transferImportMaxFileSize {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FILE_TOO_LARGE", "file too large", map[string]string{"file": "too_large"})
	}

	name := strings.TrimSpace(in.Name)
	rawNetwork := strings.TrimSpace(in.Network)
	if rawNetwork != "" && !strings.EqualFold(rawNetwork, "internal") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "transfer batch is internal-ledger only", map[string]string{
			"network": "internal_only",
		})
	}
	network := "INTERNAL"
	defaultCurrency := strings.ToUpper(strings.TrimSpace(in.Currency))
	desc := strings.TrimSpace(in.Description)
	if name == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"name": "required",
		})
	}

	transferType := normalizeTransferBatchType(in.TransferType)
	if transferType == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"transfer_type": "invalid"})
	}

	previewItems, validRows, totalRows, totalAmount, currencyStats, errs, err := l.parseAndValidateTransferImport(filename, in.File, network, defaultCurrency, transferType)
	if err != nil {
		return nil, err
	}

	validCount := int32(len(validRows))
	invalidCount := int32(0)
	if totalRows >= validCount {
		invalidCount = totalRows - validCount
	}

	respErrItems := make([]*pb.ImportTransferErrorItem, 0, len(errs))
	for _, e := range errs {
		respErrItems = append(respErrItems, &pb.ImportTransferErrorItem{
			Row:   e.RowNum,
			Error: e.Msg,
		})
	}
	respPreviewItems := make([]*pb.ImportTransferPreviewItem, 0, len(previewItems))
	for _, it := range previewItems {
		respPreviewItems = append(respPreviewItems, &pb.ImportTransferPreviewItem{
			Row:      it.RowNum,
			Uid:      it.UID,
			Currency: it.Currency,
			Address:  it.Address,
			Amount:   it.Amount,
			Note:     it.Note,
			Errors:   it.Errors,
		})
	}

	respCurrencyStats := make([]*pb.ImportTransferCurrencyStat, 0, len(currencyStats))
	for _, s := range currencyStats {
		respCurrencyStats = append(respCurrencyStats, &pb.ImportTransferCurrencyStat{
			Currency:    s.Currency,
			ValidRows:   s.ValidRows,
			TotalAmount: s.TotalAmount,
		})
	}

	batchCurrency := pickBatchCurrency(currencyStats, defaultCurrency)

	// Preview-only: return validation result, do not create DB records.
	if in.Preview {
		return &pb.ImportTransferBatchResponse{
			Success: true,
			Message: resp.Msg(l.ctx, "ok"),
			Data: &pb.ImportTransferBatchData{
				BatchId:      "",
				Name:         name,
				Network:      network,
				Currency:     batchCurrency,
				Status:       "preview",
				CreatedAt:    "",
				TransferType: transferType,
				ImportResult: &pb.ImportTransferResult{
					TotalRows:     totalRows,
					ValidRows:     validCount,
					InvalidRows:   invalidCount,
					TotalAmount:   totalAmount,
					Errors:        respErrItems,
					CurrencyStats: respCurrencyStats,
				},
				PreviewItems: respPreviewItems,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// Enforce: any invalid row blocks submission.
	if invalidCount > 0 || validCount == 0 {
		return &pb.ImportTransferBatchResponse{
			Success: false,
			Message: "存在错误数据，请修正后再提交",
			Data: &pb.ImportTransferBatchData{
				BatchId:      "",
				Name:         name,
				Network:      network,
				Currency:     batchCurrency,
				Status:       "invalid",
				CreatedAt:    "",
				TransferType: transferType,
				ImportResult: &pb.ImportTransferResult{
					TotalRows:     totalRows,
					ValidRows:     validCount,
					InvalidRows:   invalidCount,
					TotalAmount:   totalAmount,
					Errors:        respErrItems,
					CurrencyStats: respCurrencyStats,
				},
				PreviewItems: respPreviewItems,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	now := time.Now()
	batchID := "batch-" + utils.GenerateIDString()

	notePtr := strPtrOrNilTrim(desc)
	var created *model.TransferBatchModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		batchRepo := repository.NewTransferBatchRepository(tx)
		itemRepo := repository.NewTransferBatchItemRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m := &model.TransferBatchModel{
			BatchID:         batchID,
			Name:            name,
			TransferType:    transferType,
			Description:     notePtr,
			Network:         network,
			Currency:        batchCurrency,
			TotalRecipients: validCount,
			TotalAmount:     totalAmount,
			TotalAmountUSD:  "0",
			SuccessCount:    0,
			FailedCount:     0,
			Status:          "draft",
			CreatedBy:       current.Username,
			CreatedByID:     current.ID,
			CreatedAt:       &now,
		}
		if err := batchRepo.Create(l.ctx, m); err != nil {
			return err
		}

		items := make([]*model.TransferBatchItemModel, 0, len(validRows))
		for _, r := range validRows {
			uid := r.UserID
			ccy := strings.ToUpper(strings.TrimSpace(r.Currency))
			addr := strings.TrimSpace(r.Address)
			amt := strings.TrimSpace(r.Amount)
			note := strings.TrimSpace(r.Note)
			items = append(items, &model.TransferBatchItemModel{
				ID:        "transfer-" + utils.GenerateIDString(),
				BatchID:   batchID,
				UserID:    &uid,
				Currency:  ccy,
				ToAddress: addr,
				Amount:    amt,
				Note:      strPtrOrNilTrim(note),
				Status:    "pending",
				CreatedAt: &now,
			})
		}
		if err := itemRepo.CreateMany(l.ctx, items); err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id": batchID,
			"name":     name,
			"type":     transferType,
			"network":  network,
			"currency": batchCurrency,
			"currencies": func() []string {
				out := make([]string, 0, len(currencyStats))
				for _, s := range currencyStats {
					out = append(out, s.Currency)
				}
				return out
			}(),
			"total":   totalRows,
			"valid":   validCount,
			"invalid": invalidCount,
		})
		ip := middleware.GetClientIP(l.ctx)
		ua := middleware.GetUserAgent(l.ctx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.import",
			TargetType:  "transfer_batch",
			TargetID:    batchID,
			Description: "导入转账批次",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})

		created = m
		return nil
	}); err != nil {
		l.Logger.Errorf("import transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "import failed", nil)
	}

	return &pb.ImportTransferBatchResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "BATCH_IMPORTED"),
		Data: &pb.ImportTransferBatchData{
			BatchId:      batchID,
			Name:         created.Name,
			Network:      created.Network,
			Currency:     created.Currency,
			Status:       created.Status,
			CreatedAt:    formatTimePtr(created.CreatedAt),
			TransferType: created.TransferType,
			ImportResult: &pb.ImportTransferResult{
				TotalRows:     totalRows,
				ValidRows:     validCount,
				InvalidRows:   invalidCount,
				TotalAmount:   totalAmount,
				Errors:        respErrItems,
				CurrencyStats: respCurrencyStats,
			},
			PreviewItems: respPreviewItems,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func normalizeTransferBatchType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "normal":
		return "normal"
	case "airdrop":
		return "airdrop"
	default:
		return ""
	}
}

type transferImportCurrencyStat struct {
	Currency    string
	ValidRows   int32
	TotalAmount string
}

func pickBatchCurrency(stats []transferImportCurrencyStat, defaultCurrency string) string {
	if len(stats) == 1 && strings.TrimSpace(stats[0].Currency) != "" {
		return strings.ToUpper(strings.TrimSpace(stats[0].Currency))
	}
	if len(stats) > 1 {
		return "MIXED"
	}
	if strings.TrimSpace(defaultCurrency) != "" {
		return strings.ToUpper(strings.TrimSpace(defaultCurrency))
	}
	return ""
}

func (l *ImportTransferBatchLogic) parseAndValidateTransferImport(filename string, content []byte, network string, defaultCurrency string, transferType string) ([]transferImportPreviewItem, []transferImportRow, int32, string, []transferImportCurrencyStat, []transferImportError, error) {
	records, totalRows, err := parseTransferImportFile(filename, content)
	if err != nil {
		return nil, nil, 0, "", nil, nil, err
	}
	previewItems, validRows, totalAmount, currencyStats, errs, err := l.validateTransferImportRecords(records, network, defaultCurrency, transferType)
	if err != nil {
		return nil, nil, 0, "", nil, nil, err
	}
	return previewItems, validRows, totalRows, totalAmount, currencyStats, errs, nil
}

type transferImportPreviewItem struct {
	RowNum   int32
	UID      string
	Currency string
	Address  string
	Amount   string
	Note     string
	Errors   []string
}

type transferImportRawRow struct {
	RowNum   int32
	UID      string
	Currency string
	Amount   string
	Note     string
}

func parseTransferImportFile(filename string, content []byte) ([]transferImportRawRow, int32, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))

	// Some browsers (and fetch/blob uploads) may send filename="blob" (or other temp names)
	// without a meaningful extension. Try to sniff file type from the content.
	if ext == "" || (ext != ".csv" && ext != ".xlsx") {
		// XLSX is a ZIP container; it typically starts with "PK\x03\x04".
		if len(content) >= 4 && bytes.HasPrefix(content, []byte("PK\x03\x04")) {
			ext = ".xlsx"
		} else {
			// Heuristic for CSV: avoid NUL bytes (binary), and require at least a separator/newline.
			if len(content) > 0 && !bytes.Contains(content, []byte{0x00}) {
				s := strings.TrimSpace(string(content))
				if s != "" && (strings.Contains(s, ",") || strings.Contains(s, "\t")) && (strings.Contains(s, "\n") || strings.Contains(s, "\r")) {
					ext = ".csv"
				}
			}
		}
	}

	switch ext {
	case ".csv":
		rows, total, err := parseTransferCSV(content)
		if err != nil {
			return nil, 0, err
		}
		return rows, total, nil
	case ".xlsx":
		rows, total, err := parseTransferXLSX(content)
		if err != nil {
			return nil, 0, err
		}
		return rows, total, nil
	default:
		return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FILE_FORMAT", "file format not supported", map[string]string{"file": "invalid_format"})
	}
}

func parseTransferCSV(content []byte) ([]transferImportRawRow, int32, error) {
	r := csv.NewReader(bytes.NewReader(content))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FILE_FORMAT", "invalid csv", map[string]string{"file": "invalid"})
	}
	return parseTransferRecords(records)
}

func parseTransferXLSX(content []byte) ([]transferImportRawRow, int32, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FILE_FORMAT", "invalid xlsx", map[string]string{"file": "invalid"})
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "EMPTY_FILE", "empty xlsx", map[string]string{"file": "empty"})
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FILE_FORMAT", "invalid xlsx", map[string]string{"file": "invalid"})
	}
	return parseTransferRecords(rows)
}

func parseTransferRecords(records [][]string) ([]transferImportRawRow, int32, error) {
	trimCell := func(s string) string {
		return strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))
	}

	isHeader := func(row []string) bool {
		if len(row) < 2 {
			return false
		}
		a := strings.ToLower(trimCell(row[0]))
		b := strings.ToLower(trimCell(row[1]))
		c := ""
		if len(row) > 2 {
			c = strings.ToLower(trimCell(row[2]))
		}
		return strings.Contains(a, "uid") || strings.Contains(a, "用户") ||
			strings.Contains(b, "token") || strings.Contains(b, "currency") || strings.Contains(b, "asset") || strings.Contains(b, "币种") ||
			strings.Contains(b, "金额") || strings.Contains(b, "amount") ||
			strings.Contains(c, "金额") || strings.Contains(c, "amount")
	}

	parseHeaderIndex := func(row []string) map[string]int {
		out := map[string]int{}
		for idx, cell := range row {
			v := strings.ToLower(trimCell(cell))
			switch {
			case out["uid"] == 0 && (strings.Contains(v, "uid") || strings.Contains(v, "用户")):
				out["uid"] = idx + 1
			case out["currency"] == 0 && (strings.Contains(v, "token") || strings.Contains(v, "currency") || strings.Contains(v, "asset") || strings.Contains(v, "币种")):
				out["currency"] = idx + 1
			case out["amount"] == 0 && (strings.Contains(v, "amount") || strings.Contains(v, "金额")):
				out["amount"] = idx + 1
			case out["note"] == 0 && (strings.Contains(v, "note") || strings.Contains(v, "备注") || strings.Contains(v, "remark")):
				out["note"] = idx + 1
			}
		}
		return out
	}

	startIdx := 0
	headerIdx := map[string]int{}
	if len(records) > 0 && isHeader(records[0]) {
		headerIdx = parseHeaderIndex(records[0])
		startIdx = 1
	}

	totalRows := int32(0)
	out := make([]transferImportRawRow, 0, len(records))

	for i := startIdx; i < len(records); i++ {
		rowNum := int32(i + 1)
		row := records[i]
		if len(row) == 0 {
			continue
		}

		getBy1Based := func(oneBased int) string {
			if oneBased <= 0 {
				return ""
			}
			idx := oneBased - 1
			if idx < 0 || idx >= len(row) {
				return ""
			}
			return trimCell(row[idx])
		}

		uid := getBy1Based(headerIdx["uid"])
		ccy := getBy1Based(headerIdx["currency"])
		amt := getBy1Based(headerIdx["amount"])
		note := getBy1Based(headerIdx["note"])

		// Fallback for non-header or incomplete headers.
		if uid == "" && len(row) > 0 && headerIdx["uid"] == 0 {
			uid = trimCell(row[0])
		}
		// Default layout:
		// - uid,currency,amount,note (>=4 cols)
		// - uid,amount,note (>=3 cols, currency missing)
		if headerIdx["currency"] == 0 && ccy == "" && len(row) >= 4 {
			ccy = trimCell(row[1])
		}
		if headerIdx["amount"] == 0 && amt == "" {
			if len(row) >= 4 {
				amt = trimCell(row[2])
			} else if len(row) > 1 {
				amt = trimCell(row[1])
			}
		}
		if headerIdx["note"] == 0 && note == "" {
			if len(row) >= 4 && len(row) > 3 {
				note = trimCell(row[3])
			} else if len(row) > 2 {
				note = trimCell(row[2])
			}
		}

		if uid == "" && ccy == "" && amt == "" && note == "" {
			continue
		}

		totalRows++
		if totalRows > transferImportMaxRows {
			return nil, 0, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "TOO_MANY_ROWS", "too many rows", map[string]string{"file": "too_many_rows"})
		}

		out = append(out, transferImportRawRow{
			RowNum:   rowNum,
			UID:      uid,
			Currency: ccy,
			Amount:   amt,
			Note:     note,
		})
	}

	return out, totalRows, nil
}

func isTransferTypeToken(s string) bool {
	v := strings.ToLower(strings.TrimSpace(s))
	return v == "normal" || v == "airdrop"
}

func (l *ImportTransferBatchLogic) validateTransferImportRecords(rows []transferImportRawRow, network string, defaultCurrency string, transferType string) ([]transferImportPreviewItem, []transferImportRow, string, []transferImportCurrencyStat, []transferImportError, error) {
	network = strings.TrimSpace(network)
	defaultCurrency = strings.ToUpper(strings.TrimSpace(defaultCurrency))
	transferType = strings.ToLower(strings.TrimSpace(transferType))
	if network == "" {
		network = "INTERNAL"
	}
	// Transfer batch is internal-ledger only; all recipients are UID-based.
	// Any on-chain addressing is intentionally not supported here.

	// Collect & parse UIDs (best-effort, for batch DB lookups).
	userIDSet := map[int64]struct{}{}
	for _, r := range rows {
		uid := strings.TrimSpace(r.UID)
		if uid == "" {
			continue
		}
		id, err := strconv.ParseInt(uid, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		userIDSet[id] = struct{}{}
	}

	userIDs := make([]int64, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	// Batch-check user existence (users table is shared; enforce deleted_at IS NULL).
	exists := map[int64]struct{}{}
	if len(userIDs) > 0 {
		var ids []int64
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Table("users").
			Select("id").
			Where("id IN ? AND deleted_at IS NULL", userIDs).
			Scan(&ids).Error; err != nil {
			l.Logger.Errorf("validate transfer import users failed: %v", err)
			return nil, nil, "", nil, nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		for _, id := range ids {
			exists[id] = struct{}{}
		}
	}

	// Collect currencies (for asset validation + address lookup).
	ccySet := map[string]struct{}{}
	for _, r := range rows {
		ccy := strings.ToUpper(strings.TrimSpace(r.Currency))
		if ccy == "" {
			ccy = defaultCurrency
		}
		if ccy == "" {
			continue
		}
		ccySet[ccy] = struct{}{}
	}
	ccys := make([]string, 0, len(ccySet))
	for c := range ccySet {
		ccys = append(ccys, c)
	}

	// Validate currencies via Accounting (source of truth).
	assetStatusByCode := map[string]int32{} // 0 = invalid/not found, 1=enabled, 2=disabled/other
	if len(ccys) > 0 {
		if l.svcCtx.AccountingRpc == nil {
			return nil, nil, "", nil, nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service not configured", nil)
		}
		for _, c := range ccys {
			code := strings.ToUpper(strings.TrimSpace(c))
			if code == "" {
				continue
			}
			accResp, callErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: code})
			if callErr != nil {
				l.Logger.Errorf("call accounting GetAsset failed: %v", callErr)
				return nil, nil, "", nil, nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
			}
			if accResp == nil || !accResp.Success || accResp.Item == nil || strings.TrimSpace(accResp.Item.Code) == "" {
				assetStatusByCode[code] = 0
				continue
			}
			status := accResp.Item.Status
			assetStatusByCode[code] = status
		}
	}

	// Validate rows + build preview.
	sumByCurrency := map[string]decimal.Decimal{}
	cntByCurrency := map[string]int32{}
	valid := make([]transferImportRow, 0, len(rows))
	preview := make([]transferImportPreviewItem, 0, len(rows))
	errItems := make([]transferImportError, 0)

	seenKey := map[string]int32{} // uid|currency -> first row

	for _, r := range rows {
		uidRaw := strings.TrimSpace(r.UID)
		ccyRaw := strings.ToUpper(strings.TrimSpace(r.Currency))
		if ccyRaw == "" {
			ccyRaw = defaultCurrency
		}
		amtRaw := strings.TrimSpace(r.Amount)
		note := strings.TrimSpace(r.Note)

		rowErrs := make([]string, 0)
		userID := int64(0)
		address := ""

		if ccyRaw == "" {
			rowErrs = append(rowErrs, "Token不能为空")
		} else if len(ccyRaw) > 16 {
			rowErrs = append(rowErrs, "Token格式无效")
		} else {
			st := assetStatusByCode[ccyRaw]
			if st == 0 {
				rowErrs = append(rowErrs, "Token不存在")
			} else if st != 1 {
				rowErrs = append(rowErrs, "Token已禁用")
			}
		}

		if uidRaw == "" {
			rowErrs = append(rowErrs, "UID不能为空")
		} else {
			uidCanonical := uidRaw
			id, err := strconv.ParseInt(uidRaw, 10, 64)
			if err != nil || id <= 0 {
				rowErrs = append(rowErrs, "UID格式无效")
			} else {
				userID = id
				uidCanonical = strconv.FormatInt(id, 10)
				if _, ok := exists[userID]; !ok {
					rowErrs = append(rowErrs, "用户不存在")
				} else {
					// Internal-ledger transfer: recipient is UID-based.
					// Store UID in `to_address` for display/search compatibility.
					address = uidRaw
				}
			}

			// Duplicate rule: one row per (uid, token).
			if userID > 0 && ccyRaw != "" {
				key := fmt.Sprintf("%s|%s", uidCanonical, ccyRaw)
				if first, ok := seenKey[key]; ok {
					rowErrs = append(rowErrs, fmt.Sprintf("重复的 UID+Token（与第 %d 行重复）", first))
				} else {
					seenKey[key] = r.RowNum
				}
			}
		}

		normalizedAmount := amtRaw
		amountDec := decimal.Zero
		amountOk := false
		if amtRaw == "" {
			rowErrs = append(rowErrs, "金额必须大于 0")
		} else {
			d, err := decimal.NewFromString(amtRaw)
			if err != nil || d.LessThanOrEqual(decimal.Zero) {
				rowErrs = append(rowErrs, "金额必须大于 0")
			} else if d.Exponent() < -18 {
				rowErrs = append(rowErrs, "金额格式无效")
			} else {
				normalizedAmount = d.StringFixed(6)
				amountDec = d
				amountOk = true
			}
		}

		// Note length is bounded by DB schema (varchar(256)).
		// Prevent mis-using "note" to store transfer type (normal/airdrop).
		// This usually happens when importing 3/4-column files and the last column is actually an order type column.
		if isTransferTypeToken(note) && (transferType == "" || isTransferTypeToken(transferType)) {
			rowErrs = append(rowErrs, "备注(note) 不能填写 normal/airdrop（这是订单类型，请在批次信息里选择转账类型）")
			note = ""
		}
		if len(note) > 256 {
			rowErrs = append(rowErrs, "备注过长（最多 256 字符）")
		}

		preview = append(preview, transferImportPreviewItem{
			RowNum:   r.RowNum,
			UID:      uidRaw,
			Currency: ccyRaw,
			Address:  address,
			Amount:   normalizedAmount,
			Note:     note,
			Errors:   rowErrs,
		})

		if len(rowErrs) > 0 {
			errMsg := strings.Join(rowErrs, "; ")
			errItems = append(errItems, transferImportError{RowNum: r.RowNum, Msg: errMsg})
			continue
		}

		// Sanity guard: address must be set for valid row.
		if address == "" || userID <= 0 || !amountOk {
			errItems = append(errItems, transferImportError{RowNum: r.RowNum, Msg: "收款信息无效"})
			continue
		}

		if ccyRaw != "" {
			sumByCurrency[ccyRaw] = sumByCurrency[ccyRaw].Add(amountDec)
			cntByCurrency[ccyRaw] = cntByCurrency[ccyRaw] + 1
		}
		valid = append(valid, transferImportRow{
			RowNum:   r.RowNum,
			UID:      uidRaw,
			UserID:   userID,
			Currency: ccyRaw,
			Address:  address,
			Amount:   normalizedAmount,
			Note:     note,
		})
	}

	stats := make([]transferImportCurrencyStat, 0, len(cntByCurrency))
	for ccy, cnt := range cntByCurrency {
		sum := sumByCurrency[ccy]
		total := sum.StringFixed(6)
		if sum.IsZero() {
			total = "0.000000"
		}
		stats = append(stats, transferImportCurrencyStat{
			Currency:    ccy,
			ValidRows:   cnt,
			TotalAmount: total,
		})
	}
	// Stable order for UI.
	if len(stats) > 1 {
		sort.Slice(stats, func(i, j int) bool { return stats[i].Currency < stats[j].Currency })
	}

	totalAmount := "0.000000"
	if len(stats) == 1 {
		totalAmount = stats[0].TotalAmount
	}

	return preview, valid, totalAmount, stats, errItems, nil
}
