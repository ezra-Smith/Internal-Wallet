package logic

import (
	"testing"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"github.com/stretchr/testify/require"
)

func TestDedupeMenuModels_DedupesByPathAndKeepsBetterRow(t *testing.T) {
	t.Parallel()

	key := "rbac.admins"
	path := "/rbac/admins"
	tree := "70"

	now := time.Now()
	earlier := now.Add(-time.Minute)

	in := []*model.AdminMenuModel{
		{
			ID:        73,
			Pid:       70,
			Path:      &path,
			Key:       &key,
			Tree:      &tree,
			UpdatedAt: &now,
		},
		{
			ID:        74,
			Pid:       70,
			Path:      &path,
			Key:       &key,
			Tree:      nil,
			UpdatedAt: &earlier,
		},
	}

	out, removed := dedupeMenuModels(in)
	require.Equal(t, 1, removed)
	require.Len(t, out, 1)
	require.Equal(t, int64(73), out[0].ID)
}

func TestDedupeMenuModels_RewiresChildrenOfRemovedDuplicates(t *testing.T) {
	t.Parallel()

	path := "/parent"
	key := "parent"
	tree := "x"

	in := []*model.AdminMenuModel{
		{ID: 1, Pid: 0, Path: &path, Key: &key, Tree: &tree},
		{ID: 2, Pid: 0, Path: &path, Key: &key, Tree: nil},
		{ID: 3, Pid: 1, Path: strPtrOrNil("/parent/a"), Key: strPtrOrNil("parent.a")},
		{ID: 4, Pid: 2, Path: strPtrOrNil("/parent/b"), Key: strPtrOrNil("parent.b")},
	}

	out, removed := dedupeMenuModels(in)
	require.Equal(t, 1, removed)
	require.Len(t, out, 3)

	var parent *model.AdminMenuModel
	var childA *model.AdminMenuModel
	var childB *model.AdminMenuModel
	for _, m := range out {
		switch m.ID {
		case 1:
			parent = m
		case 3:
			childA = m
		case 4:
			childB = m
		case 2:
			t.Fatalf("unexpected duplicate parent kept: %v", m.ID)
		}
	}

	require.NotNil(t, parent)
	require.NotNil(t, childA)
	require.NotNil(t, childB)
	require.Equal(t, int64(1), childA.Pid)
	require.Equal(t, int64(1), childB.Pid)
}
