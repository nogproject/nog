# vim: sw=8

define USAGE
Usage:
  make
  make binaries
  make static  # DEPRECATED
  make clean
  make vendor  # DEPRECATED
  make vendor-status  # DEPRECATED
  make vendor-upgrade  # DEPRECATED
  make vet
  make test
  make test-t
  make errcheck
  make images
  make gc
  make down
  make down-state
  make up-godoc
  make devcerts
  make gogen
  make meteor  # DEPRECATED
  make nog-app
  make nog-app-2
  make deb
  make deb-from-product
  make docker
  make docker-from-product

Make is only used to build Go code in `backend/`.  Javascript still uses Npm
scripts.

`make` runs `go install`.

`make clean` removes files that `make` creates.  It also removes the `go`
Docker volume, which is necessary after a Docker image change to re-populated
it from the latest image.

`make binaries` builds Linux binaries in `product/bin`.  The binaries depend on
glibc.  The `netgo` build tag is not supported, because Git2go requires glibc.
See `tools/images/godev/Dockerfile`.

`make static` (DEPRECATED) is an alias for `make binaries`.

`make vendor` is DEPRECATED.  Use Go module commands instead, like:

```
godev go list -m all
godev go mod tidy
godev go get ...
godev go get -u=patch  # upgrade all to latest patch
godev go get -u        # upgrade all to latest minor
```

`make vet`, `make test`, and `make errcheck` run the respective Go tools.

`make test-t` runs the sharness tests in `backend/t/`.  See
<https://github.com/chriscool/sharness>. To manually run them:

```
godev bash
cd backend/t
./test-all.sh  # All tests.
./t0000-test.sh  # Individual tests.
```

`make images` builds Docker dev images explicitly.  `make push-dev-images`
pushes the dev images, and `make pull-dev-images` pulls them.  To build and
push the dev images to a GitLab container image registry, log in with a
personal access token that has scope `api`, and build and push the dev images,
for example:

```
docker login registry.git.zib.de
DEV_IMAGE_REGISTRY=registry.git.zib.de/nog/nog/ make images push-dev-images
```

`make gc` removes exited containers.  `make down` removes Docker objects that
can be re-created.  `make down-state` also removes volumes that contain state
that cannot be re-created.

`make up-godoc` starts the godoc server and on Mac opens the project doc in
Chrome.

`make devcert` recreates the SSL certificates used for local testing.

`make gogen` runs `go generate`.

`make meteor` (DEPRECATED) builds the production Meteor app.  Use `make
nog-app` instead.

`make nog-app` builds the production Meteor Nog App.

`make nog-app-2` builds the production Meteor Nog App 2.

`make deb` builds the required products and packs them into deb files.  `make
deb-from-product` packs the deb files without building the products, assuming
that the products are already up to date.

`make docker` builds products and bundles them into Docker images.  `make
docker-from-product` builds the Docker images without building the products,
assuming that the products are already up to date.

See CONTRIBUTING.md for details.
endef

IS_CONTAINER := $(shell test -d /go && echo isContainer)
IS_GIT_FILE := $(shell test -f .git && echo isGitFile)

ifdef IS_CONTAINER
    $(error "This Makefile must be used outside the godev container.")
endif

ifdef IS_GIT_FILE
    $(error "`.git` is a file.  It must be the git dir.  See `tools/env.sh`.")
endif

# The build tag encodes information about the Git commit and the build time.
# It is determined outside the container, so that the same Git version is used
# that is also used for managing the worktree.
GIT_COMMIT_TAG := $(shell \
    TZ=UTC git show -s \
	 --date=format-local:%Y%m%dT%H%M%SZ --abbrev=10 --pretty=%cd-g%h \
)
GIT_DIRTY := $(shell \
    if [ -n "$$(git status -s)" ]; then echo "-dirty"; fi \
)
BUILD_DATE := $(shell \
    date -u +%s \
)
BUILD_TAG := $(GIT_COMMIT_TAG)-b$(BUILD_DATE)$(GIT_DIRTY)

OS := $(shell uname)
DC := docker-compose
DDEV := $(DC) run --rm godev-make
DMAKE := $(DDEV) make -f Makefile.docker BUILD_TAG=$(BUILD_TAG)


.PHONY: all
all:
	@echo '    DMAKE all'
	$(DMAKE) all

.PHONY: help
export USAGE
help:
	@echo "$${USAGE}"

.PHONY: install
install:
	@echo '    DMAKE install'
	$(DMAKE) install

.PHONY: binaries
binaries:
	@echo '    DMAKE binaries'
	$(DMAKE) binaries

product/bin/tar-incremental-mtime:
	@./tools/bin/make-tar-incremental-mtime

.PHONY: deb
deb: nog-app-2 product/bin/tar-incremental-mtime
	@echo '    DMAKE deb'
	$(DMAKE) deb

.PHONY: deb-from-product
deb-from-product:
	@echo '    DMAKE deb-from-product'
	$(DMAKE) deb-from-product

.PHONY: docker
docker: deb
	@BUILD_TAG=$(BUILD_TAG) ./tools/bin/make-docker

