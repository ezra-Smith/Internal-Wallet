package logic

import (
	"context"
	"testing"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateAdminLogic_CreateAdmin_ReturnsOnlyMissingFieldViolations(t *testing.T) {
	t.Parallel()

	l := NewCreateAdminLogic(context.Background(), &svc.ServiceContext{})

	resp, err := l.CreateAdmin(&pb.CreateAdminRequest{
		Username: "u@example.com",
		Name:     "User",
		Role:     "",
	})
	require.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	var br *errdetails.BadRequest
	for _, d := range st.Details() {
		if v, ok := d.(*errdetails.BadRequest); ok {
			br = v
			break
		}
	}
	require.NotNil(t, br)

	violations := map[string]string{}
	for _, fv := range br.FieldViolations {
		violations[fv.GetField()] = fv.GetDescription()
	}
	require.Equal(t, map[string]string{"role": "required"}, violations)
}
