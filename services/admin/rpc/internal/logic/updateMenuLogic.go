package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type UpdateMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuLogic {
	return &UpdateMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateMenuLogic) UpdateMenu(in *pb.UpdateMenuRequest) (*pb.UpdateMenuResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	now := time.Now()
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		menuRepo := l.svcCtx.AdminMenuRepo.WithTx(tx)

		m, err := menuRepo.FindByID(l.ctx, in.Id)
		if err != nil || m == nil {
			return errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "menu not found", nil)
		}

		fields := map[string]interface{}{
			"updated_at": &now,
		}

		pidChanged := false
		newPid := m.Pid
		newLevel := m.Level
		newTree := m.Tree

		if in.Pid != nil {
			pidChanged = true
			newPid = in.Pid.Value
			if newPid < 0 {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid pid", map[string]string{"pid": "invalid"})
			}
			if newPid == m.ID {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "circular parent", map[string]string{"pid": "circular"})
			}

			if newPid == 0 {
				newLevel = 1
				newTree = nil
			} else {
				parent, err := menuRepo.FindByID(l.ctx, newPid)
				if err != nil || parent == nil {
					return errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "parent menu not found", nil)
				}
				if treeContainsID(parent.Tree, m.ID) {
					return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "circular parent", map[string]string{"pid": "circular"})
				}
				newLevel = parent.Level + 1
				newTree = buildChildTree(parent.Tree, parent.ID)
			}

			fields["pid"] = newPid
			fields["level"] = newLevel
			fields["tree"] = newTree
		}

		if in.Name != nil {
			v := strings.TrimSpace(in.Name.Value)
			if v == "" {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid name", map[string]string{"name": "required"})
			}
			fields["name"] = v
		}
		if in.Path != nil {
			fields["path"] = strPtrOrNil(in.Path.Value)
		}
		if in.Icon != nil {
			fields["icon"] = strPtrOrNil(in.Icon.Value)
		}
		if in.HideInMenu != nil {
			fields["hide_in_menu"] = in.HideInMenu.Value
		}
		if in.HideChildrenInMenu != nil {
			fields["hide_childrenIn_menu"] = in.HideChildrenInMenu.Value
		}
		if in.Sort != nil {
			fields["sort"] = int(in.Sort.Value)
		}
		if in.Remark != nil {
			fields["remark"] = strPtrOrNil(in.Remark.Value)
		}
		if in.Status != nil {
			fields["status"] = in.Status.Value
		}
		if in.Target != nil {
			fields["target"] = strPtrOrNil(in.Target.Value)
		}
		if in.Access != nil {
			fields["access"] = strPtrOrNil(in.Access.Value)
		}
		if in.Key != nil {
			fields["key"] = strPtrOrNil(in.Key.Value)
		}

		if err := menuRepo.UpdateFields(l.ctx, m.ID, fields); err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update menu failed", nil)
		}

		if pidChanged {
			type node struct {
				id    int64
				level int
				tree  *string
			}
			layer := []node{{id: m.ID, level: newLevel, tree: newTree}}
			seen := map[int64]bool{m.ID: true}

			for len(layer) > 0 {
				parentIDs := make([]int64, 0, len(layer))
				parentInfo := make(map[int64]node, len(layer))
				for _, n := range layer {
					parentIDs = append(parentIDs, n.id)
					parentInfo[n.id] = n
				}

				kids, err := menuRepo.ListByPidIn(l.ctx, parentIDs)
				if err != nil {
					return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update descendants failed", nil)
				}
				next := make([]node, 0, len(kids))
				for _, child := range kids {
					if child == nil || seen[child.ID] {
						continue
					}
					seen[child.ID] = true
					p := parentInfo[child.Pid]
					cl := p.level + 1
					ct := buildChildTree(p.tree, p.id)

					if err := menuRepo.UpdateFields(l.ctx, child.ID, map[string]interface{}{
						"level":      cl,
						"tree":       ct,
						"updated_at": &now,
					}); err != nil {
						return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update descendants failed", nil)
					}
					next = append(next, node{id: child.ID, level: cl, tree: ct})
				}
				layer = next
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	updated, err := l.svcCtx.AdminMenuRepo.FindByID(l.ctx, in.Id)
	if err != nil || updated == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "menu updated but reload failed", nil)
	}

	return &pb.UpdateMenuResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateMenuData{
			Menu: toPBAdminMenu(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
