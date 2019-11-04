// Package `gits`: Init GitLab projects.
package gits

import (
	"fmt"
	"net/url"
	"path"

	"github.com/nogproject/nog/backend/pkg/gitlab"
)

type GitlabConfig struct {
	Addr      string
	TokenPath string
}

type Gitlab struct {
	client   *gitlab.Client
	hostname string
}

func NewGitlab(cfg *GitlabConfig) (*Gitlab, error) {
	u, err := url.Parse(cfg.Addr)
	if err != nil {
		return nil, err
	}
	hostname := u.Hostname()

	client, err := gitlab.NewClient(cfg.Addr, cfg.TokenPath)
	if err != nil {
		return nil, err
	}

	return &Gitlab{
		client:   client,
		hostname: hostname,
	}, nil
}

type RepoInfo struct {
	GitlabHostname string
	GitlabId       int
	GitlabPath     string
	GitlabSsh      string
}

func (g *Gitlab) Init(inf *RepoInfo) (*RepoInfo, error) {
	prj, err := g.findOrInit(inf)
	if err != nil {
		return nil, err
	}

	dup := *inf
	inf = &dup

	inf.GitlabSsh = prj.SSHURLToRepo
	inf.GitlabId = prj.ID

	return inf, nil
}

func (g *Gitlab) findOrInit(inf *RepoInfo) (*gitlab.Project, error) {
	if inf.GitlabHostname != g.hostname {
		err := fmt.Errorf("GitLab hostname mismatch")
		return nil, err
	}

	prj, err := g.findProjectByRepoPath(inf.GitlabPath)
	if err != nil {
		return nil, err
	}
	if prj != nil {
		return prj, nil
	}

	nsId, err := g.findNamespaceByRepoPath(inf.GitlabPath)
	if err != nil {
		return nil, err
	}

	pathName := path.Base(inf.GitlabPath)
	opts := gitlab.CreateProjectOptions{
		Path:        gitlab.String(pathName),
		NamespaceID: gitlab.Int(nsId),
	}
	prj, _, err = g.client.Projects.CreateProject(&opts)
	if err != nil {
		return nil, err
	}
	if prj.PathWithNamespace != inf.GitlabPath {
		err := fmt.Errorf(
			"Gitlab returned unexpected path, "+
				"wanted `%s`, got `%s`",
			inf.GitlabPath, prj.PathWithNamespace,
		)
		return nil, err
	}

	return prj, nil
}

func (g *Gitlab) findProjectByRepoPath(
	repoPath string,
) (*gitlab.Project, error) {
	pathName := path.Base(repoPath)
	opts := gitlab.ListProjectsOptions{
		Search: gitlab.String(pathName),
		Simple: gitlab.Bool(true),
	}
	prjs, _, err := g.client.Projects.ListProjects(&opts)
	if err != nil {
		return nil, err
	}

	for _, p := range prjs {
		if p.PathWithNamespace == repoPath {
			return p, nil
		}
	}

	return nil, nil
}

func (g *Gitlab) findNamespaceByRepoPath(repoPath string) (int, error) {
	dir := path.Dir(repoPath)

	nss, _, err := g.client.Namespaces.SearchNamespace(dir)
	if err != nil {
		return 0, err
	}

	for _, ns := range nss {
		if ns.Path == dir {
			return ns.ID, nil
		}
	}

	return 0, fmt.Errorf("no namespace with path `%s`", dir)
}