.PHONY: docker-from-product
docker-from-product: deb-from-product
	@BUILD_TAG=$(BUILD_TAG) ./tools/bin/make-docker

.PHONY: static
static:
	@echo '    DMAKE static'
	@echo 'WARNING: Target `static` is deprecated.  Use `binaries` instead.'
	$(MAKE) binaries

.PHONY: clean
clean:
	@echo '    DMAKE clean'
	$(DMAKE) clean
	$(MAKE) gc
	docker volume rm -f nog_go nog_meteor

.PHONY: vendor
vendor:
	@echo '    DEPRECATED vendor'
	@echo '`vendor` is no longer used, since Go modules have been enabled.'

.PHONY: vendor-status
vendor-status:
	@echo '    DEPRECATED vendor-status'
	@echo '`vendor-status` is no longer used, since Go modules have been enabled.'
	@echo 'Use Go module commands instead, like:'
	@echo
	@echo '    godev go list -m all'
	@echo

.PHONY: vendor-upgrade
vendor-upgrade:
	@echo '    DEPRECATED vendor-upgrade'
	@echo '`vendor-upgrade` is no longer used, since Go modules have been enabled.'
	@echo 'Use Go module commands instead, like:'
	@echo
	@echo '    godev go mod tidy'
	@echo '    godev go get ...'
	@echo

.PHONY: devcerts
devcerts:
	@echo '    DMAKE devcerts (force)'
	$(DMAKE) -B devcerts

.PHONY: gogen
gogen:
	@echo '    DMAKE gogen (force)'
	$(DMAKE) -B gogen

.PHONY: vet
vet:
	@echo '    DMAKE vet'
	$(DMAKE) vet

.PHONY: test
test:
	@echo '    DMAKE test'
	$(DMAKE) test

.PHONY: test-t
test-t:
	@echo '    DMAKE test-t'
	$(DMAKE) test-t

.PHONY: errcheck
errcheck:
	@echo '    DMAKE errcheck'
	$(DMAKE) errcheck

.PHONY: images
images:
	@echo '    DOCKER BUILD'
	$(DC) build

.PHONY: push-dev-images
push-dev-images:
	@echo '    DOCKER PUSH'
	$(DC) push dev godev meteordev-make

.PHONY: pull-dev-images
pull-dev-images:
	@echo '    DOCKER PULL'
	$(DC) pull dev godev meteordev-make

.PHONY: gc
gc:
	@echo '    DOCKER RM exited containers'
	$(DC) rm -f

.PHONY: down
down:
	@echo '    DOCKER COMPOSE down'
	$(DC) down

.PHONY: down-state
down-state:
	@echo '    DOCKER COMPOSE down stateful'
	$(DC) down --volumes

.PHONY: up-godoc
up-godoc:
	@echo '    DOCKER COMPOSE up godoc http://localhost:6060'
	$(DC) up -d godoc
ifeq ($(OS),Darwin)
	@sleep 1
	open -b com.google.Chrome http://localhost:6060/pkg/github.com/nogproject/?m=all
else
	@echo
	@echo open http://localhost:6060/pkg/github.com/nogproject/?m=all
endif

# DEPRECATED: Use `make nog-app` instead of `make meteor`.  `make meteor` can
# be removed when control has been migrated to `make nog-app`.
.PHONY: meteor
meteor:
	@echo '    METEOR BUILD product/nogappd-meteor.tar.gz'
	@mkdir -p product
	@$(DC) run --rm meteordev-make bash -c ' \
		set -x && \
		pushd apps/nog-app/meteor >/dev/null && \
		meteor npm install --production && \
		meteor build /tmp --architecture os.linux.x86_64 && \
		popd >/dev/null && \
		mv /tmp/meteor.tar.gz product/nogappd-meteor.tar.gz && \
		set +x && \
		echo OK product/nogappd-meteor.tar.gz \
	'

.PHONY: nog-app
nog-app:
	@echo '    METEOR BUILD product/nog-app.tar.gz'
	@mkdir -p product
	@$(DC) run --rm meteordev-make bash -c ' \
		set -x && \
		pushd apps/nog-app/meteor >/dev/null && \
		meteor npm install --production && \
		meteor build /tmp --architecture os.linux.x86_64 && \
		popd >/dev/null && \
		mv /tmp/meteor.tar.gz product/nog-app.tar.gz && \
		set +x && \
		echo OK product/nog-app.tar.gz \
	'

.PHONY: nog-app-2
nog-app-2:
	@echo '    METEOR BUILD product/nog-app-2.tar.gz'
	@mkdir -p product
	@$(DC) run --rm meteordev-make bash -c ' \
		set -x && \
		pushd web/apps/nog-app-2 >/dev/null && \
		meteor npm install --production && \
		meteor build /tmp --architecture os.linux.x86_64 && \
		popd >/dev/null && \
		mv /tmp/nog-app-2.tar.gz product/nog-app-2.tar.gz && \
		set +x && \
		echo OK product/nog-app-2.tar.gz \
	'
