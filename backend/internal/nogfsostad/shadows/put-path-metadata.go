package shadows

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"time"

	git "github.com/libgit2/git2go"
	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func (fs *Filesystem) PutPathMetadata(
	ctx context.Context, shadowPath string, i *pb.PutPathMetadataI,
) (*pb.PutPathMetadataO, error) {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return nil, err
	}

	// XXX `gitHeads()` should be reimplemented using git2go.
	heads, err := fs.gitHeads(ctx, shadowPath)
	if err != nil {
		return nil, err
	}

	if i.OldGitNogCommit != nil {
		actual, err := headsSha(heads)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(i.OldGitNogCommit, actual) {
			err := errors.New("old Git Nog commit mismatch")
			return nil, err
		}
	}

	if i.OldMetaGitCommit != nil {
		if !bytes.Equal(i.OldMetaGitCommit, heads.Meta) {
			err := errors.New("old meta Git commit mismatch")
			return nil, err
		}
	}

	// XXX Should be reimplemented using git2go.
	createBranch := func() error {
		head, err := fs.gitRevParseBytes(
			ctx, shadowPath, "refs/heads/master-stub",
		)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(
			ctx,
			fs.tools.git.Path, "branch",
			"master-meta", hex.EncodeToString(head),
		)
		cmd.Dir = shadowPath
		cmd.Env = fs.gitEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			err := fmt.Errorf(
				"git branch failed: %s; output: %s", err, out,
			)
			return err
		}

		heads.Meta = head
		return nil
	}

	var isNewCommit bool
	if heads.Meta == nil {
		if err := createBranch(); err != nil {
			return nil, err
		}
		isNewCommit = true
	}

	repo, err := git.OpenRepository(shadowPath)
	if err != nil {
		return nil, err
	}
	odb, err := repo.Odb()
	if err != nil {
		return nil, err
	}

	oldCommit, err := repo.LookupCommit(git.NewOidFromBytes(heads.Meta))
	if err != nil {
		return nil, err
	}

	oldTree, err := oldCommit.Tree()
	if err != nil {
		return nil, err
	}

	index, err := git.NewIndex()
	if err != nil {
		return nil, err
	}
	defer index.Free()

	if err := index.ReadTree(oldTree); err != nil {
		return nil, err
	}

	for _, pmd := range i.PathMetadata {
		path := pmd.Path
		if path == "" {
			return nil, errors.New("invalid path")
		}
		if path == "." {
			path = ".nogtree"
		} else if path[len(path)-1] == '/' {
			path += ".nogtree"
		}

		yaml, err := recodeMeta(pmd.MetadataJson)
		if err != nil {
			return nil, err
		}

		if len(yaml) == 0 {
			if err := index.RemoveByPath(path); err != nil {
				return nil, err
			}
			continue
		}

		fp, err := odb.NewWriteStream(int64(len(yaml)), git.ObjectBlob)
		if err != nil {
			return nil, err
		}
		n, err := fp.Write(yaml)
		if err != nil {
			return nil, err
		}
		if n != len(yaml) {
			return nil, errors.New("failed to write metadata blob")
		}
		if err := fp.Close(); err != nil {
			return nil, errors.New("failed to write metadata blob")
		}
		yamlId := &fp.Id

		if err := index.Add(&git.IndexEntry{
			Mode: git.FilemodeBlob,
			Size: uint32(len(yaml)),
			Id:   yamlId,
			Path: path,
		}); err != nil {
			return nil, err
		}
	}

	newTreeId, err := index.WriteTreeTo(repo)
	if err != nil {
		return nil, err
	}

	commitWho := func(sig *git.Signature) *pb.WhoDate {
		return &pb.WhoDate{
			Name:  sig.Name,
			Email: sig.Email,
			Date:  sig.When.Format(time.RFC3339),
		}
	}

	var commitAuthor *pb.WhoDate
	var commitCommitter *pb.WhoDate
	if !oldTree.Id().Equal(newTreeId) {
		newTree, err := repo.LookupTree(newTreeId)
		if err != nil {
			return nil, err
		}

		now := time.Now().Truncate(time.Second)
		author := git.Signature{
			Name:  i.AuthorName,
			Email: i.AuthorEmail,
			When:  now,
		}
		committer := git.Signature{
			Name:  "nogfsostad",
			Email: "nogfsostad@sys.nogproject.io",
			When:  now,
		}
		newCommitId, err := repo.CreateCommit(
			"refs/heads/master-meta",
			&author, &committer, i.CommitMessage,
			newTree, oldCommit,
		)
		if err != nil {
			return nil, err
		}

		commitAuthor = commitWho(&author)
		commitCommitter = commitWho(&committer)

		heads.Meta = newCommitId[:]
		isNewCommit = true
	} else {
		commitAuthor = commitWho(oldCommit.Author())
		commitCommitter = commitWho(oldCommit.Committer())
	}

	o := &pb.PutPathMetadataO{
		IsNewCommit:   isNewCommit,
		GitCommits:    heads,
		MetaAuthor:    commitAuthor,
		MetaCommitter: commitCommitter,
	}
	o.GitNogCommit, err = headsSha(heads)
	if err != nil {
		err := fmt.Errorf("headsSha() failed: %s", err)
		return nil, err
	}

	return o, nil
}
