package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	pb "github.com/nogproject/nog/backend/internal/unixdomainspb"
	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/ulid"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

func cmdUnixDomain(args map[string]interface{}) {
	conn, err := connect.DialX509(
		args["--nogfsoregd"].(string),
		args["--tls-cert"].(string),
		args["--tls-ca"].(string),
	)
	if err != nil {
		lg.Fatalw("Failed to dial nogfsoregd.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw("Failed to close conn.", "err", err)
		}
	}()

	switch {
	case args["init"].(bool) && args["unix-domain"].(bool):
		cmdUnixDomainInit(args, conn)
	case args["get"].(bool) && args["unix-domain"].(bool):
		cmdUnixDomainGet(args, conn)
	case args["unix-domain"].(bool) && args["create-group"].(bool):
		cmdUnixDomainCreateGroup(args, conn)
	case args["unix-domain"].(bool) && args["delete-group"].(bool):
		cmdUnixDomainDeleteGroup(args, conn)
	case args["unix-domain"].(bool) && args["create-user"].(bool):
		cmdUnixDomainCreateUser(args, conn)
	case args["unix-domain"].(bool) && args["delete-user"].(bool):
		cmdUnixDomainDeleteUser(args, conn)
	case args["unix-domain"].(bool) && args["add-group-user"].(bool):
		cmdUnixDomainAddGroupUser(args, conn)
	case args["unix-domain"].(bool) && args["remove-group-user"].(bool):
		cmdUnixDomainRemoveGroupUser(args, conn)
	default:
		lg.Fatalw("Logic error: invalid `unix-domain` sub-command.")
	}
}

func cmdUnixDomainInit(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	i := &pb.CreateUnixDomainI{
		DomainName: args["<domain>"].(string),
	}
	if args["--no-vid"].(bool) {
		i.MainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.MainVid = vid[:]
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAInitUnixDomain,
		Name:   i.DomainName,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.CreateUnixDomain(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnUuidBytes("domainId", o.DomainId)
	mustPrintlnVidBytes("domainVid", o.DomainVid)
	mustPrintlnVidBytes("mainVid", o.MainVid)
}

func cmdUnixDomainGet(args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	i := &pb.GetUnixDomainI{
		DomainName: args["<domain>"].(string),
	}
	creds, err := getRPCCredsScope(ctx, args, auth.SimpleScope{
		Action: AAReadUnixDomain,
		Name:   i.DomainName,
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.GetUnixDomain(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}

	mustPrintlnUuidBytes("domainId", o.DomainId)
	mustPrintlnVidBytes("domainVid", o.DomainVid)
	fmt.Printf("domainName: %s\n", o.DomainName)

	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	if len(o.Users) == 0 {
		fmt.Println("users: []")
	} else {
		fmt.Println("users:")
		for _, u := range o.Users {
			os.Stdout.Write([]byte("- "))
			if err := jout.Encode(&u); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
	}

	if len(o.Groups) == 0 {
		fmt.Println("groups: []")
	} else {
		fmt.Println("groups:")
		for _, g := range o.Groups {
			os.Stdout.Write([]byte("- "))
			if err := jout.Encode(&g); err != nil {
				lg.Fatalw("JSON marshal failed.", "err", err)
			}
		}
	}
}

func resolveUnixDomain(
	ctx context.Context,
	c pb.UnixDomainsClient,
	domainName string,
	creds grpc.CallOption,
) uuid.I {
	i := &pb.GetUnixDomainI{
		DomainName: domainName,
	}
	o, err := c.GetUnixDomain(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	id, err := uuid.FromBytes(o.DomainId)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	return id
}

func cmdUnixDomainCreateGroup(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.CreateUnixGroupI{
		DomainId: domainId[:],
		Name:     args["<group>"].(string),
		Gid:      uint32(args["<gid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.CreateUnixGroup(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}

func cmdUnixDomainDeleteGroup(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.DeleteUnixGroupI{
		DomainId: domainId[:],
		Gid:      uint32(args["<gid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.DeleteUnixGroup(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}

func cmdUnixDomainCreateUser(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.CreateUnixUserI{
		DomainId: domainId[:],
		Name:     args["<user>"].(string),
		Uid:      uint32(args["<uid>"].(int32)),
		Gid:      uint32(args["<gid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.CreateUnixUser(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}

func cmdUnixDomainDeleteUser(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.DeleteUnixUserI{
		DomainId: domainId[:],
		Uid:      uint32(args["<uid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.DeleteUnixUser(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}

func cmdUnixDomainAddGroupUser(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.AddUnixGroupUserI{
		DomainId: domainId[:],
		Gid:      uint32(args["<gid>"].(int32)),
		Uid:      uint32(args["<uid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.AddUnixGroupUser(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}

func cmdUnixDomainRemoveGroupUser(
	args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c := pb.NewUnixDomainsClient(conn)
	domainName := args["<domain>"].(string)
	creds, err := getRPCCredsScopes(ctx, args, []interface{}{
		auth.SimpleScope{Action: AAReadUnixDomain, Name: domainName},
		auth.SimpleScope{Action: AAWriteUnixDomain, Name: domainName},
	})
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}

	domainId := resolveUnixDomain(ctx, c, domainName, creds)
	i := &pb.RemoveUnixGroupUserI{
		DomainId: domainId[:],
		Gid:      uint32(args["<gid>"].(int32)),
		Uid:      uint32(args["<uid>"].(int32)),
	}
	if args["--no-vid"].(bool) {
		i.DomainVid = nil
	} else {
		vid := args["--vid"].(ulid.I)
		i.DomainVid = vid[:]
	}
	o, err := c.RemoveUnixGroupUser(ctx, i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	mustPrintlnVidBytes("domainVid", o.DomainVid)
}
