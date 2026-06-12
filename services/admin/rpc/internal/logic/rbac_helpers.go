package logic

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
)

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtrOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func timeOrEmpty(t *time.Time) string {
	return formatTimePtr(t)
}

func int64Ptr(v int64) *int64 { return &v }

func buildChildTree(parentTree *string, parentID int64) *string {
	if parentID <= 0 {
		return nil
	}
	if parentTree == nil || strings.TrimSpace(*parentTree) == "" {
		s := strconv.FormatInt(parentID, 10)
		return &s
	}
	s := strings.TrimSpace(*parentTree) + "," + strconv.FormatInt(parentID, 10)
	return &s
}

func treeContainsID(tree *string, id int64) bool {
	if tree == nil {
		return false
	}
	idStr := strconv.FormatInt(id, 10)
	t := strings.TrimSpace(*tree)
	if t == "" {
		return false
	}
	hay := "," + t + ","
	needle := "," + idStr + ","
	return strings.Contains(hay, needle)
}

func toPBAdminMenu(m *model.AdminMenuModel) *pb.AdminMenu {
	if m == nil {
		return nil
	}
	return &pb.AdminMenu{
		Id:                 m.ID,
		Pid:                m.Pid,
		Level:              int32(m.Level),
		Tree:               strOrEmpty(m.Tree),
		Name:               m.Name,
		Path:               strOrEmpty(m.Path),
		Icon:               strOrEmpty(m.Icon),
		HideInMenu:         m.HideInMenu,
		HideChildrenInMenu: m.HideChildrenInMenu,
		Sort:               int32(m.Sort),
		Remark:             strOrEmpty(m.Remark),
		Status:             m.Status,
		UpdatedAt:          timeOrEmpty(m.UpdatedAt),
		CreatedAt:          timeOrEmpty(m.CreatedAt),
		DeletedAt:          timeOrEmpty(m.DeletedAt),
		Target:             strOrEmpty(m.Target),
		Access:             strOrEmpty(m.Access),
		Key:                strOrEmpty(m.Key),
		Children:           nil,
	}
}

func toPBAdminPermission(m *model.AdminPermissionModel) *pb.AdminPermission {
	if m == nil {
		return nil
	}
	menuID := int64(0)
	if m.MenuID != nil {
		menuID = *m.MenuID
	}
	return &pb.AdminPermission{
		Id:          m.ID,
		Name:        m.Name,
		Code:        m.Code,
		Description: strOrEmpty(m.Description),
		MenuId:      menuID,
		Status:      m.Status,
		CreatedAt:   timeOrEmpty(m.CreatedAt),
		UpdatedAt:   timeOrEmpty(m.UpdatedAt),
		DeletedAt:   timeOrEmpty(m.DeletedAt),
	}
}

func toPBAdminRole(m *model.AdminRoleModel) *pb.AdminRole {
	if m == nil {
		return nil
	}
	return &pb.AdminRole{
		Id:          m.ID,
		Name:        m.Name,
		Code:        m.Code,
		Description: strOrEmpty(m.Description),
		Status:      m.Status,
		CreatedAt:   timeOrEmpty(m.CreatedAt),
		UpdatedAt:   timeOrEmpty(m.UpdatedAt),
		DeletedAt:   timeOrEmpty(m.DeletedAt),
		Menus:       nil,
		Permissions: nil,
	}
}

