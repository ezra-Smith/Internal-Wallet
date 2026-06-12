package logic

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/xuri/excelize/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ExportUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportUsersLogic {
	return &ExportUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportUsersLogic) ExportUsers(in *pb.ExportUsersRequest) (*pb.ExportUsersResponse, error) {
	if in == nil {
		in = &pb.ExportUsersRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	format := strings.TrimSpace(in.Format)
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" && format != "csv" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FORMAT", "无效导出格式", map[string]string{"format": "invalid"})
	}

	// Build filter parameters from request
	var status, role, keyword, sortBy, sortOrder string
	var twoFactorEnabled, bypassWithdrawAudit *bool
	var createdFrom, createdTo *time.Time

	l.Logger.Infof("export users: raw request filters: %+v", in.Filters)

	if in.Filters != nil {
		status = strings.TrimSpace(in.Filters.Status)
		role = strings.TrimSpace(in.Filters.Role)
		keyword = strings.TrimSpace(in.Filters.Keyword)
		sortBy = strings.TrimSpace(in.Filters.SortBy)
		sortOrder = strings.TrimSpace(in.Filters.SortOrder)

		if in.Filters.TwoFactorEnabled != nil {
			v := in.Filters.TwoFactorEnabled.Value
			twoFactorEnabled = &v
		}
		if in.Filters.BypassWithdrawAudit != nil {
			v := in.Filters.BypassWithdrawAudit.Value
			bypassWithdrawAudit = &v
		}

		var err error
		createdFrom, createdTo, err = parseDateFromTo(in.Filters.CreatedFrom, in.Filters.CreatedTo)
		if err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date range", map[string]string{"created_from": "invalid"})
		}
	}

	l.Logger.Infof("export users: parsed query params - status=%q, role=%q, keyword=%q, sortBy=%q, sortOrder=%q, twoFactorEnabled=%v, bypassWithdrawAudit=%v, createdFrom=%v, createdTo=%v",
		status, role, keyword, sortBy, sortOrder, twoFactorEnabled, bypassWithdrawAudit, createdFrom, createdTo)

	// Fetch all matching users (with pagination to avoid memory overflow)
	const maxExportRows int64 = 10000
	const pageSize int32 = 200
	var (
		total int64
		users []*model.UserModel
		page  int32 = 1
	)

	for {
		batch, t, err := l.svcCtx.UserRepo.List(l.ctx, page, pageSize, status, role, keyword, createdFrom, createdTo, sortBy, sortOrder, twoFactorEnabled, bypassWithdrawAudit)
		if err != nil {
			l.Logger.Errorf("export users query failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
		}
		if page == 1 {
			total = t
			l.Logger.Infof("export users: found %d total users", total)
			if total > maxExportRows {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "TOO_MANY_RECORDS", "导出记录过多，请缩小筛选范围", map[string]string{"max_rows": strconv.FormatInt(maxExportRows, 10)})
			}
			if total == 0 {
				l.Logger.Infof("export users: no users found with filters: status=%s, role=%s, keyword=%s", status, role, keyword)
			}
		}
		if len(batch) == 0 {
			break
		}
		users = append(users, batch...)
		if int64(len(users)) >= total || int64(len(users)) >= maxExportRows {
			break
		}
		page++
	}

	l.Logger.Infof("export users: fetched %d users for export (format: %s)", len(users), format)

	// Generate file content based on format
	var content []byte
	var fileName string
	var err error

	if format == "csv" {
		content, err = l.generateCSV(users, in.Fields)
		fileName = fmt.Sprintf("users_%s.csv", time.Now().UTC().Format("20060102_150405"))
	} else {
		content, err = l.generateXLSX(users, in.Fields)
		fileName = fmt.Sprintf("users_%s.xlsx", time.Now().UTC().Format("20060102_150405"))
	}

	if err != nil {
		l.Logger.Errorf("export users generate file failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "export failed", nil)
	}

	l.Logger.Infof("export users: generated file size: %d bytes", len(content))

	// Best-effort audit log
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"format":  format,
		"rows":    len(users),
		"filters": in.Filters,
		"fields":  in.Fields,
	})
	_ = repository.NewAdminAuditLogRepository(l.svcCtx.DB).CreateLog(l.ctx, &model.AdminAuditLogModel{
		AdminID:     current.ID,
		Action:      "user.export",
		TargetType:  "user",
		TargetID:    "",
		Description: "导出用户列表",
		Details:     detailsBytes,
		IP:          ip,
		UserAgent:   ua,
	})

	return &pb.ExportUsersResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ExportUsersData{
			Format:   format,
			FileName: fileName,
			Content:  content,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func (l *ExportUsersLogic) generateCSV(users []*model.UserModel, fields []string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header
	headers := l.getExportHeaders(fields)
	l.Logger.Infof("CSV export: writing %d headers: %v", len(headers), headers)
	if err := w.Write(headers); err != nil {
		return nil, err
	}

	// Write data rows
	dataRowCount := 0
	for _, user := range users {
		if user == nil {
			continue
		}
		row := l.getUserRow(user, fields)
		if err := w.Write(row); err != nil {
			return nil, err
		}
		dataRowCount++
	}
	l.Logger.Infof("CSV export: wrote %d data rows", dataRowCount)

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	result := buf.Bytes()
	l.Logger.Infof("CSV export: final buffer size: %d bytes, content preview: %q", len(result), string(result[:min(len(result), 200)]))
	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (l *ExportUsersLogic) generateXLSX(users []*model.UserModel, fields []string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Users"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(index)

	// Write header
	headers := l.getExportHeaders(fields)
	for i, header := range headers {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return nil, err
		}
	}

	// Write data rows
	for rowIdx, user := range users {
		if user == nil {
			continue
		}
		row := l.getUserRow(user, fields)
		for colIdx, value := range row {
			cell := fmt.Sprintf("%s%d", string(rune('A'+colIdx)), rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				return nil, err
			}
		}
	}

	// Delete default Sheet1 if exists
	if f.GetSheetName(0) == "Sheet1" && sheetName != "Sheet1" {
		f.DeleteSheet("Sheet1")
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (l *ExportUsersLogic) getExportHeaders(fields []string) []string {
	if len(fields) == 0 {
		// Default fields - 中文列头（与列表截图完全一致）
		return []string{
			"UID", "邮箱", "手机", "昵称",
			"角色", "2FA", "创建时间", "最近登录", "最近登录IP",
		}
	}

	// Custom fields - 中文列头（与前端传参字段名匹配）
	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		switch field {
		case "uid":
			headers = append(headers, "UID")
		case "email":
			headers = append(headers, "邮箱")
		case "phone":
			headers = append(headers, "手机")
		case "name":
			headers = append(headers, "昵称")
		case "status":
			headers = append(headers, "状态")
		case "role":
			headers = append(headers, "角色")
		case "two_factor_enabled":
			headers = append(headers, "2FA")
		case "created_at":
			headers = append(headers, "创建时间")
		case "last_login_at":
			headers = append(headers, "最近登录")
		case "last_login_ip":
			headers = append(headers, "最近登录IP")
		case "country_code":
			headers = append(headers, "国家代码")
		case "avatar":
			headers = append(headers, "头像")
		case "kyc_status":
			headers = append(headers, "KYC状态")
		case "kyc_level":
			headers = append(headers, "KYC等级")
		case "member_level":
			headers = append(headers, "会员等级")
		case "freeze_assets":
			headers = append(headers, "资产冻结状态")
		case "freeze_reason":
			headers = append(headers, "冻结原因")
		case "register_ip":
			headers = append(headers, "注册IP")
		case "updated_at":
			headers = append(headers, "更新时间")
		}
	}
	return headers
}

func (l *ExportUsersLogic) getUserRow(user *model.UserModel, fields []string) []string {
	if len(fields) == 0 {
		// Default fields（与列表截图完全一致）
		return []string{
			fmt.Sprintf("%d", user.ID), // UID
			user.Email,                 // 邮箱
			maskPhone(fullPhone(user.CountryCode, user.Phone)),                             // 手机（脱敏）
			maskNicknameIfDefault(user.Nickname, user.Phone, user.CountryCode, user.Email), // 昵称（智能脱敏）
			formatUserRole(user.MemberLevel),                                               // 角色
			format2FAStatus(user.Is2faEnabled),                                             // 2FA
			formatExportTime(user.CreatedAt),                                               // 创建时间
			formatExportTime(user.LastLoginTime),                                           // 最近登录
			user.LastLoginIp,                                                               // 最近登录IP
		}
	}

	// Custom fields（与前端传参字段名匹配）
	row := make([]string, 0, len(fields))
	for _, field := range fields {
		switch field {
		case "uid":
			row = append(row, fmt.Sprintf("%d", user.ID))
		case "email":
			row = append(row, user.Email)
		case "phone":
			row = append(row, maskPhone(fullPhone(user.CountryCode, user.Phone))) // 脱敏手机号
		case "name":
			row = append(row, maskNicknameIfDefault(user.Nickname, user.Phone, user.CountryCode, user.Email)) // 智能脱敏昵称
		case "status":
			row = append(row, formatUserStatusForExport(user.Status, user.KycLevel))
		case "role":
			row = append(row, formatUserRole(user.MemberLevel))
		case "two_factor_enabled":
			row = append(row, format2FAStatus(user.Is2faEnabled))
		case "created_at":
			row = append(row, formatExportTime(user.CreatedAt))
		case "last_login_at":
			row = append(row, formatExportTime(user.LastLoginTime))
		case "last_login_ip":
			row = append(row, user.LastLoginIp)
		case "country_code":
			row = append(row, user.CountryCode)
		case "avatar":
			row = append(row, user.Avatar)
		case "kyc_status":
			row = append(row, formatKycStatus(user.KycLevel))
		case "kyc_level":
			row = append(row, fmt.Sprintf("%d", user.KycLevel))
		case "member_level":
			row = append(row, fmt.Sprintf("%d", user.MemberLevel))
		case "freeze_assets":
			row = append(row, formatBool(false))
		case "freeze_reason":
			row = append(row, "")
		case "register_ip":
			row = append(row, user.RegisterIp)
		case "updated_at":
			row = append(row, formatExportTime(user.UpdatedAt))
		}
	}
	return row
}

func formatBool(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// formatUserStatusForExport 格式化用户状态（中文，与列表逻辑保持一致）
// 使用与 listUsersLogic 相同的逻辑：userStatusFrom(status, kycLevel)
func formatUserStatusForExport(status int32, kycLevel int32) string {
	switch status {
	case 2:
		return "冻结"
	case 3:
		return "终止"
	default:
		if kycLevel <= 0 {
			return "待KYC"
		}
		return "正常"
	}
}

// formatUserRole 格式化用户角色（中文，与列表逻辑保持一致）
func formatUserRole(memberLevel int32) string {
	if memberLevel >= 2 {
		return "VIP"
	}
	return "普通用户"
}

// formatKycStatus 格式化KYC状态（中文，与列表逻辑保持一致）
func formatKycStatus(kycLevel int32) string {
	if kycLevel > 0 {
		return "已认证"
	}
	return "待认证"
}

// formatExportTime 格式化时间为导出专用格式：yyyy-MM-dd HH:mm:ss
func formatExportTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// 使用本地时区格式化
	return t.Local().Format("2006-01-02 15:04:05")
}

// formatPhoneWithCountryCode 格式化手机号（包含国家代码）
// 例如：+8618121111452
func formatPhoneWithCountryCode(countryCode, phone string) string {
	if phone == "" {
		return ""
	}
	if countryCode == "" {
		return phone
	}
	// 确保国家代码没有 + 前缀
	cc := strings.TrimPrefix(strings.TrimSpace(countryCode), "+")
	return "+" + cc + strings.TrimSpace(phone)
}

// format2FAStatus 格式化2FA状态（与列表显示一致）
func format2FAStatus(enabled bool) string {
	if enabled {
		return "已开启"
	}
	return "未开启"
}
