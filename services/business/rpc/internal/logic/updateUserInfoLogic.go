package logic

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserInfoLogic {
	return &UpdateUserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserInfoLogic) UpdateUserInfo(in *pb.UpdateUserInfoReq) (*pb.UpdateUserInfoResp, error) {
	uidStr := strings.TrimSpace(middleware.GetUserID(l.ctx))
	if uidStr == "" {
		l.Logger.Error("UpdateUserInfo: missing user id in context")
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		l.Logger.Errorf("UpdateUserInfo: invalid user id format: %q, err=%v", uidStr, err)
		return nil, errx.InvalidParam("invalid user id")
	}

	if l.svcCtx.UserAccountRepository == nil {
		l.Logger.Error("UpdateUserInfo: UserAccountRepository not initialized (likely DB/GORM config missing)")
		return nil, errx.ServiceNotAvailable("db")
	}

	fields := map[string]interface{}{}

	nickname := strings.TrimSpace(in.Nickname)
	if nickname != "" {
		fields["nickname"] = nickname
	}

	avatarURL := strings.TrimSpace(in.AvatarUrl)
	if avatarURL != "" {
		normalized, vErr := normalizeAvatarURL(avatarURL)
		if vErr != nil {
			return nil, errx.InvalidParam("invalid avatar_url")
		}
		fields["avatar"] = normalized
	}

	if len(fields) == 0 {
		return &pb.UpdateUserInfoResp{Success: true, Message: "ok"}, nil
	}

	if err := l.svcCtx.UserAccountRepository.UpdateFields(l.ctx, uid, fields); err != nil {
		return nil, errx.DBError()
	}
	return &pb.UpdateUserInfoResp{Success: true, Message: "ok"}, nil
}

func normalizeAvatarURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 255 {
		return "", errors.New("avatar_url too long")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", errors.New("invalid avatar_url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid avatar_url")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("invalid avatar_url")
	}
	return raw, nil
}
