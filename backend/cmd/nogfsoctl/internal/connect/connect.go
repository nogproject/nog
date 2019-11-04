package connect

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/jwtauth"
	"github.com/nogproject/nog/backend/internal/grpcjwt"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"github.com/nogproject/nog/backend/pkg/x509io"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var (
	clientAliveInterval      = 40 * time.Second
	clientAliveWithoutStream = true
)

type SimpleScope = auth.SimpleScope
type RepoIdScope = jwtauth.RepoIdScope

func DialX509(addr, certFile, caFile string) (*grpc.ClientConn, error) {
	cert, err := x509io.LoadCombinedCert(certFile)
	if err != nil {
		return nil, err
	}
	ca, err := x509io.LoadCABundle(caFile)
	if err != nil {
		return nil, err
	}
	return grpc.Dial(
		addr,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      ca,
		})),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                clientAliveInterval,
			PermitWithoutStream: clientAliveWithoutStream,
		}),
	)
}

func GetRPCCredsScopes(
	ctx context.Context,
	args map[string]interface{},
	scopes []interface{},
) (grpc.CallOption, error) {
	tok, err := grpcjwt.Load(args["--jwt"].(string))
	if err != nil {
		err := fmt.Errorf("failed to load --jwt: %s", err)
		return nil, err
	}
	rpcCreds := grpc.PerRPCCredentials(tok)

	authURL, ok := args["--jwt-auth"].(string)
	if !ok {
		return rpcCreds, nil
	}

	scTok, err := jwtauth.GetScopedJWT(authURL, tok.Token, scopes)
	if err != nil {
		err := fmt.Errorf(
			"failed to get token for scopes `%+v`: %s",
			scopes, err,
		)
		return nil, err
	}
	rpcCreds = grpc.PerRPCCredentials(scTok)

	return rpcCreds, nil
}

func GetRPCCredsScope(
	ctx context.Context,
	args map[string]interface{},
	sc auth.SimpleScope,
) (grpc.CallOption, error) {
	tok, err := grpcjwt.Load(args["--jwt"].(string))
	if err != nil {
		err := fmt.Errorf("failed to load --jwt: %s", err)
		return nil, err
	}
	rpcCreds := grpc.PerRPCCredentials(tok)

	authURL, ok := args["--jwt-auth"].(string)
	if !ok {
		return rpcCreds, nil
	}

	scopes := []interface{}{sc}
	scTok, err := jwtauth.GetScopedJWT(authURL, tok.Token, scopes)
	if err != nil {
		err := fmt.Errorf(
			"failed to get token for scope `%+v`: %s", sc, err,
		)
		return nil, err
	}
	rpcCreds = grpc.PerRPCCredentials(scTok)

	return rpcCreds, nil
}

func GetRPCCredsSimple(
	ctx context.Context,
	args map[string]interface{},
	scopes []SimpleScope,
) (grpc.CallOption, error) {
	tok, err := grpcjwt.Load(args["--jwt"].(string))
	if err != nil {
		err := fmt.Errorf("failed to load --jwt: %s", err)
		return nil, err
	}
	rpcCreds := grpc.PerRPCCredentials(tok)

	authURL, ok := args["--jwt-auth"].(string)
	if !ok {
		return rpcCreds, nil
	}

	scIfaces := make([]interface{}, 0, len(scopes))
	for _, sc := range scopes {
		scIfaces = append(scIfaces, sc)
	}
	scTok, err := jwtauth.GetScopedJWT(authURL, tok.Token, scIfaces)
	if err != nil {
		err := fmt.Errorf(
			"failed to get token for scopes `%+v`: %s",
			scopes, err,
		)
		return nil, err
	}
	rpcCreds = grpc.PerRPCCredentials(scTok)

	return rpcCreds, nil
}

func GetRPCCredsRepoId(
	ctx context.Context,
	args map[string]interface{},
	action auth.Action,
	repoId uuid.I,
) (grpc.CallOption, error) {
	tok, err := grpcjwt.Load(args["--jwt"].(string))
	if err != nil {
		err := fmt.Errorf("failed to load --jwt: %s", err)
		return nil, err
	}
	rpcCreds := grpc.PerRPCCredentials(tok)

	authURL, ok := args["--jwt-auth"].(string)
	if !ok {
		return rpcCreds, nil
	}

	scTok, err := jwtauth.GetScopedJWTRepoId(
		authURL, tok.Token, action, repoId,
	)
	if err != nil {
		return nil, err
	}

	rpcCreds = grpc.PerRPCCredentials(scTok)
	return rpcCreds, nil
}

func GetRPCCredsSimpleAndRepoId(
	ctx context.Context,
	args map[string]interface{},
	simpleScopes []SimpleScope,
	idScopes []RepoIdScope,
) (grpc.CallOption, error) {
	tok, err := grpcjwt.Load(args["--jwt"].(string))
	if err != nil {
		err := fmt.Errorf("failed to load --jwt: %s", err)
		return nil, err
	}
	rpcCreds := grpc.PerRPCCredentials(tok)

	authURL, ok := args["--jwt-auth"].(string)
	if !ok {
		return rpcCreds, nil
	}

	scTok, err := jwtauth.GetScopedJWTSimpleAndRepoId(
		authURL, tok.Token,
		simpleScopes, idScopes,
	)
	if err != nil {
		return nil, err
	}
	rpcCreds = grpc.PerRPCCredentials(scTok)

	return rpcCreds, nil
}
