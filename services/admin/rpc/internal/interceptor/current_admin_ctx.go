package interceptor

import (
	"context"

	"internalwallet/services/admin/rpc/internal/model"
)

// ContextWithCurrentAdmin injects the current admin into the context.
// This is primarily used by internal unit tests and helpers.
func ContextWithCurrentAdmin(ctx context.Context, admin *model.AdminUserModel) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, currentAdminKey, admin)
}
