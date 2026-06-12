package auditctx

import "context"

type ctxKey string

const auditLogIDKey ctxKey = "admin-audit-log-id"

// WithAuditLogID stores the created audit log id into context for the current request.
func WithAuditLogID(ctx context.Context, id int64) context.Context {
	if ctx == nil || id <= 0 {
		return ctx
	}
	return context.WithValue(ctx, auditLogIDKey, id)
}

// AuditLogID returns the audit log id from context (if any).
func AuditLogID(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	v := ctx.Value(auditLogIDKey)
	if v == nil {
		return 0, false
	}
	id, ok := v.(int64)
	if !ok || id <= 0 {
		return 0, false
	}
	return id, true
}
