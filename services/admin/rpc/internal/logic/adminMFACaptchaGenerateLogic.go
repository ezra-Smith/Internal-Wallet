package logic

import (
	"context"
	"time"

	"internalwallet/common/captcha"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AdminMFACaptchaGenerateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFACaptchaGenerateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFACaptchaGenerateLogic {
	return &AdminMFACaptchaGenerateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFACaptchaGenerateLogic) AdminMFACaptchaGenerate(in *pb.AdminMFACaptchaGenerateRequest) (*pb.AdminMFACaptchaGenerateResponse, error) {
	_ = in
	if l.svcCtx.RedisClient == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "redis not configured", nil)
	}

	store := captcha.NewRedisStoreWithPrefix(l.svcCtx.RedisClient, 5*time.Minute, "admin:mfa:captcha:")
	generator := captcha.NewSecureDigitGenerator(store)
	c, err := generator.Generate(l.ctx)
	if err != nil {
		l.Logger.Errorf("mfa captcha generate failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to generate captcha", nil)
	}

	return &pb.AdminMFACaptchaGenerateResponse{
		Success:      true,
		CaptchaImage: c.ImageData,
		CaptchaKey:   c.CaptchaID,
		RequestId:    resp.RequestID(l.ctx),
		Timestamp:    resp.Timestamp(),
	}, nil
}
