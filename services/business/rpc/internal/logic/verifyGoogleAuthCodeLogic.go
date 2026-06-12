package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyGoogleAuthCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyGoogleAuthCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyGoogleAuthCodeLogic {
	return &VerifyGoogleAuthCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VerifyGoogleAuthCode 校验Google验证器验证码（仅校验，不做任何状态修改）
func (l *VerifyGoogleAuthCodeLogic) VerifyGoogleAuthCode(in *pb.VerifyGoogleAuthCodeReq) (*pb.VerifyGoogleAuthCodeResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	googleCode := strings.TrimSpace(in.GoogleCode)
	if googleCode == "" {
		return &pb.VerifyGoogleAuthCodeResp{
			Success: true,
			IsValid: false,
			Message: "google auth code is required",
		}, nil
	}

	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// Get user info to check if GA is bound
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, uid)
	if err != nil || user == nil {
		return &pb.VerifyGoogleAuthCodeResp{
			Success: true,
			IsValid: false,
			Message: "user not found",
		}, nil
	}

	// Check if Google Auth is bound
	googleSecret := strings.TrimSpace(user.GoogleAuthSecret)
	if googleSecret == "" {
		return &pb.VerifyGoogleAuthCodeResp{
			Success: true,
			IsValid: false,
			Message: "google auth not bound",
		}, nil
	}

	// Verify Google Auth code
	isValid := totp.Validate(googleCode, googleSecret)
	if isValid {
		return &pb.VerifyGoogleAuthCodeResp{
			Success: true,
			IsValid: true,
			Message: "verification code is valid",
		}, nil
	}

	return &pb.VerifyGoogleAuthCodeResp{
		Success: true,
		IsValid: false,
		Message: "verification code is invalid",
	}, nil
}
