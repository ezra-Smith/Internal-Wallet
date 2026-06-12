package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/auditctx"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminAuditLogRepository interface {
	commonRepo.BaseRepository[model.AdminAuditLogModel]

	CreateLog(ctx context.Context, m *model.AdminAuditLogModel) error
	List(ctx context.Context, page, pageSize int32, f AuditLogListFilter) ([]*model.AdminAuditLogModel, int64, *AuditLogStats, error)
}

type AuditLogStats struct {
	Total   int64
	Success int64
	Failed  int64
	Today   int64
}

type AuditLogListFilter struct {
	OperatorID int64

	Role       string
	Action     string
	TargetType string
	TargetID   string
	Module     string

	IP         string
	RequestID  string
	HttpMethod string
	HttpPath   string
	RpcMethod  string

	DurationMsFrom *int32
	DurationMsTo   *int32

	Keyword string
	Success *bool

	DateFrom *time.Time
	DateTo   *time.Time
}

type adminAuditLogRepo struct {
	commonRepo.BaseRepository[model.AdminAuditLogModel]
}

func NewAdminAuditLogRepository(db *gorm.DB) AdminAuditLogRepository {
	return &adminAuditLogRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminAuditLogModel](db),
	}
}

func (r *adminAuditLogRepo) CreateLog(ctx context.Context, m *model.AdminAuditLogModel) error {
	if m == nil {
		return nil
	}

	// If the audit interceptor already created a request-level log, treat CreateLog as
	// a "best-effort enrich" operation to avoid duplicated rows.
	if id, ok := auditctx.AuditLogID(ctx); ok && id > 0 {
		return r.enrichRequestLog(ctx, id, m)
	}

	return r.GetDB().WithContext(ctx).Create(m).Error
}

