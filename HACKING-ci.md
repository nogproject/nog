# Nog Continuous Integration
By Steffen Prohaska
<!--@@VERSIONINC@@-->

## How to use a GitLab project runner to test CI?

To test GitLab CI, it can be useful to create a personal CI environment that
uses the developer's Docker to execute CI jobs.

Assuming, you have forked the GitLab project, for example, to:

```bash
repoUrl='git.zib.de/nog/nog-my-fork' && echo "repoUrl: ${repoUrl}"
repoName="$(basename "${repoUrl}")" && echo "repoName: ${repoName}"
```

Register a runner:

* Go to the project's Settings / CI/CD / Runners, and copy the registration
  token.

```bash
 token=<copy-from-GitLab>

docker volume create "${repoName}_gitlab-runner-etc"

 docker run -it --rm \
    -v "${repoName}_gitlab-runner-etc:/etc/gitlab-runner" \
    gitlab/gitlab-runner:v12.1.0@sha256:a2e3fd77ea1fad193871eafa151604fc24f3c20bef5c8bd93aa1488d1d1a293c \
    register --non-interactive \
    --url https://git.zib.de/ \
    --registration-token "${token}" \
    --executor docker \
    --description "${repoName}--gitlab-runner" \
    --docker-image "docker:19.03.1@sha256:021de036f36d3e5c9ba5dd832276c51ea0f9ed413eab075016812bf70c046319" \
    --docker-volumes /var/run/docker.sock:/var/run/docker.sock \
    --docker-volumes /srv/${repoName}/builds:/srv/${repoName}/builds \
    --builds-dir /srv/${repoName}/builds
```

To start the runner:

```bash
docker run -d --restart always \
    --name "${repoName}--gitlab-runner" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${repoName}_gitlab-runner-etc:/etc/gitlab-runner" \
    gitlab/gitlab-runner:v12.1.0@sha256:a2e3fd77ea1fad193871eafa151604fc24f3c20bef5c8bd93aa1488d1d1a293c
```

To build and push the dev images:

```bash
docker login "registry.${repoUrl}"

DEV_IMAGE_REGISTRY=registry.${repoUrl}/ make images push-dev-images
```

To trigger a CI build, push a branch that is listed as a ref in
`.gitlab-ci.yml` to the GitLab project fork:

```bash
git remote add dev "git@${repoUrl/\//:}.git"

branch=<ci-branch> &&
git push dev "${branch}"
```
