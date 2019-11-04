// Package `maind`: GRPC service `nogfso.Main` to access the FSO main root
// entity.
package maind

import (
	"context"

	"github.com/nogproject/nog/backend/internal/fsomain"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	authn    auth.Authenticator
	authz    auth.Authorizer
	main     *fsomain.Main
	mainId   uuid.I
	mainName string
}

func New(
	ctx context.Context, // For future use to stop streaming RPCs.
	authn auth.Authenticator,
	authz auth.Authorizer,
	main *fsomain.Main,
	mainId uuid.I,
	mainName string,
) *Server {
	return &Server{
		authn:    authn,
		authz:    authz,
		main:     main,
		mainId:   mainId,
		mainName: mainName,
	}
}

func (srv *Server) GetRegistries(
	ctx context.Context, req *pb.GetRegistriesI,
) (*pb.GetRegistriesO, error) {
	err := srv.authName(ctx, AAFsoReadMain, srv.mainName)
	if err != nil {
		return nil, err
	}

	s, err := srv.main.FindId(srv.mainId)
	if err != nil {
		err = status.Errorf(codes.Unknown, "main error: %v", err)
		return nil, err
	}

	vid := s.Vid()
	rsp := pb.GetRegistriesO{
		Main: srv.mainName,
		Vid:  vid[:],
	}
	for _, r := range s.Registries() {
		rsp.Registries = append(rsp.Registries, &pb.RegistryMainInfo{
			Name:      r.Name,
			Confirmed: r.Confirmed,
		})
	}
	return &rsp, nil
}