func (r *adminAuditLogRepo) enrichRequestLog(ctx context.Context, id int64, m *model.AdminAuditLogModel) error {
	updates := map[string]interface{}{}

	if m.AdminID > 0 {
		updates["admin_id"] = m.AdminID
	}
	if v := strings.TrimSpace(m.Action); v != "" {
		updates["action"] = v
	}
	if v := strings.TrimSpace(m.TargetType); v != "" {
		updates["target_type"] = v
	}
	if v := strings.TrimSpace(m.TargetID); v != "" {
		updates["target_id"] = v
	}
	if v := strings.TrimSpace(m.Description); v != "" {
		updates["description"] = v
	}
	if v := strings.TrimSpace(m.IP); v != "" {
		updates["ip"] = v
	}
	if v := strings.TrimSpace(m.UserAgent); v != "" {
		updates["user_agent"] = v
	}

	// v2 fields (optional enrich)
	if v := strings.TrimSpace(m.RequestID); v != "" {
		updates["request_id"] = v
	}
	if v := strings.TrimSpace(m.RpcMethod); v != "" {
		updates["rpc_method"] = v
	}
	if v := strings.TrimSpace(m.HttpMethod); v != "" {
		updates["http_method"] = v
	}
	if v := strings.TrimSpace(m.HttpPath); v != "" {
		updates["http_path"] = v
	}
	if v := strings.TrimSpace(m.OperatorEmail); v != "" {
		updates["operator_email"] = v
	}
	if v := strings.TrimSpace(m.IdempotencyKey); v != "" {
		updates["idempotency_key"] = v
	}
	if v := strings.TrimSpace(m.Module); v != "" {
		updates["module"] = v
	}

	if len(m.Details) > 0 {
		merged, err := r.mergeDetails(ctx, id, m.Details)
		if err == nil && len(merged) > 0 {
			updates["details"] = merged
		}
	}

	if len(updates) == 0 {
		return nil
	}

	return r.GetDB().WithContext(ctx).
		Model(&model.AdminAuditLogModel{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *adminAuditLogRepo) mergeDetails(ctx context.Context, id int64, bizDetails []byte) ([]byte, error) {
	var existing struct {
		Details []byte `gorm:"column:details"`
	}
	if err := r.GetDB().WithContext(ctx).
		Model(&model.AdminAuditLogModel{}).
		Select("details").
		Where("id = ?", id).
		First(&existing).Error; err != nil {
		return bizDetails, nil
	}

	base := map[string]interface{}{}
	if len(existing.Details) > 0 {
		_ = json.Unmarshal(existing.Details, &base)
	}
	if base == nil {
		base = map[string]interface{}{}
	}

	var biz interface{}
	if len(bizDetails) > 0 {
		_ = json.Unmarshal(bizDetails, &biz)
	}
	if biz != nil {
		// Keep interceptor-generated payload stable; attach business-specific info under "biz".
		base["biz"] = biz
	}

	out, err := json.Marshal(base)
	if err != nil {
		return bizDetails, nil
	}
	return out, nil
}

func (r *adminAuditLogRepo) List(ctx context.Context, page, pageSize int32, f AuditLogListFilter) ([]*model.AdminAuditLogModel, int64, *AuditLogStats, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.GetDB().WithContext(ctx).Model(&model.AdminAuditLogModel{})
	if f.OperatorID > 0 {
		query = query.Where("admin_audit_logs.admin_id = ?", f.OperatorID)
	}
	if strings.TrimSpace(f.Role) != "" {
		query = query.Joins("JOIN admin_users au ON au.id = admin_audit_logs.admin_id AND au.deleted_at IS NULL").
			Where("au.role = ?", strings.TrimSpace(f.Role))
	}
	if strings.TrimSpace(f.Action) != "" {
		query = query.Where("admin_audit_logs.action = ?", strings.TrimSpace(f.Action))
	}
	if strings.TrimSpace(f.TargetType) != "" {
		query = query.Where("admin_audit_logs.target_type = ?", strings.TrimSpace(f.TargetType))
	}
	if strings.TrimSpace(f.TargetID) != "" {
		query = query.Where("admin_audit_logs.target_id = ?", strings.TrimSpace(f.TargetID))
	}
	if strings.TrimSpace(f.Module) != "" && strings.TrimSpace(f.Module) != "all" {
		query = query.Where("admin_audit_logs.module = ?", strings.TrimSpace(f.Module))
	}
	if strings.TrimSpace(f.IP) != "" {
		query = query.Where("admin_audit_logs.ip = ?", strings.TrimSpace(f.IP))
	}
	if strings.TrimSpace(f.RequestID) != "" {
		query = query.Where("admin_audit_logs.request_id = ?", strings.TrimSpace(f.RequestID))
	}
	if strings.TrimSpace(f.HttpMethod) != "" {
		query = query.Where("admin_audit_logs.http_method = ?", strings.ToUpper(strings.TrimSpace(f.HttpMethod)))
	}
	if strings.TrimSpace(f.HttpPath) != "" {
		kw := "%" + strings.TrimSpace(f.HttpPath) + "%"
		query = query.Where("admin_audit_logs.http_path LIKE ?", kw)
	}
	if strings.TrimSpace(f.RpcMethod) != "" {
		kw := "%" + strings.TrimSpace(f.RpcMethod) + "%"
		query = query.Where("admin_audit_logs.rpc_method LIKE ?", kw)
	}
	if f.DurationMsFrom != nil && *f.DurationMsFrom > 0 {
		query = query.Where("admin_audit_logs.duration_ms >= ?", *f.DurationMsFrom)
	}
	if f.DurationMsTo != nil && *f.DurationMsTo > 0 {
		query = query.Where("admin_audit_logs.duration_ms <= ?", *f.DurationMsTo)
	}
	if f.Success != nil {
		query = query.Where("admin_audit_logs.success = ?", *f.Success)
	}
	if f.DateFrom != nil && !f.DateFrom.IsZero() {
		query = query.Where("admin_audit_logs.created_at >= ?", *f.DateFrom)
	}
	if f.DateTo != nil && !f.DateTo.IsZero() {
		query = query.Where("admin_audit_logs.created_at <= ?", *f.DateTo)
	}
	if strings.TrimSpace(f.Keyword) != "" {
		kw := "%" + strings.TrimSpace(f.Keyword) + "%"
		query = query.Where(
			"admin_audit_logs.action LIKE ? OR admin_audit_logs.target_id LIKE ? OR admin_audit_logs.description LIKE ? OR admin_audit_logs.request_id LIKE ? OR admin_audit_logs.rpc_method LIKE ? OR admin_audit_logs.http_path LIKE ? OR admin_audit_logs.operator_email LIKE ? OR admin_audit_logs.ip LIKE ?",
			kw, kw, kw, kw, kw, kw, kw, kw,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, nil, err
	}

	var items []*model.AdminAuditLogModel
	offset := int((page - 1) * pageSize)
	err := query.Order("id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	if err != nil {
		return nil, 0, nil, err
	}

	stats := &AuditLogStats{Total: total}

	// success/failed counts (within current filters)
	{
		var okCnt int64
		_ = query.Session(&gorm.Session{}).Where("admin_audit_logs.success = 1").Count(&okCnt).Error
		stats.Success = okCnt
		stats.Failed = total - okCnt
	}

	// today count (within current filters)
	{
		now := time.Now()
		y, m, d := now.Date()
		start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		var todayCnt int64
		_ = query.Session(&gorm.Session{}).Where("admin_audit_logs.created_at >= ? AND admin_audit_logs.created_at < ?", start, end).Count(&todayCnt).Error
		stats.Today = todayCnt
	}

	return items, total, stats, nil
}
