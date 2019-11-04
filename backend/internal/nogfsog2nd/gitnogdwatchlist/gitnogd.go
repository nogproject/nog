package gitnogdwatchlist

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/broadcast"
	"github.com/nogproject/nog/backend/internal/nogfsog2nd/gitlab"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
	"github.com/nogproject/nog/backend/pkg/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var enableGitlabDeadtime = 1 * time.Minute

type Config struct {
	Registries  []string
	Prefixes    []string
	Gitlabs     []*gitlab.Client
	Broadcaster *broadcast.Broadcaster
}

type Server struct {
	lg           Logger
	prefixes     []string
	gitlabs      map[string]*gitlab.Client
	store        *store
	registryView *registryView
	broadcaster  *broadcast.Broadcaster
}

type Logger interface {
	Errorw(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Infow(msg string, kv ...interface{})
}

func New(lg Logger, regConn *grpc.ClientConn, cfg *Config) *Server {
	var prefixes []string
	for _, p := range cfg.Prefixes {
		// Ensure trailing slash.
		p = strings.TrimRight(p, "/") + "/"
		prefixes = append(prefixes, p)
	}

	gls := make(map[string]*gitlab.Client)
	for _, gl := range cfg.Gitlabs {
		gls[gl.Hostname] = gl
	}

	return &Server{
		lg:           lg,
		prefixes:     prefixes,
		gitlabs:      gls,
		store:        newStore(lg, regConn),
		registryView: newRegistryView(lg, regConn, cfg),
		broadcaster:  cfg.Broadcaster,
	}
}

func (srv *Server) Watch(ctx context.Context) error {
	return srv.registryView.watch(ctx)
}

func (srv *Server) Head(ctx context.Context, i *pb.HeadI) (*pb.HeadO, error) {
	_, glc, projectId, err := srv.findGitlab(ctx, i.Repo)
	if err != nil {
		return nil, err
	}

	heads, details, err := getHeadsDetails(srv.lg, glc, projectId)
	if err != nil {
		return nil, err
	}

	headId, err := headsSha(heads)
	if err != nil {
		srv.lg.Errorw("headsSha() failed.", "err", err)
		return nil, err
	}

	o := *details
	o.Repo = i.Repo
	o.CommitId = headId
	o.GitCommits = heads
	return &o, nil
}

func (srv *Server) Summary(
	ctx context.Context, i *pb.SummaryI,
) (*pb.SummaryO, error) {
	_, glc, projectId, err := srv.findGitlab(ctx, i.Repo)
	if err != nil {
		return nil, err
	}

	heads, err := getHeads(srv.lg, glc, projectId)
	if err != nil {
		return nil, err
	}

	headId, err := headsSha(heads)
	if err != nil {
		srv.lg.Errorw("headsSha() failed.", "err", err)
		return nil, err
	}

	lst, err := glc.ListTreeAll(
		projectId, "refs/heads/master-stat",
	)
	var nFiles int64
	var nDirs int64
	var nOther int64
	for _, e := range lst {
		if isHiddenName(e.Name) {
			continue
		}
		switch e.Type {
		case "blob":
			nFiles++
		case "tree":
			nDirs++
		default:
			nOther++
		}
	}

	return &pb.SummaryO{
		Repo:     i.Repo,
		CommitId: headId,
		NumFiles: nFiles,
		NumDirs:  nDirs,
		NumOther: nOther,
	}, nil
}

func (srv *Server) Meta(ctx context.Context, i *pb.MetaI) (*pb.MetaO, error) {
	_, glc, projectId, err := srv.findGitlab(ctx, i.Repo)
	if err != nil {
		return nil, err
	}

	heads, err := getHeads(srv.lg, glc, projectId)
	if err != nil {
		return nil, err
	}

	headId, err := headsSha(heads)
	if err != nil {
		srv.lg.Errorw("headsSha() failed.", "err", err)
		return nil, err
	}

	meta := make(map[string]interface{})
	if heads.Meta != nil {
		ok, err := glc.Meta(
			projectId,
			fmt.Sprintf("%x", heads.Meta),
			".nogtree",
			&meta,
		)
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to read meta `.nogtree`: %v", err,
			)
			return nil, err
		}
		// `ok==false` indicates that `.nogtree` was not found.  Ignore
		// it, and return empty meta.
		_ = ok
	}

	buf, err := json.Marshal(meta)
	if err != nil {
		err := status.Errorf(
			codes.Internal, "failed to encode JSON: %v", err,
		)
		return nil, err
	}

	return &pb.MetaO{
		Repo:     i.Repo,
		CommitId: headId,
		MetaJson: buf,
	}, nil
}

