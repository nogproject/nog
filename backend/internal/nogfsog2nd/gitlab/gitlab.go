package gitlab

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/nogproject/nog/backend/pkg/gitlab"
	yaml "gopkg.in/yaml.v2"
)

type Commit = gitlab.Commit

type Config struct {
	Hostname  string
	BaseUrl   string
	TokenPath string
}

type Client struct {
	Hostname string
	client   *gitlab.Client
}

type CommitHeader struct {
	AuthorName    string
	AuthorEmail   string
	CommitMessage string
}

func New(cfg Config) (*Client, error) {
	client, err := gitlab.NewClient(cfg.BaseUrl, cfg.TokenPath)
	if err != nil {
		return nil, err
	}

	return &Client{
		Hostname: cfg.Hostname,
		client:   client,
	}, nil
}

func (c *Client) GetBranch(
	projectId int, branch string,
) (*gitlab.Branch, error) {
	br, http, err := c.client.Branches.GetBranch(projectId, branch)
	if http != nil && http.StatusCode == 404 {
		// Return not found as nil branch w/o error.
		return nil, nil
	}
	if err == nil && http.StatusCode != 200 {
		err = fmt.Errorf("HTTP status is not 200 OK")
	}
	if err != nil {
		return nil, err
	}
	return br, err
}

func (c *Client) CreateBranch(
	projectId int, branch, ref string,
) error {
	br, http, err := c.client.Branches.CreateBranch(
		projectId,
		&gitlab.CreateBranchOptions{
			Branch: gitlab.String(branch),
			Ref:    gitlab.String(ref),
		},
	)
	if err == nil && http.StatusCode != 201 {
		err = fmt.Errorf("HTTP status is not 201 Created")
	}
	if err != nil {
		return err
	}
	// Don't return commit.  The caller should use a sha ref and thus know
	// the result.
	_ = br
	return nil
}

func (c *Client) GetFileContent(
	projectId int, ref, path string,
) ([]byte, error) {
	f, http, err := c.client.RepositoryFiles.GetFile(
		projectId, path, &gitlab.GetFileOptions{
			Ref: gitlab.String(ref),
		},
	)
	if http != nil && http.StatusCode == 404 {
		// Translate not found to nil content.
		return nil, nil
	}
	if err == nil && http.StatusCode != 200 {
		err = fmt.Errorf("HTTP status is not 200 OK")
	}
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(f.Content)
}

func (c *Client) CreateFile(
	projectId int, branch, path string,
	content []byte, header CommitHeader,
) (commitID string, err error) {
	content64 := base64.StdEncoding.EncodeToString(content)
	fileInfo, http, err := c.client.RepositoryFiles.CreateFile(
		projectId, path, &gitlab.CreateFileOptions{
			Branch:        gitlab.String(branch),
			Encoding:      gitlab.String("base64"),
			Content:       gitlab.String(content64),
			AuthorName:    gitlab.String(header.AuthorName),
			AuthorEmail:   gitlab.String(header.AuthorEmail),
			CommitMessage: gitlab.String(header.CommitMessage),
		},
	)
	if err == nil && http.StatusCode != 201 {
		err = fmt.Errorf("HTTP status is not 201 Created")
	}
	if err != nil {
		return "", err
	}
	// `fileInfo` does not contain the new commit.  Ignore it and instead
	// get the details from GitLab.
	_ = fileInfo
	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	return c.confirmedFileCommit(projectId, branchRef, path, content)
}

func (c *Client) UpdateFile(
	projectId int, branch, lastCommit, path string,
	content []byte, header CommitHeader,
) (commitID string, err error) {
	content64 := base64.StdEncoding.EncodeToString(content)
	fileInfo, http, err := c.client.RepositoryFiles.UpdateFile(
		projectId, path, &gitlab.UpdateFileOptions{
			Branch:        gitlab.String(branch),
			Encoding:      gitlab.String("base64"),
			Content:       gitlab.String(content64),
			AuthorName:    gitlab.String(header.AuthorName),
			AuthorEmail:   gitlab.String(header.AuthorEmail),
			CommitMessage: gitlab.String(header.CommitMessage),
			LastCommitID:  gitlab.String(lastCommit),
		},
	)
	if err == nil && http.StatusCode != 200 {
		err = fmt.Errorf("HTTP status is not 200 OK")
	}
	if err != nil {
		return "", err
	}
	// `fileInfo` does not contain the new commit.  Ignore it and instead
	// get the details from GitLab.
	_ = fileInfo
	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	return c.confirmedFileCommit(projectId, branchRef, path, content)
}

func (c *Client) confirmedFileCommit(
	projectId int, ref, path string, content []byte,
) (string, error) {
	f, http, err := c.client.RepositoryFiles.GetFile(
		projectId, path, &gitlab.GetFileOptions{
			Ref: gitlab.String(ref),
		},
	)
	if err == nil && http.StatusCode != 200 {
		err = fmt.Errorf("HTTP status is not 200 OK")
	}
	if err != nil {
		err := fmt.Errorf("failed to confirm file content: %v", err)
		return "", err
	}
	// Confirm the expected content to protect against races.
	got, err := base64.StdEncoding.DecodeString(f.Content)
	if err != nil {
		err := fmt.Errorf("failed to confirm file content: %v", err)
		return "", err
	}
	if !bytes.Equal(content, got) {
		err := fmt.Errorf("file content mismatch")
		return "", err
	}
	return f.CommitID, nil
}

func (c *Client) ListTreeAll(
	projectId int, ref string,
) ([]*gitlab.TreeNode, error) {
	var lst []*gitlab.TreeNode
	nextPage := 1
	for nextPage > 0 {
		part, http, err := c.client.Repositories.ListTree(
			projectId,
			&gitlab.ListTreeOptions{
				Ref:       gitlab.String(ref),
				Recursive: gitlab.Bool(true),
			},
			gitlab.WithListOptions(gitlab.ListOptions{
				Page:    nextPage,
				PerPage: 100,
			}),
		)
		if err != nil {
			return nil, err
		}
		lst = append(lst, part...)
		nextPage = http.NextPage
	}
	return lst, nil
}

func (c *Client) Meta(
	projectId int, ref, path string, out interface{},
) (ok bool, err error) {
	content, err := c.GetFileContent(projectId, ref, path)
	if err != nil {
		return false, err
	}
	if content == nil {
		return false, nil
	}
	return true, yaml.Unmarshal(content, out)
}
