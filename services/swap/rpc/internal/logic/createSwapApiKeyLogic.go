package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/security"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateSwapApiKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSwapApiKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSwapApiKeyLogic {
	return &CreateSwapApiKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== API Key Management (admin-protected) ====================
func (l *CreateSwapApiKeyLogic) CreateSwapApiKey(in *pb.CreateSwapApiKeyRequest) (*pb.CreateSwapApiKeyResponse, error) {
	if in == nil || strings.TrimSpace(in.ProjectName) == "" {
		return nil, status.Error(codes.InvalidArgument, "project_name is required")
	}
	if l.svcCtx.DB == nil || l.svcCtx.ApiKeyRepo == nil {
		return nil, status.Error(codes.Internal, "db not configured")
	}
	pepper := strings.TrimSpace(l.svcCtx.Config.Swap.Auth.ApiKeyPepper)
	if pepper == "" {
		return nil, status.Error(codes.Internal, "swap api key pepper not configured")
	}

	projectName := strings.TrimSpace(in.ProjectName)
	keyName := strings.TrimSpace(in.KeyName)
	tier := strings.TrimSpace(in.RateLimitTier)
	if tier == "" {
		tier = strings.TrimSpace(l.svcCtx.Config.Swap.Auth.DefaultTier)
	}
	if tier == "" {
		tier = "default"
	}

	var expiresAt *time.Time
	if in.ExpiresAtUnix > 0 {
		t := time.Unix(in.ExpiresAtUnix, 0).Local()
		expiresAt = &t
	}

	// Check if key_name already exists for this project (if provided)
	if keyName != "" {
		existingKeys, err := l.svcCtx.ApiKeyRepo.ListByProjectName(l.ctx, projectName)
		if err == nil {
			for _, existingKey := range existingKeys {
				if existingKey.KeyName == keyName {
					return nil, status.Errorf(codes.AlreadyExists,
						"api key with name '%s' already exists for project '%s'", keyName, projectName)
				}
			}
		}
	}

	for i := 0; i < 3; i++ {
		apiKey, err := security.GenerateAPIKey()
		if err != nil {
			l.Logger.Errorw("generate api key failed", logx.Field("error", err))
			return nil, status.Error(codes.Internal, "failed to generate api key")
		}
		keyHash, err := security.HashAPIKey(pepper, apiKey)
		if err != nil {
			l.Logger.Errorw("hash api key failed", logx.Field("error", err))
			return nil, status.Error(codes.Internal, "failed to hash api key")
		}
		//这里赋值进去，保存到库，便于查看
		tokenDisplay := apiKey
		m := &model.SwapSvcApiKeyModel{
			KeyHash:       keyHash,
			ProjectName:   projectName,
			KeyName:       keyName,
			RateLimitTier: tier,
			IsActive:      true,
			ExpiresAt:     expiresAt,
			TokenDisplay:  tokenDisplay,
		}
		if err := l.svcCtx.ApiKeyRepo.Create(l.ctx, m); err != nil {
			// extremely unlikely, but retry on duplicate key hash
			if isDuplicateKeyErr(err) {
				continue
			}
			l.Logger.Errorw("create swap api key failed", logx.Field("error", err))
			return nil, status.Error(codes.Internal, "failed to create api key")
		}
		var expiresAtUnix int64
		if expiresAt != nil {
			expiresAtUnix = expiresAt.Unix()
		}
		return &pb.CreateSwapApiKeyResponse{
			Success: true,
			Message: "ok",
			ApiKey:  apiKey,
			Item: &pb.SwapApiKeyItem{
				Id:            m.ID,
				ProjectName:   m.ProjectName,
				KeyName:       m.KeyName,
				RateLimitTier: m.RateLimitTier,
				IsActive:      m.IsActive,
				TokenDisplay:  m.TokenDisplay,
				CreatedAtUnix: m.CreatedAt.Unix(),
				ExpiresAtUnix: expiresAtUnix,
			},
		}, nil
	}

	return nil, status.Error(codes.Internal, "failed to create api key")
}

func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	// MySQL/MariaDB duplicate key: Error 1062 (HY000): Duplicate entry ...
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "Error 1062")
}