func (srv *Server) PutMeta(
	ctx context.Context, i *pb.PutMetaI,
) (*pb.PutMetaO, error) {
	repoId, glc, projectId, err := srv.findGitlab(ctx, i.Repo)
	if err != nil {
		return nil, err
	}

	commitHeader := gitlab.CommitHeader{
		AuthorName:    i.AuthorName,
		AuthorEmail:   i.AuthorEmail,
		CommitMessage: i.CommitMessage,
	}

	want, err := recodeMeta(i.MetaJson)
	if err != nil {
		return nil, err
	}

	heads, err := getHeads(srv.lg, glc, projectId)
	if err != nil {
		return nil, err
	}

	if i.OldCommitId != nil {
		actual, err := headsSha(heads)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(i.OldCommitId, actual) {
			err := status.Error(
				codes.FailedPrecondition,
				"old commit mismatch",
			)
			return nil, err
		}
	}

	createBranch := func() error {
		err := glc.CreateBranch(
			projectId, "master-meta", "refs/heads/master-stub",
		)
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to create branch `master-meta`: %v",
				err,
			)
			return err
		}
		return nil
	}

	createFile := func() error {
		commitId, err := glc.CreateFile(
			projectId, "master-meta", ".nogtree", want,
			commitHeader,
		)
		if err == nil {
			heads.Meta, err = hex.DecodeString(commitId)
		}
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to create file: %v", err,
			)
			return err
		}
		return nil
	}

	updateFile := func() error {
		lastCommit := fmt.Sprintf("%x", heads.Meta)
		commitId, err := glc.UpdateFile(
			projectId, "master-meta", lastCommit, ".nogtree",
			want, commitHeader,
		)
		if err == nil {
			heads.Meta, err = hex.DecodeString(commitId)
		}
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to update file: %v", err,
			)
			return err
		}
		return nil
	}

	var isNewCommit bool
	if heads.Meta == nil {
		if err := createBranch(); err != nil {
			return nil, err
		}
		if err := createFile(); err != nil {
			return nil, err
		}
		isNewCommit = true
	} else {
		got, err := glc.GetFileContent(
			projectId, fmt.Sprintf("%x", heads.Meta), ".nogtree",
		)
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to get previous meta from GitLab: %v",
				err,
			)
			return nil, err
		}
		if !bytes.Equal(want, got) {
			if got == nil {
				if err := createFile(); err != nil {
					return nil, err
				}
			} else {
				if err := updateFile(); err != nil {
					return nil, err
				}
			}
			isNewCommit = true
		}
	}

	headId, err := headsSha(heads)
	if err != nil {
		srv.lg.Errorw("headsSha() failed.", "err", err)
		return nil, err
	}

	err = srv.broadcaster.PostGitMetaUpdated(ctx, repoId, heads.Meta)
	if err != nil {
		srv.lg.Warnw("Failed to broadcast.", "err", err)
	}

	return &pb.PutMetaO{
		Repo:         i.Repo,
		GitNogCommit: headId,
		IsNewCommit:  isNewCommit,
	}, nil
}

