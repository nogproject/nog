// Package `gitlab` wraps a subset of `github.com/xanzy/go-gitlab`; only what
// other Nog packages use.
package gitlab

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	gitlab "github.com/xanzy/go-gitlab"
)

type Branch = gitlab.Branch
type Client = gitlab.Client
type Commit = gitlab.Commit
type CreateBranchOptions = gitlab.CreateBranchOptions
type CreateFileOptions = gitlab.CreateFileOptions
type CreateProjectOptions = gitlab.CreateProjectOptions
type GetFileOptions = gitlab.GetFileOptions
type ListOptions = gitlab.ListOptions
type ListProjectsOptions = gitlab.ListProjectsOptions
type ListTreeOptions = gitlab.ListTreeOptions
type Project = gitlab.Project
type TreeNode = gitlab.TreeNode
type UpdateFileOptions = gitlab.UpdateFileOptions

func String(s string) *string { return gitlab.String(s) }
func Int(i int) *int          { return gitlab.Int(i) }
func Bool(b bool) *bool       { return gitlab.Bool(b) }

func NewClient(addr, tokenPath string) (*Client, error) {
	buf, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	token := string(buf)
	token = strings.TrimSpace(token)

	c := gitlab.NewClient(nil, token)
	addr = strings.TrimRight(addr, "/")
	addr = fmt.Sprintf("%s/api/v4/", addr)
	c.SetBaseURL(addr)

	_, _, err = c.Version.GetVersion()
	if err != nil {
		err := fmt.Errorf("failed ping GitLab: %v", err)
		return nil, err
	}

	return c, nil
}

func WithListOptions(opts ListOptions) gitlab.OptionFunc {
	return func(req *http.Request) error {
		addq := func(q string, val int) {
			if val == 0 {
				return
			}
			if req.URL.RawQuery != "" {
				req.URL.RawQuery += "&"
			}
			req.URL.RawQuery += fmt.Sprintf("%s=%d", q, val)
		}
		addq("page", opts.Page)
		addq("per_page", opts.PerPage)
		return nil
	}
}
