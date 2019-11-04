package cmdgitnog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/connect"
	"github.com/nogproject/nog/backend/cmd/nogfsoctl/internal/parse"
	"github.com/nogproject/nog/backend/internal/fsoauthz"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
)

const AAFsoReadRepo = fsoauthz.AAFsoReadRepo
const AAFsoWriteRepo = fsoauthz.AAFsoWriteRepo

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

func Cmd(lg Logger, args map[string]interface{}) {
	var addr string
	switch {
	case args["--regd"].(bool):
		addr = args["--nogfsoregd"].(string)
	case args["--g2nd"].(bool):
		lg.Fatalw("--g2nd unsupported.")
	default:
		panic("missing either --regd or --g2nd")
	}
	conn, err := connect.DialX509(
		addr,
		args["--tls-cert"].(string),
		args["--tls-ca"].(string),
	)
	if err != nil {
		lg.Fatalw("Failed to dial nogfsog2nd.", "err", err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			lg.Errorw("Failed to close conn.", "err", err)
		}
	}()

	switch {
	case args["head"].(bool):
		cmdHead(lg, args, conn)
	case args["summary"].(bool):
		cmdSummary(lg, args, conn)
	case args["meta"].(bool):
		cmdMeta(lg, args, conn)
	case args["putmeta"].(bool):
		cmdPutMeta(lg, args, conn)
	case args["content"].(bool):
		cmdContent(lg, args, conn)
	}
}

const nullSha = "0000000000000000000000000000000000000000"

func cmdHead(lg Logger, args map[string]interface{}, conn *grpc.ClientConn) {
	ctx := context.Background()
	c := pb.NewGitNogClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.HeadI{
		Repo: uuI[:],
	}
	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.Head(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	uuO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}

	fmt.Printf(`repo: "%s"`+"\n", uuO)
	fmt.Printf(`gitNogCommit: "%x"`+"\n", o.CommitId)

	for _, f := range []struct {
		n      string
		commit []byte
	}{
		{"statGitCommit", o.GitCommits.Stat},
		{"shaGitCommit", o.GitCommits.Sha},
		{"metaGitCommit", o.GitCommits.Meta},
		{"contentGitCommit", o.GitCommits.Content},
	} {
		fmt.Printf("%s: ", f.n)
		if f.commit == nil {
			fmt.Printf(`"%s"`+"\n", nullSha)
		} else {
			fmt.Printf(`"%x"`+"\n", f.commit)
		}
	}

	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	fields := []struct {
		n  string
		wd *pb.WhoDate
	}{
		{"statAuthor", o.StatAuthor},
		{"statCommitter", o.StatCommitter},
		{"shaAuthor", o.ShaAuthor},
		{"shaCommitter", o.ShaCommitter},
		{"metaAuthor", o.MetaAuthor},
		{"metaCommitter", o.MetaCommitter},
		{"contentAuthor", o.ContentAuthor},
		{"contentCommitter", o.ContentCommitter},
	}
	for _, f := range fields {
		fmt.Printf("%s: ", f.n)
		if err := jout.Encode(f.wd); err != nil {
			lg.Fatalw("Failed to encode JSON.", "err", err)
		}
	}
}

func cmdSummary(
	lg Logger, args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	c := pb.NewGitNogClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.SummaryI{
		Repo: uuI[:],
	}
	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.Summary(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	uuO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}

	fmt.Printf(`repo: "%s"`+"\n", uuO)
	fmt.Printf(`gitNogCommit: "%x"`+"\n", o.CommitId)
	fmt.Printf("numFiles: %d\n", o.NumFiles)
	fmt.Printf("numDirs: %d\n", o.NumDirs)
	fmt.Printf("numOther: %d\n", o.NumOther)
}

func cmdMeta(
	lg Logger, args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	c := pb.NewGitNogClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.MetaI{
		Repo: uuI[:],
	}
	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.Meta(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	uuO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}
	var meta interface{}
	if err := json.Unmarshal(o.MetaJson, &meta); err != nil {
		lg.Fatalw("Failed to decode meta JSON.", "err", err)
	}

	fmt.Printf(`repo: "%s"`+"\n", uuO)
	fmt.Printf(`gitNogCommit: "%x"`+"\n", o.CommitId)
	fmt.Printf("meta: ")
	jout := json.NewEncoder(os.Stdout)
	jout.SetEscapeHTML(false)
	jout.SetIndent("", "  ")
	if err := jout.Encode(&meta); err != nil {
		lg.Fatalw("Failed to encode JSON.", "err", err)
	}
}

func cmdPutMeta(
	lg Logger, args map[string]interface{}, conn *grpc.ClientConn,
) {
	name, email, err := parse.User(args["--author"].(string))
	if err != nil {
		lg.Fatalw("Invalid author.", "err", err)
	}

	meta := make(map[string]string)
	for _, kv := range (args["<kvs>"]).([][2]string) {
		meta[kv[0]] = kv[1]
	}
	metaJson, err := json.Marshal(meta)
	if err != nil {
		lg.Fatalw("Failed to encode meta JSON.", "err", err)
	}

	ctx := context.Background()
	c := pb.NewGitNogClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.PutMetaI{
		Repo:          uuI[:],
		AuthorName:    name,
		AuthorEmail:   email,
		CommitMessage: args["--message"].(string),
		MetaJson:      metaJson,
	}
	if cId, ok := args["--old-commit"].([]byte); ok {
		i.OldCommitId = cId
	}

	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoWriteRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.PutMeta(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	uuO, err := uuid.FromBytes(o.Repo)
	if err != nil {
		lg.Fatalw("Invalid UUID.", "err", err)
	}

	fmt.Printf(`repo: "%s"`+"\n", uuO)
	fmt.Printf(`gitNogCommit: "%x"`+"\n", o.GitNogCommit)
}

func cmdContent(
	lg Logger, args map[string]interface{}, conn *grpc.ClientConn,
) {
	ctx := context.Background()
	c := pb.NewGitNogClient(conn)
	uuI := args["<repoid>"].(uuid.I)
	i := pb.ContentI{
		Repo: uuI[:],
		Path: args["<path>"].(string),
	}
	creds, err := connect.GetRPCCredsRepoId(ctx, args, AAFsoReadRepo, uuI)
	if err != nil {
		lg.Fatalw("Failed to get auth token.", "err", err)
	}
	o, err := c.Content(ctx, &i, creds)
	if err != nil {
		lg.Fatalw("RPC failed.", "err", err)
	}
	os.Stdout.Write(o.Content)
}