func (srv *Server) Content(
	ctx context.Context, i *pb.ContentI,
) (*pb.ContentO, error) {
	_, glc, projectId, err := srv.findGitlab(ctx, i.Repo)
	if err != nil {
		return nil, err
	}

	heads, err := getHeads(srv.lg, glc, projectId)
	if err != nil {
		return nil, err
	}

	headId, err := headsSha(heads)
	if err != nil {
		srv.lg.Errorw("headsSha() failed.", "err", err)
		return nil, err
	}

	var blob []byte
	if heads.Content != nil {
		c, err := glc.GetFileContent(
			projectId,
			fmt.Sprintf("%x", heads.Content),
			i.Path,
		)
		if err != nil {
			err := status.Errorf(
				codes.Unknown,
				"failed to read content: %v", err,
			)
			return nil, err
		}
		if c == nil {
			err := status.Errorf(codes.NotFound, "unknown path")
			return nil, err
		}
		blob = c
	}

	return &pb.ContentO{
		Repo:     i.Repo,
		CommitId: headId,
		Content:  blob,
	}, nil
}

func isHiddenName(n string) bool {
	return strings.HasPrefix(n, ".git") || strings.HasPrefix(n, ".nog")
}

func (srv *Server) findGitlab(
	ctx context.Context, repoIdBytes []byte,
) (uuid.I, *gitlab.Client, int, error) {
	repoId, err := uuid.FromBytes(repoIdBytes)
	if err != nil {
		err = status.Errorf(
			codes.InvalidArgument, "invalid id: %v", err,
		)
		return uuid.Nil, nil, 0, err
	}

	if !srv.registryView.isKnownRepo(repoId) {
		err := status.Errorf(codes.NotFound, "unknown repo")
		return uuid.Nil, nil, 0, err
	}

	repo, err := srv.store.repo(ctx, repoId)
	if err != nil {
		return uuid.Nil, nil, 0, err
	}

	if !isBelowPrefix(srv.prefixes, repo.globalPath) {
		err := status.Errorf(
			codes.InvalidArgument, "unknown globalPath prefix",
		)
		return uuid.Nil, nil, 0, err
	}

	if repo.gitlabProjectId == 0 {
		err := status.Errorf(
			codes.FailedPrecondition,
			"repo has no GitLab project id",
		)
		return uuid.Nil, nil, 0, err
	}
	projectId := repo.gitlabProjectId

	glc, ok := srv.gitlabs[repo.gitlabHost]
	if !ok {
		err := status.Errorf(codes.Internal, "missing GitLab client")
		srv.lg.Errorw(
			"logic error: missing gitlab client for repo",
			"repoId", repoId.String(),
		)
		return uuid.Nil, nil, 0, err
	}

	cutoff := time.Now().Add(-enableGitlabDeadtime)
	if repo.gitlabEnabledSince.After(cutoff) {
		err := status.Errorf(
			codes.FailedPrecondition,
			"GitLab enabled less than %s ago",
			enableGitlabDeadtime,
		)
		return uuid.Nil, nil, 0, err
	}

	return repoId, glc, projectId, nil
}

func getHeads(
	lg Logger, glc *gitlab.Client, projectId int,
) (*pb.HeadGitCommits, error) {
	getBranch := func(branch string) ([]byte, error) {
		br, err := glc.GetBranch(projectId, branch)
		if err != nil {
			err := status.Errorf(
				codes.Unknown, "failed to get branch: %v", err,
			)
			return nil, err
		}
		if br == nil {
			return nil, nil
		}
		head, err := hex.DecodeString(br.Commit.ID)
		if err != nil {
			msg := "failed to decode Git ID"
			err := status.Errorf(codes.Internal, msg)
			lg.Errorw(msg, "projectId", projectId)
			return nil, err
		}
		return head, nil
	}

	var err error
	heads := pb.HeadGitCommits{}
	if heads.Stat, err = getBranch("master-stat"); err != nil {
		return nil, err
	}
	if heads.Sha, err = getBranch("master-sha"); err != nil {
		return nil, err
	}
	if heads.Content, err = getBranch("master-content"); err != nil {
		return nil, err
	}
	if heads.Meta, err = getBranch("master-meta"); err != nil {
		return nil, err
	}
	return &heads, nil
}

