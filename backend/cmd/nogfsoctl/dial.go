package main

import (
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/status"
)

const AAFsoAdminRegistry = fsoauthz.AAFsoAdminRegistry
const AAFsoAdminRoot = fsoauthz.AAFsoAdminRoot
const AAFsoAdminRepo = fsoauthz.AAFsoAdminRepo
const AAFsoDeleteRoot = fsoauthz.AAFsoDeleteRoot
const AAFsoEnableDiscoveryPath = fsoauthz.AAFsoEnableDiscoveryPath
const AAFsoFind = fsoauthz.AAFsoFind
const AAFsoInitRegistry = fsoauthz.AAFsoInitRegistry
const AAFsoInitRepo = fsoauthz.AAFsoInitRepo
const AAFsoInitRoot = fsoauthz.AAFsoInitRoot
const AAFsoReadRegistry = fsoauthz.AAFsoReadRegistry
const AAFsoReadRoot = fsoauthz.AAFsoReadRoot
const AAFsoReadRepo = fsoauthz.AAFsoReadRepo
const AAFsoRefreshRepo = fsoauthz.AAFsoRefreshRepo
const AAFsoWriteRepo = fsoauthz.AAFsoWriteRepo
const AAFsoTestUdo = fsoauthz.AAFsoTestUdo
const AAFsoTestUdoAs = fsoauthz.AAFsoTestUdoAs
const AAFsoFreezeRepo = fsoauthz.AAFsoFreezeRepo
const AAFsoUnfreezeRepo = fsoauthz.AAFsoUnfreezeRepo
const AAFsoArchiveRepo = fsoauthz.AAFsoArchiveRepo
const AAFsoUnarchiveRepo = fsoauthz.AAFsoUnarchiveRepo

const AAInitUnixDomain = fsoauthz.AAInitUnixDomain
const AAReadUnixDomain = fsoauthz.AAReadUnixDomain
const AAWriteUnixDomain = fsoauthz.AAWriteUnixDomain

// funcs
var dialX509 = connect.DialX509
var getRPCCredsRepoId = connect.GetRPCCredsRepoId
var getRPCCredsScope = connect.GetRPCCredsScope
var getRPCCredsScopes = connect.GetRPCCredsScopes
var getRPCCredsSimple = connect.GetRPCCredsSimple

func authScopeFromErr(err error) (*auth.SimpleScope, bool) {
	st, ok := status.FromError(err)
	if !ok {
		return nil, false
	}

	d := st.Details()
	if len(d) < 1 {
		return nil, false
	}

	sc, ok := d[0].(*pb.AuthRequiredScope)
	if !ok {
		return nil, false
	}

	return &auth.SimpleScope{
		Action: sc.Action,
		Name:   sc.Name,
		Path:   sc.Path,
	}, true
}
