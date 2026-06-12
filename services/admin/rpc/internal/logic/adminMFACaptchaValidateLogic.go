package logic

import (
	"context"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AdminMFACaptchaValidateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminMFACaptchaValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMFACaptchaValidateLogic {
	return &AdminMFACaptchaValidateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminMFACaptchaValidateLogic) AdminMFACaptchaValidate(in *pb.AdminMFACaptchaValidateRequest) (*pb.AdminMFACaptchaValidateResponse, error) {
	if in == nil || in.CaptchaKey == "" || in.CaptchaAnswer == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"captcha_key":    "required",
			"captcha_answer": "required",
		})
	}
	if l.svcCtx.MFAFlow == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "mfa flow not configured", nil)
	}

	ok, err := l.svcCtx.MFAFlow.VerifyAndConsumeCaptcha(l.ctx, in.CaptchaKey, in.CaptchaAnswer)
	if err != nil {
		l.Logger.Errorf("mfa captcha validate failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if !ok {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeAuthCaptchaInvalid, "AUTH_CAPTCHA_INVALID", "captcha invalid", nil)
	}

	clientIP := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	tempKey, err := l.svcCtx.MFAFlow.CreateTempKey(l.ctx, clientIP, ua)
	if err != nil {
		l.Logger.Errorf("mfa temp key create failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.AdminMFACaptchaValidateResponse{
		Success:   true,
		TempKey:   tempKey,
		Message:   resp.Msg(l.ctx, "MFA_CAPTCHA_VALIDATED"),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