func getHeadsDetails(
	lg Logger, glc *gitlab.Client, projectId int,
) (*pb.HeadGitCommits, *pb.HeadO, error) {
	var head []byte
	var author *pb.WhoDate
	var committer *pb.WhoDate
	getBranch := func(branch string) error {
		br, err := glc.GetBranch(projectId, branch)
		if err != nil {
			err := status.Errorf(
				codes.Unknown, "failed to get branch: %v", err,
			)
			return err
		}
		if br == nil {
			head = nil
			author = nil
			committer = nil
			return nil
		}

		head, err = hex.DecodeString(br.Commit.ID)
		if err != nil {
			msg := "failed to decode Git ID"
			err := status.Errorf(codes.Internal, msg)
			lg.Errorw(msg, "projectId", projectId)
			return err
		}

		c := br.Commit
		author = &pb.WhoDate{
			Name:  c.AuthorName,
			Email: c.AuthorEmail,
			Date:  c.AuthoredDate.Format(time.RFC3339),
		}
		committer = &pb.WhoDate{
			Name:  c.CommitterName,
			Email: c.CommitterEmail,
			Date:  c.CommittedDate.Format(time.RFC3339),
		}
		return nil
	}

	heads := pb.HeadGitCommits{}
	details := pb.HeadO{}

	if err := getBranch("master-stat"); err != nil {
		return nil, nil, err
	}
	if head == nil {
		err := status.Errorf(
			codes.Unknown, "unknown branch `master-stat`",
		)
		return nil, nil, err
	}
	heads.Stat = head
	details.StatAuthor = author
	details.StatCommitter = committer

	if err := getBranch("master-sha"); err != nil {
		return nil, nil, err
	}
	heads.Sha = head
	details.ShaAuthor = author
	details.ShaCommitter = committer

	if err := getBranch("master-meta"); err != nil {
		return nil, nil, err
	}
	heads.Meta = head
	details.MetaAuthor = author
	details.MetaCommitter = committer

	if err := getBranch("master-content"); err != nil {
		return nil, nil, err
	}
	heads.Content = head
	details.ContentAuthor = author
	details.ContentCommitter = committer

	return &heads, &details, nil
}

func headsSha(heads *pb.HeadGitCommits) ([]byte, error) {
	buf, err := proto.Marshal(heads)
	if err != nil {
		msg := "proto encoding heads failed"
		err := status.Errorf(codes.Internal, msg)
		return nil, err
	}
	sha := sha512.Sum512_256(buf)
	return sha[:], nil
}

// `recodeMeta()` expects an input JSON object with string keys and returns it
// as YAML with sorted keys, one per line like `<key>: <json-val>`.  Example
// line: `note: "foo"`.
func recodeMeta(in []byte) ([]byte, error) {
	meta := make(map[string]interface{})
	if err := json.Unmarshal(in, &meta); err != nil {
		err := status.Errorf(
			codes.InvalidArgument,
			"failed to decode meta JSON: %v", err,
		)
		return nil, err
	}

	var out bytes.Buffer
	jout := json.NewEncoder(&out)
	jout.SetEscapeHTML(false)

	keys := make([]string, 0)
	for k, _ := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&out, "%s: ", k)
		if err := jout.Encode(meta[k]); err != nil {
			err := status.Errorf(
				codes.InvalidArgument,
				"failed to encode meta: %v", err,
			)
			return nil, err
		}
	}

	return out.Bytes(), nil
}

func isBelowPrefix(prefixes []string, path string) bool {
	for _, pfx := range prefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}