func uniqueInt64s(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func menuDedupeKey(m *model.AdminMenuModel) string {
	if m == nil {
		return ""
	}
	if p := strings.TrimSpace(strOrEmpty(m.Path)); p != "" {
		return "path:" + p
	}
	if k := strings.TrimSpace(strOrEmpty(m.Key)); k != "" {
		return "key:" + k
	}
	if m.ID <= 0 {
		return ""
	}
	return "id:" + strconv.FormatInt(m.ID, 10)
}

func pickBetterMenu(a, b *model.AdminMenuModel, childCount map[int64]int) *model.AdminMenuModel {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	if childCount[a.ID] != childCount[b.ID] {
		if childCount[a.ID] > childCount[b.ID] {
			return a
		}
		return b
	}

	hasKeyA := strings.TrimSpace(strOrEmpty(a.Key)) != ""
	hasKeyB := strings.TrimSpace(strOrEmpty(b.Key)) != ""
	if hasKeyA != hasKeyB {
		if hasKeyA {
			return a
		}
		return b
	}

	hasTreeA := strings.TrimSpace(strOrEmpty(a.Tree)) != ""
	hasTreeB := strings.TrimSpace(strOrEmpty(b.Tree)) != ""
	if hasTreeA != hasTreeB {
		if hasTreeA {
			return a
		}
		return b
	}

	ua := time.Time{}
	ub := time.Time{}
	if a.UpdatedAt != nil {
		ua = *a.UpdatedAt
	}
	if b.UpdatedAt != nil {
		ub = *b.UpdatedAt
	}
	if !ua.Equal(ub) {
		if ua.After(ub) {
			return a
		}
		return b
	}

	ca := time.Time{}
	cb := time.Time{}
	if a.CreatedAt != nil {
		ca = *a.CreatedAt
	}
	if b.CreatedAt != nil {
		cb = *b.CreatedAt
	}
	if !ca.Equal(cb) {
		if ca.Before(cb) {
			return a
		}
		return b
	}

	if a.ID <= b.ID {
		return a
	}
	return b
}

// dedupeMenuModels ensures the returned slice has no duplicate menus by logical identity.
// It also rewires children pointing at removed duplicate IDs to the kept canonical ID,
// so a menu tree built from the result remains connected.
func dedupeMenuModels(models []*model.AdminMenuModel) ([]*model.AdminMenuModel, int) {
	if len(models) == 0 {
		return models, 0
	}

	total := 0
	childCount := map[int64]int{}
	for _, m := range models {
		if m == nil {
			continue
		}
		total++
		if m.Pid > 0 {
			childCount[m.Pid]++
		}
	}
	if total == 0 {
		return nil, 0
	}

	type picked struct {
		m   *model.AdminMenuModel
		idx int
	}

	byKey := map[string]picked{}
	dupToKeep := map[int64]int64{}
	out := make([]*model.AdminMenuModel, 0, total)

	for _, m := range models {
		if m == nil {
			continue
		}
		k := menuDedupeKey(m)
		if k == "" {
			// No stable identity; keep as-is.
			out = append(out, m)
			continue
		}

		if p, ok := byKey[k]; !ok {
			byKey[k] = picked{m: m, idx: len(out)}
			out = append(out, m)
			continue
		} else {
			keep := pickBetterMenu(p.m, m, childCount)
			if keep == p.m {
				if m.ID > 0 && p.m != nil && p.m.ID > 0 {
					dupToKeep[m.ID] = p.m.ID
				}
				continue
			}

			if p.m != nil && p.m.ID > 0 && m.ID > 0 {
				dupToKeep[p.m.ID] = m.ID
			}
			out[p.idx] = m
			byKey[k] = picked{m: m, idx: p.idx}
		}
	}

	if len(dupToKeep) == 0 {
		return out, 0
	}

	resolveKeep := func(id int64) int64 {
		seen := map[int64]struct{}{}
		for id > 0 {
			next, ok := dupToKeep[id]
			if !ok || next <= 0 || next == id {
				return id
			}
			if _, loop := seen[id]; loop {
				return id
			}
			seen[id] = struct{}{}
			id = next
		}
		return id
	}

	for _, m := range out {
		if m == nil || m.Pid <= 0 {
			continue
		}
		if _, ok := dupToKeep[m.Pid]; !ok {
			continue
		}
		m.Pid = resolveKeep(m.Pid)
	}

	return out, total - len(out)
}

func buildMenuTreeFromModels(models []*model.AdminMenuModel, rootPid int64, filterHideInMenu bool, respectHideChildren bool) []*pb.AdminMenu {
	type key int64
	children := map[key][]*model.AdminMenuModel{}

	for _, m := range models {
		if m == nil {
			continue
		}
		if filterHideInMenu && m.HideInMenu {
			continue
		}
		children[key(m.Pid)] = append(children[key(m.Pid)], m)
	}

	less := func(a, b *model.AdminMenuModel) bool {
		if a.Sort != b.Sort {
			return a.Sort < b.Sort
		}
		ta := time.Time{}
		tb := time.Time{}
		if a.CreatedAt != nil {
			ta = *a.CreatedAt
		}
		if b.CreatedAt != nil {
			tb = *b.CreatedAt
		}
		if !ta.Equal(tb) {
			return ta.Before(tb)
		}
		return a.ID < b.ID
	}
	for pid := range children {
		s := children[pid]
		sort.Slice(s, func(i, j int) bool { return less(s[i], s[j]) })
		children[pid] = s
	}

	visited := map[int64]bool{}
	var build func(pid int64) []*pb.AdminMenu
	build = func(pid int64) []*pb.AdminMenu {
		src := children[key(pid)]
		if len(src) == 0 {
			return nil
		}
		out := make([]*pb.AdminMenu, 0, len(src))
		for _, m := range src {
			if m == nil {
				continue
			}
			if visited[m.ID] {
				continue
			}
			visited[m.ID] = true

			node := toPBAdminMenu(m)
			if respectHideChildren && m.HideChildrenInMenu {
				node.Children = nil
			} else {
				node.Children = build(m.ID)
			}
			out = append(out, node)
		}
		return out
	}

	return build(rootPid)
}
