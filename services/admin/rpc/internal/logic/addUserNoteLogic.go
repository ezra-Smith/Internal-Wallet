package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type AddUserNoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddUserNoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddUserNoteLogic {
	return &AddUserNoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddUserNoteLogic) AddUserNote(in *pb.AddUserNoteRequest) (*pb.AddUserNoteResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CONTENT", "content required", map[string]string{"content": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.UserAdminNoteRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	if u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid); err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"content":      strings.TrimSpace(in.Content),
		"is_important": in.IsImportant,
	})

	note := &model.UserAdminNoteModel{
		UserID:          uid,
		Content:         strings.TrimSpace(in.Content),
		IsImportant:     in.IsImportant,
		OperatorAdminID: current.ID,
	}

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		noteRepo := repository.NewUserAdminNoteRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := noteRepo.Create(l.ctx, note); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.add_note",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "添加用户备注: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("add user note failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.AddUserNoteResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "NOTE_ADDED"),
		Data: &pb.AddUserNoteData{
			Id:          fmt.Sprintf("%d", note.ID),
			Uid:         uidStr,
			Content:     note.Content,
			IsImportant: note.IsImportant,
			Operator:    current.Username,
			CreatedAt:   formatTime(note.CreatedAt),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
