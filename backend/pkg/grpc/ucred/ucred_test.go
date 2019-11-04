package ucred_test

import (
	"context"
	"syscall"
	"testing"

	"github.com/nogproject/nog/backend/pkg/grpc/ucred"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestUidAuthorizerMissingUcred(t *testing.T) {
	a := ucred.NewUidAuthorizer(0)
	err := a.Authorize(context.Background())
	if err == nil {
		t.Error("did not reject missing creds")
	}
	if err != ucred.ErrMissingUcred {
		t.Error("wrong error")
	}
}

func TestUidAuthorizerUids(t *testing.T) {
	a := ucred.NewUidAuthorizer(0, 10)

	cases := []struct {
		uid          uint32
		expectAccept bool
	}{
		{0, true},
		{10, true},
		{1, false},
	}

	for _, c := range cases {
		info := ucred.AuthInfo{
			Ucred: syscall.Ucred{Uid: c.uid},
		}
		err := a.AuthorizeInfo(&info)
		if c.expectAccept && err != nil {
			t.Errorf("did not authorize %d", c.uid)
		}
		if !c.expectAccept {
			if err == nil {
				t.Errorf("did not deny %d", c.uid)
			}
			stat, ok := status.FromError(err)
			if !ok {
				t.Error("wrong error type")
			}
			if stat.Code() != codes.PermissionDenied {
				t.Error("wrong error code")
			}
		}

		ctx := peer.NewContext(
			context.Background(),
			&peer.Peer{AuthInfo: info},
		)
		err = a.Authorize(ctx)
		if c.expectAccept && err != nil {
			t.Errorf("did not authorize %d", c.uid)
		}
		if !c.expectAccept {
			if err == nil {
				t.Errorf("did not deny %d", c.uid)
			}
			stat, ok := status.FromError(err)
			if !ok {
				t.Error("wrong error type")
			}
			if stat.Code() != codes.PermissionDenied {
				t.Error("wrong error code")
			}
		}
	}
}
