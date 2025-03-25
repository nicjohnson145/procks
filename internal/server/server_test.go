package server

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/nicjohnson145/hlp"
	pbv1 "github.com/nicjohnson145/procks/gen/procks/v1"
	"github.com/stretchr/testify/require"
)

func TestValidateConnect(t *testing.T) {
	t.Parallel()

	valid := func() *pbv1.ConnectRequest {
		return &pbv1.ConnectRequest{}
	}

	testData := []struct {
		name    string
		modFunc func(x *pbv1.ConnectRequest)
		err     string
	}{
		{
			name:    "valid empty",
			modFunc: func(x *pbv1.ConnectRequest) {},
			err:     "",
		},
		{
			name: "valid with id",
			modFunc: func(x *pbv1.ConnectRequest) {
				x.Id = hlp.Ptr("abcdef")
			},
			err: "",
		},
		{
			name: "invalid too long",
			modFunc: func(x *pbv1.ConnectRequest) {
				x.Id = hlp.Ptr(strings.Join(hlp.Fill(27, "A"), ""))
			},
			err: "id must be alphanumeric and a max of 26 characters",
		},
		{
			name: "invalid non-alphanumeric",
			modFunc: func(x *pbv1.ConnectRequest) {
				x.Id = hlp.Ptr("abac-poafd0///+")
			},
			err: "id must be alphanumeric and a max of 26 characters",
		},
	}
	for _, tc := range testData {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			validObj := valid()
			tc.modFunc(validObj)

			srv := NewServer(ServerConfig{})
			err := srv.validateConnect(connect.NewRequest(validObj))

			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}
